#!/usr/bin/env bash
# shellcheck shell=bash
# shellcheck disable=SC2154

absolute_symlink_target() {
  local link="$1"
  local target

  [ -L "$link" ] || return 1
  target="$(readlink -- "$link" 2>/dev/null || true)"
  [ -n "$target" ] || return 1
  case "$target" in
    /*) ;;
    *) target="$(dirname -- "$link")/$target" ;;
  esac
  readlink -m -- "$target"
}

end4_source_from_current_generation() {
  local gcroot generation source resolved

  gcroot="$HOME/.local/state/home-manager/gcroots/current-home"
  [ -L "$gcroot" ] || return 1
  generation="$(readlink -f -- "$gcroot" 2>/dev/null || true)"
  [ -n "$generation" ] || return 1
  source="$generation/home-files/.config/hypr/end4"
  [ -L "$source" ] || return 1
  resolved="$(readlink -f -- "$source" 2>/dev/null || true)"
  [ -n "$resolved" ] && [ -d "$resolved" ] && [ -f "$resolved/hyprland.lua" ] || return 1
  printf '%s' "$resolved"
}

end4_source_from_immutable_home_manager_files() {
  local target="$1"
  local source relative store_item suffix resolved

  source="$(absolute_symlink_target "$target" 2>/dev/null || true)"
  case "$source" in
    /nix/store/*) ;;
    *) return 1 ;;
  esac
  relative="${source#/nix/store/}"
  store_item="${relative%%/*}"
  suffix="${relative#*/}"
  [[ "$store_item" =~ ^[0-9a-df-np-sv-z]{32}-home-manager-files$ ]] || return 1
  [ "$suffix" = ".config/hypr/end4" ] || return 1
  resolved="$(readlink -f -- "$source" 2>/dev/null || true)"
  [ -n "$resolved" ] && [ -d "$resolved" ] && [ -f "$resolved/hyprland.lua" ] || return 1
  printf '%s' "$resolved"
}

validate_end4_profile_tree() {
  local dir target target_source current_source store_source

  [ "$(wahrwelt_shell_family "$profile")" = "end4" ] || return 0

  dir="$(hypr_dir)"
  target="$dir/end4"
  if [ ! -L "$target" ]; then
    log "end4 profile path is not a Home Manager symlink: $target"
    return 1
  fi
  target_source="$(readlink -f -- "$target" 2>/dev/null || true)"
  if [ -z "$target_source" ] || [ ! -d "$target_source" ] || [ ! -f "$target_source/hyprland.lua" ]; then
    log "end4 profile symlink is broken or incomplete: $target"
    return 1
  fi

  current_source="$(end4_source_from_current_generation 2>/dev/null || true)"
  if [ -n "$current_source" ] && [ "$target_source" = "$current_source" ]; then
    return 0
  fi

  store_source="$(end4_source_from_immutable_home_manager_files "$target" 2>/dev/null || true)"
  if [ -n "$store_source" ] && [ "$target_source" = "$store_source" ]; then
    return 0
  fi

  log "end4 profile symlink is not owned by the current Home Manager generation: $target"
  return 1
}

if ! declare -p wahrwelt_transaction_snapshot_dirs >/dev/null 2>&1; then
  wahrwelt_transaction_snapshot_dirs=()
fi
if ! declare -p wahrwelt_snapshot_parent_fds >/dev/null 2>&1; then
  declare -A wahrwelt_snapshot_parent_fds=()
  declare -A wahrwelt_snapshot_parent_identities=()
fi
if ! declare -p wahrwelt_snapshot_directory_fds >/dev/null 2>&1; then
  declare -A wahrwelt_snapshot_directory_fds=()
  declare -A wahrwelt_snapshot_directory_identities=()
  declare -A wahrwelt_snapshot_directory_parent_fds=()
  declare -A wahrwelt_snapshot_directory_parent_identities=()
  declare -A wahrwelt_snapshot_directory_names=()
  declare -A wahrwelt_snapshot_leaf_identities=()
  declare -A wahrwelt_snapshot_owned_types=()
  declare -A wahrwelt_snapshot_owned_identities=()
  declare -A wahrwelt_snapshot_owned_parents=()
  declare -A wahrwelt_snapshot_owned_recoveries=()
fi
if ! declare -p wahrwelt_snapshot_paths >/dev/null 2>&1; then
  declare -A wahrwelt_snapshot_paths=()
  declare -A wahrwelt_snapshot_original_types=()
  declare -A wahrwelt_snapshot_original_identities=()
  declare -A wahrwelt_snapshot_original_parents=()
fi
if ! declare -p wahrwelt_exact_path_guard_types >/dev/null 2>&1; then
  declare -A wahrwelt_exact_path_guard_types=()
  declare -A wahrwelt_exact_path_guard_identities=()
  declare -A wahrwelt_exact_path_guard_parents=()
fi
wahrwelt_new_snapshot_dir=""
wahrwelt_snapshot_recovery_fd_path=""
wahrwelt_snapshot_recovery_exact_path=""
wahrwelt_snapshot_recovery_identity=""
wahrwelt_snapshot_recovery_public_path=""
wahrwelt_snapshot_recovery_public_identity=""

runtime_path_kind() {
  local path="$1"

  if [ -L "$path" ]; then
    printf '%s' symlink
  elif [ -f "$path" ]; then
    printf '%s' regular
  elif [ -e "$path" ]; then
    printf '%s' other
  else
    printf '%s' absent
  fi
}

runtime_path_identity() {
  local path="$1"

  [ -e "$path" ] || [ -L "$path" ] || return 1
  stat -c '%d:%i' -- "$path" 2>/dev/null
}

runtime_directory_identity() {
  local path="$1"

  [ -d "$path" ] || return 1
  stat -Lc '%d:%i' -- "$path" 2>/dev/null
}

runtime_nofollow_directory_identity() {
  local path="$1"

  [ -d "$path" ] && [ ! -L "$path" ] || return 1
  stat -c '%d:%i' -- "$path" 2>/dev/null
}

runtime_regular_inode() {
  local path="$1"
  local node digest

  [ -f "$path" ] || return 1
  node="$(stat -Lc '%d:%i:%h' -- "$path" 2>/dev/null || true)"
  digest="$(sha256sum -- "$path" 2>/dev/null | awk '{print $1}' || true)"
  [ -n "$node" ] && [ -n "$digest" ] || return 1
  printf '%s:%s' "$node" "$digest"
}

runtime_regular_is_private() {
  local path="$1"

  [ -f "$path" ] &&
    [ "$(stat -Lc %h -- "$path" 2>/dev/null || true)" = 1 ]
}

runtime_regular_matches_content() {
  local parent_fd="$1"
  local name="$2"
  local content="$3"

  command -v python3 >/dev/null 2>&1 || return 1
  python3 -I -S - "$parent_fd" "$name" "$content" <<'PY'
import os
import stat
import sys

parent_fd = int(sys.argv[1])
name = os.fsencode(sys.argv[2])
expected = os.fsencode(sys.argv[3]) + b"\n"
flags = os.O_RDONLY | os.O_CLOEXEC | os.O_NOFOLLOW | os.O_NONBLOCK
fd = os.open(name, flags, dir_fd=parent_fd)
try:
    before = os.fstat(fd)
    if (
        not stat.S_ISREG(before.st_mode)
        or before.st_nlink != 1
        or stat.S_IMODE(before.st_mode) != 0o644
    ):
        raise OSError("runtime target is not an ordinary managed regular file")
    chunks = []
    remaining = len(expected) + 1
    while remaining:
        chunk = os.read(fd, remaining)
        if not chunk:
            break
        chunks.append(chunk)
        remaining -= len(chunk)
    value = b"".join(chunks)
    after = os.fstat(fd)
    visible = os.stat(name, dir_fd=parent_fd, follow_symlinks=False)
    stable = lambda info: (
        info.st_dev, info.st_ino, info.st_mode, info.st_nlink, info.st_size,
        info.st_mtime_ns, info.st_ctime_ns,
    )
    if (
        value != expected
        or stable(before) != stable(after)
        or not stat.S_ISREG(visible.st_mode)
        or visible.st_nlink != 1
        or (visible.st_dev, visible.st_ino) != (after.st_dev, after.st_ino)
    ):
        raise OSError("runtime target content or identity changed")
finally:
    os.close(fd)
PY
}

runtime_path_matches_content() {
  local path="$1"
  local content="$2"
  local parent before after fd status=1

  parent="$(dirname -- "$path")"
  [ -d "$parent" ] && [ ! -L "$parent" ] || return 1
  before="$(runtime_directory_identity "$parent" 2>/dev/null || true)"
  [ -n "$before" ] || return 1
  exec {fd}<"$parent" || return 1
  after="$(runtime_directory_identity "/proc/${BASHPID:-$$}/fd/$fd" 2>/dev/null || true)"
  if [ "$after" = "$before" ] && runtime_canonical_parent_matches "$path" "$before" &&
    runtime_regular_matches_content "$fd" "${path##*/}" "$content" 2>/dev/null &&
    runtime_canonical_parent_matches "$path" "$before"; then
    status=0
  fi
  exec {fd}<&-
  return "$status"
}

stable_runtime_entrypoint_matches() {
  local path="$1"
  local content="$2"
  local gcroot generation expected resolved resolved_after

  runtime_path_matches_content "$path" "$content" && return 0
  [ -L "$path" ] || return 1

  gcroot="$HOME/.local/state/home-manager/gcroots/current-home"
  [ -L "$gcroot" ] || return 1
  generation="$(readlink -f -- "$gcroot" 2>/dev/null || true)"
  [ -n "$generation" ] || return 1
  expected="$generation/home-files/.config/hypr/hyprland.lua"
  resolved="$(readlink -f -- "$path" 2>/dev/null || true)"
  [ -n "$resolved" ] && [ -f "$resolved" ] || return 1
  [ "$resolved" = "$(readlink -f -- "$expected" 2>/dev/null || true)" ] || return 1
  printf '%s\n' "$content" | cmp -s - "$resolved" || return 1
  resolved_after="$(readlink -f -- "$path" 2>/dev/null || true)"
  [ "$resolved_after" = "$resolved" ]
}

runtime_create_regular_candidate() {
  local parent_fd="$1"
  local name="$2"
  local content="$3"

  command -v python3 >/dev/null 2>&1 || return 1
  python3 - "$parent_fd" "$name" "$content" <<'PY'
import hashlib
import os
import stat
import sys

parent_fd = int(sys.argv[1])
name = os.fsencode(sys.argv[2])
value = os.fsencode(sys.argv[3]) + b"\n"
flags = os.O_WRONLY | os.O_CLOEXEC | os.O_NOFOLLOW | os.O_CREAT | os.O_EXCL
fd = os.open(name, flags, 0o644, dir_fd=parent_fd)
try:
    info = os.fstat(fd)
    if not stat.S_ISREG(info.st_mode) or info.st_nlink != 1:
        raise OSError("runtime candidate is not a private regular file")
    offset = 0
    while offset < len(value):
        offset += os.write(fd, value[offset:])
    os.fchmod(fd, 0o644)
    os.fsync(fd)
    info = os.fstat(fd)
    if not stat.S_ISREG(info.st_mode) or info.st_nlink != 1:
        raise OSError("runtime candidate changed during publication")
    print(f"{info.st_dev}:{info.st_ino}:{info.st_nlink}:{hashlib.sha256(value).hexdigest()}")
finally:
    os.close(fd)
PY
}

# Copy a retained transaction snapshot into a fresh private candidate.  Rollback
# must never truncate the currently visible runtime inode in place: even a
# managed-looking file can gain a hardlink after its preflight.  The caller
# exchanges this candidate only after re-checking both identities.
runtime_create_regular_candidate_from_file() {
  local parent_fd="$1"
  local name="$2"
  local source="$3"
  local mode="$4"
  local source_expected="$5"

  command -v python3 >/dev/null 2>&1 || return 1
  python3 - "$parent_fd" "$name" "$source" "$mode" "$source_expected" <<'PY'
import hashlib
import os
import stat
import sys

parent_fd = int(sys.argv[1])
name = os.fsencode(sys.argv[2])
source = os.fsencode(sys.argv[3])
mode = int(sys.argv[4], 8)
source_expected = sys.argv[5]
source_flags = os.O_RDONLY | os.O_CLOEXEC | os.O_NOFOLLOW
target_flags = os.O_WRONLY | os.O_CLOEXEC | os.O_NOFOLLOW | os.O_CREAT | os.O_EXCL
source_fd = os.open(source, source_flags)
target_fd = None
try:
    source_info = os.fstat(source_fd)
    if not stat.S_ISREG(source_info.st_mode) or source_info.st_nlink != 1:
        raise OSError("transaction snapshot source is not a private regular file")
    target_fd = os.open(name, target_flags, 0o600, dir_fd=parent_fd)
    digest = hashlib.sha256()
    while True:
        chunk = os.read(source_fd, 1024 * 1024)
        if not chunk:
            break
        digest.update(chunk)
        offset = 0
        while offset < len(chunk):
            offset += os.write(target_fd, chunk[offset:])
    source_after = os.fstat(source_fd)
    source_identity = ":".join(map(str, (
        source_after.st_dev, source_after.st_ino, source_after.st_mode,
        source_after.st_nlink, source_after.st_size, source_after.st_mtime_ns,
        source_after.st_ctime_ns,
    ))) + ":" + digest.hexdigest()
    stable = lambda info: (
        info.st_dev, info.st_ino, info.st_mode, info.st_nlink, info.st_size,
        info.st_mtime_ns, info.st_ctime_ns,
    )
    if stable(source_info) != stable(source_after) or source_identity != source_expected:
        raise OSError("transaction snapshot source changed before rollback copy")
    os.fchmod(target_fd, mode)
    os.fsync(target_fd)
    target_info = os.fstat(target_fd)
    if not stat.S_ISREG(target_info.st_mode) or target_info.st_nlink != 1:
        raise OSError("rollback candidate is not a private regular file")
    print(f"{target_info.st_dev}:{target_info.st_ino}:{target_info.st_nlink}:{digest.hexdigest()}")
finally:
    if target_fd is not None:
        os.close(target_fd)
    os.close(source_fd)
PY
}

# Publish an absent runtime leaf directly from an anonymous inode.  A named
# staging file would leave a same-UID window where a replacement of that stage
# is moved into the canonical name by rename.  linkat(AT_EMPTY_PATH) instead
# links the exact file descriptor and fails atomically when a winner appears.
runtime_publish_anonymous_regular() {
  local parent_fd="$1"
  local name="$2"
  local mode="$3"
  local source_kind="$4"
  local source_value="$5"
  local source_expected="${6:-}"

  command -v python3 >/dev/null 2>&1 || return 1
  python3 - "$parent_fd" "$name" "$mode" "$source_kind" "$source_value" "$source_expected" <<'PY'
import ctypes
import hashlib
import os
import stat
import sys

parent_fd = int(sys.argv[1])
name = os.fsencode(sys.argv[2])
mode = int(sys.argv[3], 8)
source_kind = sys.argv[4]
source_value = sys.argv[5]
source_expected = sys.argv[6]
libc = ctypes.CDLL(None, use_errno=True)
linkat = libc.linkat
linkat.argtypes = [ctypes.c_int, ctypes.c_char_p, ctypes.c_int, ctypes.c_char_p, ctypes.c_int]
linkat.restype = ctypes.c_int
AT_EMPTY_PATH = 0x1000
tmp_flags = os.O_TMPFILE | os.O_RDWR | os.O_CLOEXEC
tmp_fd = os.open(".", tmp_flags, 0o600, dir_fd=parent_fd)
source_fd = None
try:
    digest = hashlib.sha256()
    if source_kind == "content":
        chunks = (os.fsencode(source_value) + b"\n",)
    elif source_kind == "file":
        source_fd = os.open(os.fsencode(source_value), os.O_RDONLY | os.O_CLOEXEC | os.O_NOFOLLOW)
        source_info = os.fstat(source_fd)
        if not stat.S_ISREG(source_info.st_mode) or source_info.st_nlink != 1:
            raise OSError("transaction snapshot source is not a private regular file")
        chunks = iter(lambda: os.read(source_fd, 1024 * 1024), b"")
    else:
        raise OSError("unknown anonymous runtime source")
    for chunk in chunks:
        digest.update(chunk)
        offset = 0
        while offset < len(chunk):
            offset += os.write(tmp_fd, chunk[offset:])
    if source_fd is not None:
        source_after = os.fstat(source_fd)
        source_identity = ":".join(map(str, (
            source_after.st_dev, source_after.st_ino, source_after.st_mode,
            source_after.st_nlink, source_after.st_size, source_after.st_mtime_ns,
            source_after.st_ctime_ns,
        ))) + ":" + digest.hexdigest()
        stable = lambda info: (
            info.st_dev, info.st_ino, info.st_mode, info.st_nlink, info.st_size,
            info.st_mtime_ns, info.st_ctime_ns,
        )
        if stable(source_info) != stable(source_after) or source_identity != source_expected:
            raise OSError("transaction snapshot source changed before anonymous publish")
    os.fchmod(tmp_fd, mode)
    os.fsync(tmp_fd)
    info = os.fstat(tmp_fd)
    if not stat.S_ISREG(info.st_mode) or info.st_nlink != 0:
        raise OSError("anonymous runtime candidate changed before publication")
    if linkat(tmp_fd, b"", parent_fd, name, AT_EMPTY_PATH) != 0:
        err = ctypes.get_errno()
        raise OSError(err, os.strerror(err))
    info = os.fstat(tmp_fd)
    target_info = os.stat(name, dir_fd=parent_fd, follow_symlinks=False)
    if (not stat.S_ISREG(target_info.st_mode) or info.st_nlink != 1 or
            (target_info.st_dev, target_info.st_ino) != (info.st_dev, info.st_ino)):
        raise OSError("anonymous runtime publication lost identity")
    print(f"{info.st_dev}:{info.st_ino}:{info.st_nlink}:{digest.hexdigest()}")
finally:
    if source_fd is not None:
        os.close(source_fd)
    os.close(tmp_fd)
PY
}

runtime_state_identity() {
  local path="$1"

  case "$(runtime_path_kind "$path")" in
    regular) runtime_regular_inode "$path" ;;
    symlink) runtime_path_identity "$path" ;;
    absent) return 0 ;;
    *) return 1 ;;
  esac
}

runtime_parent_identity() {
  local parent

  parent="$(dirname -- "$1")"
  [ -d "$parent" ] && [ ! -L "$parent" ] || return 1
  runtime_path_identity "$parent"
}

wahrwelt_capture_exact_path_guards() {
  local path type identity parent

  wahrwelt_exact_path_guard_types=()
  wahrwelt_exact_path_guard_identities=()
  wahrwelt_exact_path_guard_parents=()
  for path in "$@"; do
    type="$(runtime_path_kind "$path")"
    case "$type" in
      absent) identity="" ;;
      regular)
        if ! runtime_regular_is_private "$path"; then
          log "refusing hardlinked state path at transaction begin: $path"
          return 1
        fi
        identity="$(runtime_state_identity "$path" 2>/dev/null || true)"
        [ -n "$identity" ] || return 1
        ;;
      symlink)
        identity="$(runtime_state_identity "$path" 2>/dev/null || true)"
        [ -n "$identity" ] || return 1
        ;;
      *)
        log "refusing non-regular state path at transaction begin: $path"
        return 1
        ;;
    esac
    parent="$(runtime_parent_identity "$path" 2>/dev/null || true)"
    if [ -z "$parent" ]; then
      log "state parent was absent or unsafe at transaction begin: $(dirname -- "$path")"
      return 1
    fi
    wahrwelt_exact_path_guard_types["$path"]="$type"
    wahrwelt_exact_path_guard_identities["$path"]="$identity"
    wahrwelt_exact_path_guard_parents["$path"]="$parent"
  done
}

wahrwelt_verify_exact_path_guards() {
  local path expected_type expected_identity expected_parent
  local current_type current_identity current_parent

  for path in "$@"; do
    if [ -z "${wahrwelt_exact_path_guard_types[$path]+x}" ]; then
      log "state path lacks a transaction-begin ownership guard: $path"
      return 1
    fi
    expected_type="${wahrwelt_exact_path_guard_types[$path]}"
    expected_identity="${wahrwelt_exact_path_guard_identities[$path]}"
    expected_parent="${wahrwelt_exact_path_guard_parents[$path]}"
    current_type="$(runtime_path_kind "$path")"
    current_identity="$(runtime_state_identity "$path" 2>/dev/null || true)"
    current_parent="$(runtime_parent_identity "$path" 2>/dev/null || true)"
    if [ "$current_type" != "$expected_type" ] ||
      [ "$current_identity" != "$expected_identity" ] ||
      [ "$current_parent" != "$expected_parent" ]; then
      log "state path changed after transaction begin; preserving concurrent winner: $path"
      return 1
    fi
  done
}

wahrwelt_snapshot_matches_exact_path_guards() {
  local snapshot_dir="$1"
  local path index key expected_type expected_identity expected_parent
  shift

  for path in "$@"; do
    if [ -z "${wahrwelt_exact_path_guard_types[$path]+x}" ]; then
      log "state snapshot path lacks a transaction-begin ownership guard: $path"
      return 1
    fi
    index="$(snapshot_path_index "$snapshot_dir" "$path" 2>/dev/null || true)"
    [ -n "$index" ] || return 1
    key="$(snapshot_parent_key "$snapshot_dir" "$index")"
    expected_type="${wahrwelt_exact_path_guard_types[$path]}"
    expected_identity="${wahrwelt_exact_path_guard_identities[$path]}"
    expected_parent="${wahrwelt_exact_path_guard_parents[$path]}"
    if [ "${wahrwelt_snapshot_original_types[$key]:-}" != "$expected_type" ] ||
      [ "${wahrwelt_snapshot_original_identities[$key]:-}" != "$expected_identity" ] ||
      [ "${wahrwelt_snapshot_original_parents[$key]:-}" != "$expected_parent" ]; then
      log "state snapshot differs from transaction-begin ownership guard; preserving concurrent winner: $path"
      return 1
    fi
  done
}

wahrwelt_begin_exact_snapshot() {
  local parent="$1"
  local prefix="$2"
  local kind="${3:-transaction}"
  local parent_identity parent_fd parent_pinned opened_parent snapshot_dir snapshot_fd snapshot_identity

  wahrwelt_new_snapshot_dir=""
  [ -d "$parent" ] || return 1
  case "$parent" in
    "/proc/${BASHPID:-$$}/fd/"* | "/proc/$$/fd/"*) ;;
    *) [ ! -L "$parent" ] || return 1 ;;
  esac
  parent_identity="$(runtime_directory_identity "$parent" 2>/dev/null || true)"
  [ -n "$parent_identity" ] || return 1
  exec {parent_fd}<"$parent" || return 1
  parent_pinned="/proc/${BASHPID:-$$}/fd/$parent_fd"
  opened_parent="$(runtime_directory_identity "$parent_pinned" 2>/dev/null || true)"
  if [ "$opened_parent" != "$parent_identity" ]; then
    exec {parent_fd}<&-
    return 1
  fi
  if ! wahrwelt_create_pinned_private_directory "$parent_fd" prefix "$prefix" "snapshot-$kind"; then
    exec {parent_fd}<&-
    return 1
  fi
  snapshot_fd="$wahrwelt_created_directory_fd"
  snapshot_identity="$wahrwelt_created_directory_identity"
  snapshot_dir="$parent/${wahrwelt_created_directory_name}"
  if [ "$(runtime_directory_identity "/proc/${BASHPID:-$$}/fd/$snapshot_fd" 2>/dev/null || true)" != "$snapshot_identity" ] ||
    [ "$(runtime_directory_identity "$parent_pinned/${wahrwelt_created_directory_name}" 2>/dev/null || true)" != "$snapshot_identity" ]; then
    exec {snapshot_fd}<&-
    exec {parent_fd}<&-
    return 1
  fi
  wahrwelt_snapshot_directory_fds["$snapshot_dir"]="$snapshot_fd"
  wahrwelt_snapshot_directory_identities["$snapshot_dir"]="$snapshot_identity"
  wahrwelt_snapshot_directory_parent_fds["$snapshot_dir"]="$parent_fd"
  wahrwelt_snapshot_directory_parent_identities["$snapshot_dir"]="$parent_identity"
  wahrwelt_snapshot_directory_names["$snapshot_dir"]="$wahrwelt_created_directory_name"
  wahrwelt_new_snapshot_dir="$snapshot_dir"
}

wahrwelt_snapshot_directory_fd() {
  printf '%s' "${wahrwelt_snapshot_directory_fds[$1]:-}"
}

wahrwelt_refresh_snapshot_recovery_report() {
  local snapshot_dir="$1"
  local fd expected resolved resolved_identity

  wahrwelt_snapshot_recovery_fd_path=""
  wahrwelt_snapshot_recovery_exact_path=""
  wahrwelt_snapshot_recovery_identity=""
  wahrwelt_snapshot_recovery_public_path="$snapshot_dir"
  wahrwelt_snapshot_recovery_public_identity="$(runtime_path_identity "$snapshot_dir" 2>/dev/null || true)"
  fd="${wahrwelt_snapshot_directory_fds[$snapshot_dir]:-}"
  expected="${wahrwelt_snapshot_directory_identities[$snapshot_dir]:-}"
  [ -n "$fd" ] && [ -n "$expected" ] || return 1
  wahrwelt_snapshot_recovery_fd_path="/proc/${BASHPID:-$$}/fd/$fd"
  [ "$(runtime_directory_identity "$wahrwelt_snapshot_recovery_fd_path" 2>/dev/null || true)" = "$expected" ] || return 1
  wahrwelt_snapshot_recovery_identity="$expected"
  resolved="$(readlink -- "$wahrwelt_snapshot_recovery_fd_path" 2>/dev/null || true)"
  case "$resolved" in "" | *' (deleted)') return 1 ;; esac
  if declare -F wahrwelt_before_snapshot_recovery_verify_hook >/dev/null 2>&1; then
    wahrwelt_before_snapshot_recovery_verify_hook "$snapshot_dir" "$resolved" || return 1
  fi
  wahrwelt_snapshot_recovery_public_identity="$(runtime_path_identity "$snapshot_dir" 2>/dev/null || true)"
  resolved_identity="$(runtime_nofollow_directory_identity "$resolved" 2>/dev/null || true)"
  [ "$resolved_identity" = "$expected" ] || return 1
  wahrwelt_snapshot_recovery_exact_path="$resolved"
}

wahrwelt_snapshot_directory_recovery_path() {
  wahrwelt_refresh_snapshot_recovery_report "$1" || return 1
  printf '%s' "$wahrwelt_snapshot_recovery_exact_path"
}

log_snapshot_recovery() {
  local context="$1"
  local snapshot_dir="$2"
  if wahrwelt_refresh_snapshot_recovery_report "$snapshot_dir" 2>/dev/null; then
    log "$context: $wahrwelt_snapshot_recovery_exact_path identity=$wahrwelt_snapshot_recovery_identity"
    return 0
  fi
  log "$context: durable path unproven; live descriptor=$wahrwelt_snapshot_recovery_fd_path identity=${wahrwelt_snapshot_recovery_identity:-unknown}; public collision=$wahrwelt_snapshot_recovery_public_path identity=${wahrwelt_snapshot_recovery_public_identity:-absent}"
  return 1
}

wahrwelt_verify_exact_snapshot() {
  local snapshot_dir="$1"
  local fd expected parent_fd parent_expected name parent_pinned

  fd="${wahrwelt_snapshot_directory_fds[$snapshot_dir]:-}"
  expected="${wahrwelt_snapshot_directory_identities[$snapshot_dir]:-}"
  parent_fd="${wahrwelt_snapshot_directory_parent_fds[$snapshot_dir]:-}"
  parent_expected="${wahrwelt_snapshot_directory_parent_identities[$snapshot_dir]:-}"
  name="${wahrwelt_snapshot_directory_names[$snapshot_dir]:-}"
  [ -n "$fd" ] && [ -n "$expected" ] && [ -n "$parent_fd" ] && [ -n "$parent_expected" ] && [ -n "$name" ] || return 1
  parent_pinned="/proc/${BASHPID:-$$}/fd/$parent_fd"
  [ "$(runtime_directory_identity "/proc/${BASHPID:-$$}/fd/$fd" 2>/dev/null || true)" = "$expected" ] &&
    [ "$(runtime_directory_identity "$parent_pinned" 2>/dev/null || true)" = "$parent_expected" ] &&
    [ "$(runtime_directory_identity "$parent_pinned/$name" 2>/dev/null || true)" = "$expected" ]
}

snapshot_leaf_key() {
  printf '%s:%s' "$1" "$2"
}

snapshot_write_field() {
  snapshot_write_fields "$@"
}

snapshot_write_fields() {
  local snapshot_dir="$1"
  local fd output name key index=0
  local -a fields=("${@:2}") identities=()

  [ "$((${#fields[@]} % 2))" -eq 0 ] || return 1
  fd="$(wahrwelt_snapshot_directory_fd "$snapshot_dir")"
  [ -n "$fd" ] || return 1
  output="$(
    python3 -I -S - "$fd" "${fields[@]}" 2>/dev/null <<'PY'
import hashlib
import os
import stat
import sys

parent_fd = int(sys.argv[1])
fields = sys.argv[2:]
if len(fields) % 2:
    raise OSError("invalid snapshot field batch")
for raw_name, raw_value in zip(fields[::2], fields[1::2]):
    name = os.fsencode(raw_name)
    value = os.fsencode(raw_value)
    flags = os.O_WRONLY | os.O_CLOEXEC | os.O_NOFOLLOW | os.O_CREAT | os.O_EXCL
    fd = os.open(name, flags, 0o600, dir_fd=parent_fd)
    try:
        offset = 0
        while offset < len(value):
            offset += os.write(fd, value[offset:])
        os.fsync(fd)
        info = os.fstat(fd)
        if not stat.S_ISREG(info.st_mode) or info.st_nlink != 1:
            raise OSError("snapshot field is not a private regular file")
        print(":".join(map(str, (
            info.st_dev, info.st_ino, info.st_mode, info.st_nlink, info.st_size,
            info.st_mtime_ns, info.st_ctime_ns,
        ))) + ":" + hashlib.sha256(value).hexdigest())
    finally:
        os.close(fd)
PY
  )" || return 1
  mapfile -t identities <<<"$output"
  [ "${#identities[@]}" -eq "$((${#fields[@]} / 2))" ] || return 1
  while [ "$index" -lt "${#fields[@]}" ]; do
    name="${fields[$index]}"
    key="$(snapshot_leaf_key "$snapshot_dir" "$name")"
    wahrwelt_snapshot_leaf_identities["$key"]="${identities[$((index / 2))]}"
    index=$((index + 2))
  done
}

snapshot_has_field() {
  local key

  key="$(snapshot_leaf_key "$1" "$2")"
  [ -n "${wahrwelt_snapshot_leaf_identities[$key]+x}" ]
}

snapshot_read_field() {
  local snapshot_dir="$1"
  local name="$2"
  local limit="${3:-16777216}"
  local fd key expected

  fd="$(wahrwelt_snapshot_directory_fd "$snapshot_dir")"
  key="$(snapshot_leaf_key "$snapshot_dir" "$name")"
  expected="${wahrwelt_snapshot_leaf_identities[$key]:-}"
  [ -n "$fd" ] && [ -n "$expected" ] || return 1
  python3 -I -S - "$fd" "$name" "$expected" "$limit" <<'PY'
import hashlib
import os
import stat
import sys

parent_fd = int(sys.argv[1])
name = os.fsencode(sys.argv[2])
expected = sys.argv[3]
limit = int(sys.argv[4])
flags = os.O_RDONLY | os.O_CLOEXEC | os.O_NOFOLLOW | os.O_NONBLOCK
fd = os.open(name, flags, dir_fd=parent_fd)
try:
    before = os.fstat(fd)
    if not stat.S_ISREG(before.st_mode) or before.st_nlink != 1:
        raise OSError("snapshot field is not a private regular file")
    chunks = []
    remaining = limit + 1
    while remaining:
        chunk = os.read(fd, min(1024 * 1024, remaining))
        if not chunk:
            break
        chunks.append(chunk)
        remaining -= len(chunk)
    value = b"".join(chunks)
    if len(value) > limit:
        raise OSError("snapshot field is too large")
    after = os.fstat(fd)
    identity = ":".join(map(str, (
        after.st_dev, after.st_ino, after.st_mode, after.st_nlink, after.st_size,
        after.st_mtime_ns, after.st_ctime_ns,
    ))) + ":" + hashlib.sha256(value).hexdigest()
    stable = lambda info: (
        info.st_dev, info.st_ino, info.st_mode, info.st_nlink, info.st_size,
        info.st_mtime_ns, info.st_ctime_ns,
    )
    if stable(before) != stable(after) or identity != expected:
        raise OSError("snapshot field changed before anchored read")
    sys.stdout.buffer.write(value)
finally:
    os.close(fd)
PY
}

wahrwelt_register_exact_snapshot() {
  local snapshot_dir="$1"
  local existing

  wahrwelt_verify_exact_snapshot "$snapshot_dir" || return 1
  for existing in "${wahrwelt_transaction_snapshot_dirs[@]}"; do
    [ "$existing" = "$snapshot_dir" ] && return 0
  done
  wahrwelt_transaction_snapshot_dirs+=("$snapshot_dir")
}

wahrwelt_unregister_exact_snapshot() {
  local snapshot_dir="$1"
  local existing key fd
  local -a retained=()

  for existing in "${wahrwelt_transaction_snapshot_dirs[@]}"; do
    [ "$existing" = "$snapshot_dir" ] || retained+=("$existing")
  done
  wahrwelt_transaction_snapshot_dirs=("${retained[@]}")
  for key in "${!wahrwelt_snapshot_parent_fds[@]}"; do
    case "$key" in
      "$snapshot_dir":*)
        fd="${wahrwelt_snapshot_parent_fds[$key]}"
        exec {fd}<&-
        unset 'wahrwelt_snapshot_parent_fds[$key]' 'wahrwelt_snapshot_parent_identities[$key]'
        ;;
    esac
  done
  for key in "${!wahrwelt_snapshot_leaf_identities[@]}"; do
    case "$key" in "$snapshot_dir":*) unset 'wahrwelt_snapshot_leaf_identities[$key]' ;; esac
  done
  for key in "${!wahrwelt_snapshot_owned_types[@]}"; do
    case "$key" in
      "$snapshot_dir":*)
        unset 'wahrwelt_snapshot_owned_types[$key]' 'wahrwelt_snapshot_owned_identities[$key]' \
          'wahrwelt_snapshot_owned_parents[$key]' 'wahrwelt_snapshot_owned_recoveries[$key]'
        ;;
    esac
  done
  for key in "${!wahrwelt_snapshot_paths[@]}"; do
    case "$key" in
      "$snapshot_dir":*)
        unset 'wahrwelt_snapshot_paths[$key]' 'wahrwelt_snapshot_original_types[$key]' \
          'wahrwelt_snapshot_original_identities[$key]' 'wahrwelt_snapshot_original_parents[$key]'
        ;;
    esac
  done
  fd="${wahrwelt_snapshot_directory_fds[$snapshot_dir]:-}"
  [ -z "$fd" ] || exec {fd}<&-
  fd="${wahrwelt_snapshot_directory_parent_fds[$snapshot_dir]:-}"
  [ -z "$fd" ] || exec {fd}<&-
  unset 'wahrwelt_snapshot_directory_fds[$snapshot_dir]' 'wahrwelt_snapshot_directory_identities[$snapshot_dir]' \
    'wahrwelt_snapshot_directory_parent_fds[$snapshot_dir]' 'wahrwelt_snapshot_directory_parent_identities[$snapshot_dir]' \
    'wahrwelt_snapshot_directory_names[$snapshot_dir]'
}

snapshot_parent_key() {
  printf '%s:%s' "$1" "$2"
}

snapshot_pin_runtime_parent() {
  local snapshot_dir="$1"
  local index="$2"
  local path="$3"
  local expected_parent="$4"
  local parent before after key fd

  parent="$(dirname -- "$path")"
  [ -d "$parent" ] && [ ! -L "$parent" ] || return 1
  before="$(runtime_directory_identity "$parent" 2>/dev/null || true)"
  [ -n "$before" ] || return 1
  exec {fd}<"$parent" || return 1
  after="$(runtime_directory_identity "/proc/$$/fd/$fd" 2>/dev/null || true)"
  if [ "$before" != "$expected_parent" ] || [ "$after" != "$before" ] ||
    ! runtime_canonical_parent_matches "$path" "$before"; then
    exec {fd}<&-
    return 1
  fi
  key="$(snapshot_parent_key "$snapshot_dir" "$index")"
  wahrwelt_snapshot_parent_fds["$key"]="$fd"
  wahrwelt_snapshot_parent_identities["$key"]="$after"
}

snapshot_pinned_runtime_path() {
  local snapshot_dir="$1"
  local index="$2"
  local path="$3"
  local key fd

  key="$(snapshot_parent_key "$snapshot_dir" "$index")"
  fd="${wahrwelt_snapshot_parent_fds[$key]:-}"
  [ -n "$fd" ] || return 1
  printf '/proc/%s/fd/%s/%s' "$$" "$fd" "${path##*/}"
}

snapshot_pinned_parent_identity() {
  local snapshot_dir="$1"
  local index="$2"
  local key

  key="$(snapshot_parent_key "$snapshot_dir" "$index")"
  printf '%s' "${wahrwelt_snapshot_parent_identities[$key]:-}"
}

snapshot_runtime_recovery_path() {
  local snapshot_dir="$1"
  local index="$2"
  local name="$3"
  local key fd parent

  key="$(snapshot_parent_key "$snapshot_dir" "$index")"
  fd="${wahrwelt_snapshot_parent_fds[$key]:-}"
  [ -n "$fd" ] || return 1
  parent="$(readlink -- "/proc/${BASHPID:-$$}/fd/$fd" 2>/dev/null || true)"
  [ -n "$parent" ] || return 1
  printf '%s/%s' "$parent" "$name"
}

transaction_pinned_snapshot_path() {
  local path="$1"
  local snapshot_dir index anchored parent

  for snapshot_dir in "${wahrwelt_transaction_snapshot_dirs[@]}"; do
    index="$(snapshot_path_index "$snapshot_dir" "$path" 2>/dev/null || true)"
    [ -n "$index" ] || continue
    anchored="$(snapshot_pinned_runtime_path "$snapshot_dir" "$index" "$path" 2>/dev/null || true)"
    parent="$(snapshot_pinned_parent_identity "$snapshot_dir" "$index")"
    [ -n "$anchored" ] && [ -n "$parent" ] || continue
    printf '%s\t%s\t%s\t%s' "$snapshot_dir" "$index" "$anchored" "$parent"
    return 0
  done
  return 1
}

snapshot_path_index() {
  local snapshot_dir="$1"
  local path="$2"
  local index=0 key

  while :; do
    key="$(snapshot_parent_key "$snapshot_dir" "$index")"
    [ -n "${wahrwelt_snapshot_paths[$key]+x}" ] || return 1
    if [ "${wahrwelt_snapshot_paths[$key]}" = "$path" ]; then
      printf '%s' "$index"
      return 0
    fi
    index=$((index + 1))
  done
}

snapshot_expected_state() {
  local snapshot_dir="$1"
  local index="$2"
  local type identity parent key

  key="$(snapshot_parent_key "$snapshot_dir" "$index")"
  if [ -n "${wahrwelt_snapshot_owned_types[$key]+x}" ]; then
    type="${wahrwelt_snapshot_owned_types[$key]}"
    identity="${wahrwelt_snapshot_owned_identities[$key]}"
    parent="${wahrwelt_snapshot_owned_parents[$key]}"
  else
    [ -n "${wahrwelt_snapshot_paths[$key]+x}" ] || return 1
    type="${wahrwelt_snapshot_original_types[$key]}"
    identity="${wahrwelt_snapshot_original_identities[$key]}"
    parent="${wahrwelt_snapshot_original_parents[$key]}"
  fi
  printf '%s\n%s\n%s\n' "$type" "$identity" "$parent"
}

record_exact_snapshot_mutation() {
  local path="$1"
  local type="$2"
  local identity="$3"
  local parent="$4"
  local recovery="${5:-}"
  local snapshot_dir index key

  for snapshot_dir in "${wahrwelt_transaction_snapshot_dirs[@]}"; do
    index="$(snapshot_path_index "$snapshot_dir" "$path" 2>/dev/null || true)"
    [ -n "$index" ] || continue
    key="$(snapshot_parent_key "$snapshot_dir" "$index")"
    wahrwelt_snapshot_owned_types["$key"]="$type"
    wahrwelt_snapshot_owned_identities["$key"]="$identity"
    wahrwelt_snapshot_owned_parents["$key"]="$parent"
    wahrwelt_snapshot_owned_recoveries["$key"]="$recovery"
  done
}

preflight_exact_transaction_path() {
  local path="$1"
  local actual_path="${2:-$path}"
  local actual_parent="${3:-}"
  local snapshot_dir index current_type current_identity current_parent
  local expected_type expected_identity expected_parent
  local -a expected=()

  for snapshot_dir in "${wahrwelt_transaction_snapshot_dirs[@]}"; do
    index="$(snapshot_path_index "$snapshot_dir" "$path" 2>/dev/null || true)"
    [ -n "$index" ] || continue
    mapfile -t expected < <(snapshot_expected_state "$snapshot_dir" "$index")
    expected_type="${expected[0]:-}"
    expected_identity="${expected[1]:-}"
    expected_parent="${expected[2]:-}"
    current_type="$(runtime_path_kind "$actual_path")"
    current_identity="$(runtime_state_identity "$actual_path" 2>/dev/null || true)"
    current_parent="$actual_parent"
    if [ -z "$current_parent" ]; then
      current_parent="$(runtime_parent_identity "$actual_path" 2>/dev/null || true)"
    fi
    if [ "$current_type" != "$expected_type" ] ||
      { [ "$current_type" != absent ] && [ "$current_identity" != "$expected_identity" ]; } ||
      { [ -n "$expected_parent" ] && [ "$current_parent" != "$expected_parent" ]; }; then
      log "transaction path changed before mutation; preserving concurrent winner: $path"
      return 1
    fi
  done
}

transaction_path_requires_missing_parent_fail_closed() {
  local path="$1"
  local snapshot_dir index
  local -a expected=()

  for snapshot_dir in "${wahrwelt_transaction_snapshot_dirs[@]}"; do
    index="$(snapshot_path_index "$snapshot_dir" "$path" 2>/dev/null || true)"
    [ -n "$index" ] || continue
    mapfile -t expected < <(snapshot_expected_state "$snapshot_dir" "$index")
    [ -n "${expected[2]:-}" ] || return 0
  done
  return 1
}

runtime_pinned_parent_fd=""
runtime_pinned_parent_identity=""
runtime_pinned_parent_path=""

open_pinned_runtime_parent() {
  local path="$1"
  local parent before after

  runtime_pinned_parent_fd=""
  runtime_pinned_parent_identity=""
  runtime_pinned_parent_path=""
  parent="$(dirname -- "$path")"
  if [ ! -e "$parent" ] && [ ! -L "$parent" ]; then
    if transaction_path_requires_missing_parent_fail_closed "$path"; then
      log "runtime parent was absent at transaction begin; refusing unanchored publication: $parent"
      return 1
    fi
    mkdir -p -- "$parent" || return 1
  fi
  [ -d "$parent" ] && [ ! -L "$parent" ] || {
    log "refusing non-directory runtime parent: $parent"
    return 1
  }
  before="$(runtime_directory_identity "$parent" 2>/dev/null || true)"
  [ -n "$before" ] || return 1
  exec {runtime_pinned_parent_fd}<"$parent" || return 1
  after="$(runtime_directory_identity "/proc/$$/fd/$runtime_pinned_parent_fd" 2>/dev/null || true)"
  if [ "$after" != "$before" ]; then
    exec {runtime_pinned_parent_fd}<&-
    runtime_pinned_parent_fd=""
    log "runtime parent changed while pinning; preserving concurrent winner: $parent"
    return 1
  fi
  runtime_pinned_parent_identity="$after"
  runtime_pinned_parent_path="/proc/$$/fd/$runtime_pinned_parent_fd"
}

close_pinned_runtime_parent() {
  [ -n "$runtime_pinned_parent_fd" ] || return 0
  exec {runtime_pinned_parent_fd}<&-
  runtime_pinned_parent_fd=""
  runtime_pinned_parent_identity=""
  runtime_pinned_parent_path=""
}

runtime_canonical_parent_matches() {
  local path="$1"
  local expected_identity="$2"
  local parent current

  parent="$(dirname -- "$path")"
  [ -d "$parent" ] && [ ! -L "$parent" ] || return 1
  current="$(runtime_directory_identity "$parent" 2>/dev/null || true)"
  [ "$current" = "$expected_identity" ]
}

quarantine_exact_runtime_path() {
  local path="$1"
  local expected_identity="$2"
  local parent base placeholder placeholder_recovery attempt moved_identity placeholder_identity current_identity
  local fd placeholder_fd pinned

  parent="$(dirname -- "$path")"
  base="$(basename -- "$path")"
  [ -d "$parent" ] || return 1
  declare -F wahrwelt_exchange_pinned_names >/dev/null 2>&1 || return 1
  exec {fd}<"$parent" || return 1
  pinned="/proc/${BASHPID:-$$}/fd/$fd"
  if [ "$(runtime_state_identity "$pinned/$base" 2>/dev/null || true)" != "$expected_identity" ]; then
    exec {fd}<&-
    return 1
  fi
  for attempt in $(seq 1 32); do
    if ! wahrwelt_create_pinned_private_directory "$fd" prefix .wahrwelt-runtime-quarantine- runtime-quarantine; then
      continue
    fi
    placeholder="$wahrwelt_created_directory_name"
    placeholder_identity="$wahrwelt_created_directory_identity"
    placeholder_fd="$wahrwelt_created_directory_fd"
    if declare -F wahrwelt_before_runtime_quarantine_exchange_hook >/dev/null 2>&1; then
      wahrwelt_before_runtime_quarantine_exchange_hook "$path" || {
        exec {placeholder_fd}<&-
        exec {fd}<&-
        return 1
      }
    fi
    if ! wahrwelt_exchange_pinned_names "$fd" "$base" "$placeholder" 2>/dev/null; then
      exec {placeholder_fd}<&-
      continue
    fi
    moved_identity="$(runtime_state_identity "$pinned/$placeholder" 2>/dev/null || true)"
    current_identity="$(runtime_directory_identity "$pinned/$base" 2>/dev/null || true)"
    if [ "$moved_identity" != "$expected_identity" ] || [ "$current_identity" != "$placeholder_identity" ]; then
      if [ "$current_identity" = "$placeholder_identity" ]; then
        wahrwelt_exchange_pinned_names "$fd" "$base" "$placeholder" 2>/dev/null || true
      fi
      exec {placeholder_fd}<&-
      exec {fd}<&-
      return 1
    fi
    placeholder_recovery=".wahrwelt-runtime-placeholder-${BASHPID:-$$}-${RANDOM}-$attempt"
    if ! wahrwelt_rename_noreplace_pinned_names "$fd" "$base" "$placeholder_recovery" 2>/dev/null; then
      wahrwelt_exchange_pinned_names "$fd" "$base" "$placeholder" 2>/dev/null || true
      exec {placeholder_fd}<&-
      continue
    fi
    current_identity="$(runtime_directory_identity "$pinned/$placeholder_recovery" 2>/dev/null || true)"
    if [ "$current_identity" = "$placeholder_identity" ] && [ "$(runtime_path_kind "$pinned/$base")" = absent ]; then
      exec {placeholder_fd}<&-
      exec {fd}<&-
      printf '%s' "$parent/$placeholder"
      return 0
    fi
    if [ "$(runtime_path_kind "$pinned/$base")" = absent ]; then
      wahrwelt_rename_noreplace_pinned_names "$fd" "$placeholder_recovery" "$base" 2>/dev/null || true
    fi
    exec {placeholder_fd}<&-
    exec {fd}<&-
    return 1
  done
  exec {fd}<&-
  return 1
}

snapshot_capture_runtime_leaf() {
  local snapshot_dir="$1"
  local index="$2"
  local runtime_parent_fd="$3"
  local base="$4"
  local snapshot_fd record type identity mode value_identity extra key

  wahrwelt_captured_snapshot_type=""
  wahrwelt_captured_snapshot_identity=""
  wahrwelt_captured_snapshot_mode=""
  snapshot_fd="$(wahrwelt_snapshot_directory_fd "$snapshot_dir")"
  [ -n "$snapshot_fd" ] || return 1
  record="$(
    python3 -I -S - "$snapshot_fd" "$index.value" "$runtime_parent_fd" "$base" <<'PY'
import hashlib
import os
import stat
import sys

snapshot_fd = int(sys.argv[1])
value_name = os.fsencode(sys.argv[2])
runtime_parent_fd = int(sys.argv[3])
runtime_name = os.fsencode(sys.argv[4])

def write_value(value):
    flags = os.O_WRONLY | os.O_CLOEXEC | os.O_NOFOLLOW | os.O_CREAT | os.O_EXCL
    fd = os.open(value_name, flags, 0o600, dir_fd=snapshot_fd)
    try:
        offset = 0
        while offset < len(value):
            offset += os.write(fd, value[offset:])
        os.fsync(fd)
        info = os.fstat(fd)
        if not stat.S_ISREG(info.st_mode) or info.st_nlink != 1:
            raise OSError("snapshot value is not a private regular file")
        return ":".join(map(str, (
            info.st_dev, info.st_ino, info.st_mode, info.st_nlink, info.st_size,
            info.st_mtime_ns, info.st_ctime_ns,
        ))) + ":" + hashlib.sha256(value).hexdigest()
    finally:
        os.close(fd)

try:
    visible = os.stat(runtime_name, dir_fd=runtime_parent_fd, follow_symlinks=False)
except FileNotFoundError:
    print("absent|||")
    raise SystemExit(0)

if stat.S_ISLNK(visible.st_mode):
    value = os.readlink(runtime_name, dir_fd=runtime_parent_fd)
    if isinstance(value, str):
        value = os.fsencode(value)
    after = os.stat(runtime_name, dir_fd=runtime_parent_fd, follow_symlinks=False)
    if (visible.st_dev, visible.st_ino) != (after.st_dev, after.st_ino) or not stat.S_ISLNK(after.st_mode):
        raise OSError("runtime symlink changed during snapshot")
    value_identity = write_value(value)
    print(f"symlink|{after.st_dev}:{after.st_ino}||{value_identity}")
    raise SystemExit(0)

if not stat.S_ISREG(visible.st_mode):
    raise OSError("runtime snapshot source is not regular, symlink, or absent")
flags = os.O_RDONLY | os.O_CLOEXEC | os.O_NOFOLLOW | os.O_NONBLOCK
source_fd = os.open(runtime_name, flags, dir_fd=runtime_parent_fd)
try:
    before = os.fstat(source_fd)
    if not stat.S_ISREG(before.st_mode) or before.st_nlink != 1:
        raise OSError("runtime snapshot source is not a private regular file")
    chunks = []
    digest = hashlib.sha256()
    while True:
        chunk = os.read(source_fd, 1024 * 1024)
        if not chunk:
            break
        chunks.append(chunk)
        digest.update(chunk)
    after = os.fstat(source_fd)
    current = os.stat(runtime_name, dir_fd=runtime_parent_fd, follow_symlinks=False)
    stable = lambda info: (
        info.st_dev, info.st_ino, info.st_mode, info.st_nlink, info.st_size,
        info.st_mtime_ns, info.st_ctime_ns,
    )
    if stable(before) != stable(after) or (current.st_dev, current.st_ino) != (after.st_dev, after.st_ino):
        raise OSError("runtime regular source changed during snapshot")
    value = b"".join(chunks)
    value_identity = write_value(value)
    identity = f"{after.st_dev}:{after.st_ino}:{after.st_nlink}:{digest.hexdigest()}"
    print(f"regular|{identity}|{stat.S_IMODE(after.st_mode):o}|{value_identity}")
finally:
    os.close(source_fd)
PY
  )" || return 1
  IFS='|' read -r type identity mode value_identity extra <<<"$record"
  [ -z "${extra:-}" ] || return 1
  case "$type" in
    absent) [ -z "$identity" ] && [ -z "$mode" ] && [ -z "$value_identity" ] || return 1 ;;
    regular)
      [ -n "$identity" ] && [ -n "$mode" ] && [ -n "$value_identity" ] || return 1
      key="$(snapshot_leaf_key "$snapshot_dir" "$index.value")"
      wahrwelt_snapshot_leaf_identities["$key"]="$value_identity"
      ;;
    symlink)
      [ -n "$identity" ] && [ -z "$mode" ] && [ -n "$value_identity" ] || return 1
      key="$(snapshot_leaf_key "$snapshot_dir" "$index.value")"
      wahrwelt_snapshot_leaf_identities["$key"]="$value_identity"
      ;;
    *) return 1 ;;
  esac
  wahrwelt_captured_snapshot_type="$type"
  wahrwelt_captured_snapshot_identity="$identity"
  wahrwelt_captured_snapshot_mode="$mode"
}

snapshot_exact_paths() {
  local snapshot_dir="$1"
  local path type identity parent parent_path mode index=0 key runtime_parent_fd
  shift

  wahrwelt_verify_exact_snapshot "$snapshot_dir" || {
    log "refusing unproven transaction snapshot directory: $snapshot_dir"
    return 1
  }
  for path in "$@"; do
    parent_path="$(dirname -- "$path")"
    parent="$(runtime_directory_identity "$parent_path" 2>/dev/null || true)"
    if [ -z "$parent" ] || [ -L "$parent_path" ]; then
      log "runtime parent was absent or unsafe at transaction begin; refusing unanchored snapshot: $parent_path"
      return 1
    fi
    if declare -F wahrwelt_before_snapshot_parent_pin_hook >/dev/null 2>&1; then
      wahrwelt_before_snapshot_parent_pin_hook "$path" "$parent" || return 1
    fi
    if ! snapshot_pin_runtime_parent "$snapshot_dir" "$index" "$path" "$parent"; then
      log "runtime parent changed while taking transaction snapshot; preserving concurrent winner: $path"
      return 1
    fi
    key="$(snapshot_parent_key "$snapshot_dir" "$index")"
    runtime_parent_fd="${wahrwelt_snapshot_parent_fds[$key]:-}"
    [ -n "$runtime_parent_fd" ] || return 1
    snapshot_capture_runtime_leaf "$snapshot_dir" "$index" "$runtime_parent_fd" "${path##*/}" || return 1
    type="$wahrwelt_captured_snapshot_type"
    identity="$wahrwelt_captured_snapshot_identity"
    mode="$wahrwelt_captured_snapshot_mode"
    case "$type" in regular | symlink | absent) ;; *) return 1 ;; esac
    if declare -F wahrwelt_before_snapshot_leaf_write_hook >/dev/null 2>&1; then
      wahrwelt_before_snapshot_leaf_write_hook "$snapshot_dir" "$index" "$(wahrwelt_snapshot_directory_fd "$snapshot_dir")" || return 1
    fi
    if [ "$type" = regular ]; then
      snapshot_write_fields "$snapshot_dir" \
        "$index.path" "$path" \
        "$index.type" "$type" \
        "$index.identity" "$identity" \
        "$index.parent" "$parent" \
        "$index.mode" "$mode" || return 1
    else
      snapshot_write_fields "$snapshot_dir" \
        "$index.path" "$path" \
        "$index.type" "$type" \
        "$index.identity" "$identity" \
        "$index.parent" "$parent" || return 1
    fi
    wahrwelt_snapshot_paths["$key"]="$path"
    wahrwelt_snapshot_original_types["$key"]="$type"
    wahrwelt_snapshot_original_identities["$key"]="$identity"
    wahrwelt_snapshot_original_parents["$key"]="$parent"
    if ! runtime_canonical_parent_matches "$path" "$parent"; then
      log "runtime parent changed while taking transaction snapshot; preserving concurrent winner: $path"
      return 1
    fi
    index=$((index + 1))
  done
  wahrwelt_register_exact_snapshot "$snapshot_dir"
}

snapshot_value_path() {
  local fd

  fd="$(wahrwelt_snapshot_directory_fd "$1")"
  [ -n "$fd" ] || return 1
  printf '/proc/%s/fd/%s/%s.value' "$$" "$fd" "$2"
}

snapshot_value_identity() {
  local key

  key="$(snapshot_leaf_key "$1" "$2.value")"
  printf '%s' "${wahrwelt_snapshot_leaf_identities[$key]:-}"
}

snapshot_value_matches_path() {
  local snapshot_dir="$1"
  local index="$2"
  local path="$3"
  local source expected

  source="$(snapshot_value_path "$snapshot_dir" "$index")" || return 1
  expected="$(snapshot_value_identity "$snapshot_dir" "$index")"
  [ -n "$expected" ] || return 1
  python3 -I -S - "$source" "$expected" "$path" <<'PY'
import hashlib
import os
import stat
import sys

source_path = os.fsencode(sys.argv[1])
source_expected = sys.argv[2]
target_path = os.fsencode(sys.argv[3])
flags = os.O_RDONLY | os.O_CLOEXEC | os.O_NOFOLLOW | os.O_NONBLOCK
source_fd = os.open(source_path, flags)
target_fd = os.open(target_path, flags)
try:
    source_before = os.fstat(source_fd)
    target_before = os.fstat(target_fd)
    if not stat.S_ISREG(source_before.st_mode) or source_before.st_nlink != 1 or not stat.S_ISREG(target_before.st_mode):
        raise OSError("snapshot comparison requires regular files")
    source_hash = hashlib.sha256()
    while True:
        left = os.read(source_fd, 1024 * 1024)
        right = os.read(target_fd, 1024 * 1024)
        if left != right:
            raise OSError("snapshot value mismatch")
        if not left:
            break
        source_hash.update(left)
    source_after = os.fstat(source_fd)
    target_after = os.fstat(target_fd)
    source_identity = ":".join(map(str, (
        source_after.st_dev, source_after.st_ino, source_after.st_mode,
        source_after.st_nlink, source_after.st_size, source_after.st_mtime_ns,
        source_after.st_ctime_ns,
    ))) + ":" + source_hash.hexdigest()
    stable = lambda info: (
        info.st_dev, info.st_ino, info.st_mode, info.st_nlink, info.st_size,
        info.st_mtime_ns, info.st_ctime_ns,
    )
    if (
        stable(source_before) != stable(source_after)
        or stable(target_before) != stable(target_after)
        or source_identity != source_expected
    ):
        raise OSError("snapshot comparison source changed")
finally:
    os.close(target_fd)
    os.close(source_fd)
PY
}

restore_regular_snapshot() {
  local snapshot_dir="$1"
  local index="$2"
  local path="$3"
  local expected_identity="$4"
  local mode parent base fd pinned candidate candidate_identity moved_identity current_identity value value_identity

  mode="$(snapshot_read_field "$snapshot_dir" "$index.mode" 2>/dev/null || true)"
  value="$(snapshot_value_path "$snapshot_dir" "$index" 2>/dev/null || true)"
  value_identity="$(snapshot_value_identity "$snapshot_dir" "$index")"
  [ -n "$mode" ] && [ -n "$value" ] && [ -n "$value_identity" ] || return 1
  parent="$(dirname -- "$path")"
  base="${path##*/}"
  exec {fd}<"$parent" || return 1
  pinned="/proc/${BASHPID:-$$}/fd/$fd"
  if [ "$(runtime_state_identity "$pinned/$base" 2>/dev/null || true)" != "$expected_identity" ] ||
    ! runtime_regular_is_private "$pinned/$base"; then
    exec {fd}<&-
    return 1
  fi
  for _ in $(seq 1 32); do
    candidate=".wahrwelt-runtime-rollback-${BASHPID:-$$}-${RANDOM}"
    candidate_identity="$(runtime_create_regular_candidate_from_file "$fd" "$candidate" "$value" "$mode" "$value_identity" 2>/dev/null || true)"
    [ -n "$candidate_identity" ] || continue
    if ! wahrwelt_exchange_pinned_names "$fd" "$base" "$candidate" 2>/dev/null; then
      # The candidate is transaction-private but cannot be safely removed
      # after a failed exchange.  Retain it for inspection instead.
      log "failed to exchange rollback candidate; retaining recovery: $parent/$candidate"
      exec {fd}<&-
      return 1
    fi
    moved_identity="$(runtime_state_identity "$pinned/$candidate" 2>/dev/null || true)"
    current_identity="$(runtime_state_identity "$pinned/$base" 2>/dev/null || true)"
    if [ "$moved_identity" = "$expected_identity" ] && [ "$current_identity" = "$candidate_identity" ] &&
      snapshot_value_matches_path "$snapshot_dir" "$index" "$pinned/$base" &&
      [ "$(stat -c %a -- "$pinned/$base" 2>/dev/null || true)" = "$mode" ]; then
      log "transaction-owned runtime result retained at $parent/$candidate"
      exec {fd}<&-
      return 0
    fi
    if [ "$current_identity" = "$candidate_identity" ]; then
      wahrwelt_exchange_pinned_names "$fd" "$base" "$candidate" 2>/dev/null || true
    fi
    log "rollback candidate ownership changed; retaining recovery: $parent/$candidate"
    exec {fd}<&-
    return 1
  done
  exec {fd}<&-
  return 1
}

restore_absent_regular_snapshot() {
  local snapshot_dir="$1"
  local index="$2"
  local path="$3"
  local mode parent base fd pinned restored_identity value value_identity

  mode="$(snapshot_read_field "$snapshot_dir" "$index.mode" 2>/dev/null || true)"
  value="$(snapshot_value_path "$snapshot_dir" "$index" 2>/dev/null || true)"
  value_identity="$(snapshot_value_identity "$snapshot_dir" "$index")"
  [ -n "$mode" ] && [ -n "$value" ] && [ -n "$value_identity" ] || return 1
  parent="$(dirname -- "$path")"
  base="${path##*/}"
  exec {fd}<"$parent" || return 1
  pinned="/proc/${BASHPID:-$$}/fd/$fd"
  if [ "$(runtime_path_kind "$pinned/$base")" != absent ]; then
    exec {fd}<&-
    return 1
  fi
  restored_identity="$(runtime_publish_anonymous_regular "$fd" "$base" "$mode" file "$value" "$value_identity" 2>/dev/null || true)"
  if [ -z "$restored_identity" ] ||
    [ "$(runtime_state_identity "$pinned/$base" 2>/dev/null || true)" != "$restored_identity" ] ||
    ! runtime_regular_is_private "$pinned/$base" ||
    ! snapshot_value_matches_path "$snapshot_dir" "$index" "$pinned/$base" ||
    [ "$(stat -c %a -- "$pinned/$base" 2>/dev/null || true)" != "$mode" ]; then
    exec {fd}<&-
    return 1
  fi
  exec {fd}<&-
}

restore_regular_from_recovery() {
  local path="$1"
  local recovery="$2"
  local expected_current="$3"
  local expected_original="$4"
  local parent base recovery_parent recovery_base parent_identity recovery_parent_identity fd pinned

  parent="$(dirname -- "$path")"
  base="$(basename -- "$path")"
  recovery_parent="$(dirname -- "$recovery")"
  recovery_base="$(basename -- "$recovery")"
  parent_identity="$(runtime_directory_identity "$parent" 2>/dev/null || true)"
  recovery_parent_identity="$(runtime_directory_identity "$recovery_parent" 2>/dev/null || true)"
  [ -n "$parent_identity" ] && [ "$parent_identity" = "$recovery_parent_identity" ] || return 1
  exec {fd}<"$parent" || return 1
  pinned="/proc/${BASHPID:-$$}/fd/$fd"
  if [ "$(runtime_state_identity "$pinned/$base" 2>/dev/null || true)" != "$expected_current" ] ||
    [ "$(runtime_state_identity "$pinned/$recovery_base" 2>/dev/null || true)" != "$expected_original" ] ||
    ! wahrwelt_exchange_pinned_names "$fd" "$base" "$recovery_base" 2>/dev/null; then
    exec {fd}<&-
    return 1
  fi
  if [ "$(runtime_state_identity "$pinned/$base" 2>/dev/null || true)" != "$expected_original" ] ||
    [ "$(runtime_state_identity "$pinned/$recovery_base" 2>/dev/null || true)" != "$expected_current" ]; then
    if [ "$(runtime_state_identity "$pinned/$base" 2>/dev/null || true)" = "$expected_current" ]; then
      wahrwelt_exchange_pinned_names "$fd" "$base" "$recovery_base" 2>/dev/null || true
    fi
    exec {fd}<&-
    return 1
  fi
  exec {fd}<&-
}

restore_exact_paths() {
  local snapshot_dir="$1"
  local path type index=0 status=0
  local owned_type owned_identity owned_parent owned_recovery current_type current_identity current_parent
  local recovery link_target actual_path pinned_parent original_identity restored restored_type restored_identity key
  shift

  for path in "$@"; do
    type="$(snapshot_read_field "$snapshot_dir" "$index.type" 2>/dev/null || true)"
    if [ "$type" != regular ] && [ "$type" != symlink ] && [ "$type" != absent ]; then
      log "transaction snapshot metadata is missing for path: $path"
      status=1
      index=$((index + 1))
      continue
    fi
    key="$(snapshot_parent_key "$snapshot_dir" "$index")"
    if [ -z "${wahrwelt_snapshot_owned_types[$key]+x}" ]; then
      index=$((index + 1))
      continue
    fi
    actual_path="$(snapshot_pinned_runtime_path "$snapshot_dir" "$index" "$path" 2>/dev/null || true)"
    pinned_parent="$(snapshot_pinned_parent_identity "$snapshot_dir" "$index")"
    if [ -z "$actual_path" ] || [ -z "$pinned_parent" ]; then
      log "transaction parent is no longer pinned for rollback; preserving recovery: $path"
      status=1
      index=$((index + 1))
      continue
    fi
    owned_type="${wahrwelt_snapshot_owned_types[$key]}"
    owned_identity="${wahrwelt_snapshot_owned_identities[$key]}"
    owned_parent="${wahrwelt_snapshot_owned_parents[$key]}"
    owned_recovery="${wahrwelt_snapshot_owned_recoveries[$key]}"
    current_type="$(runtime_path_kind "$actual_path")"
    current_identity="$(runtime_state_identity "$actual_path" 2>/dev/null || true)"
    current_parent="$pinned_parent"
    if [ "$current_type" != "$owned_type" ] ||
      { [ "$current_type" != absent ] && [ "$current_identity" != "$owned_identity" ]; } ||
      [ "$current_parent" != "$owned_parent" ]; then
      log "transaction-owned result changed; preserving concurrent winner and recovery: $path"
      status=1
      index=$((index + 1))
      continue
    fi

    restored=1
    case "$type" in
      absent)
        if [ "$owned_type" != absent ]; then
          recovery="$(quarantine_exact_runtime_path "$actual_path" "$owned_identity" 2>/dev/null || true)"
          if [ -z "$recovery" ]; then
            log "failed to restore prior absence without deleting a concurrent winner: $path"
            status=1
          else
            log "transaction-owned runtime result retained at $recovery"
            restored=0
          fi
        else
          restored=0
        fi
        ;;
      regular)
        if [ "$owned_type" = absent ]; then
          if ! restore_absent_regular_snapshot "$snapshot_dir" "$index" "$actual_path"; then
            log "failed to restore original regular runtime path without replacement: $path"
            status=1
          else
            restored=0
          fi
        elif [ -n "$owned_recovery" ]; then
          original_identity="$(snapshot_read_field "$snapshot_dir" "$index.identity" 2>/dev/null || true)"
          if ! restore_regular_from_recovery "$actual_path" "$owned_recovery" "$owned_identity" "$original_identity"; then
            log "failed to restore exact transaction-owned runtime recovery without replacement: $path"
            status=1
          else
            restored=0
          fi
        elif ! restore_regular_snapshot "$snapshot_dir" "$index" "$actual_path" "$owned_identity"; then
          log "failed to restore exact transaction-owned runtime regular file: $path"
          status=1
        else
          restored=0
        fi
        ;;
      symlink)
        if [ "$owned_type" != absent ]; then
          recovery="$(quarantine_exact_runtime_path "$actual_path" "$owned_identity" 2>/dev/null || true)"
          if [ -z "$recovery" ]; then
            log "failed to quarantine transaction-owned runtime result before symlink restore: $path"
            status=1
            index=$((index + 1))
            continue
          fi
        fi
        link_target="$(
          snapshot_read_field "$snapshot_dir" "$index.value" 2>/dev/null
          printf '.'
        )" || link_target=""
        link_target="${link_target%.}"
        if ! ln -s -- "$link_target" "$actual_path" 2>/dev/null; then
          log "failed to restore original runtime symlink without replacement: $path"
          status=1
        else
          restored=0
        fi
        ;;
    esac
    if [ "$restored" -eq 0 ]; then
      restored_type="$(runtime_path_kind "$actual_path")"
      restored_identity="$(runtime_state_identity "$actual_path" 2>/dev/null || true)"
      if ! record_exact_snapshot_mutation "$path" "$restored_type" "$restored_identity" "$pinned_parent"; then
        log "failed to update enclosing transaction ownership after rollback: $path"
        status=1
      fi
    fi
    index=$((index + 1))
  done
  return "$status"
}

cleanup_committed_runtime_stage_recoveries() {
  local snapshot_dir="$1"
  local index=0 path type identity parent owned_type owned_identity owned_parent recovery
  local base recovery_parent_identity pinned_path pinned_parent parent_fd key

  while snapshot_has_field "$snapshot_dir" "$index.path"; do
    path="$(snapshot_read_field "$snapshot_dir" "$index.path" 2>/dev/null || true)"
    type="$(snapshot_read_field "$snapshot_dir" "$index.type" 2>/dev/null || true)"
    identity="$(snapshot_read_field "$snapshot_dir" "$index.identity" 2>/dev/null || true)"
    parent="$(snapshot_read_field "$snapshot_dir" "$index.parent" 2>/dev/null || true)"
    key="$(snapshot_parent_key "$snapshot_dir" "$index")"
    owned_type="${wahrwelt_snapshot_owned_types[$key]:-}"
    owned_identity="${wahrwelt_snapshot_owned_identities[$key]:-}"
    owned_parent="${wahrwelt_snapshot_owned_parents[$key]:-}"
    recovery="${wahrwelt_snapshot_owned_recoveries[$key]:-}"
    index=$((index + 1))

    [ "$type" = regular ] && [ "$owned_type" = regular ] && [ -n "$recovery" ] || continue
    base="${recovery##*/}"
    case "$base" in
      .wahrwelt-runtime-stage-*) ;;
      *) continue ;;
    esac
    pinned_path="$(snapshot_pinned_runtime_path "$snapshot_dir" "$((index - 1))" "$path" 2>/dev/null || true)"
    pinned_parent="$(snapshot_pinned_parent_identity "$snapshot_dir" "$((index - 1))")"
    parent_fd="${wahrwelt_snapshot_parent_fds[$(snapshot_parent_key "$snapshot_dir" "$((index - 1))")]:-}"
    recovery_parent_identity="$(runtime_directory_identity "$(dirname -- "$recovery")" 2>/dev/null || true)"
    if [ -z "$pinned_path" ] || [ -z "$parent_fd" ] || [ "$parent" != "$pinned_parent" ] ||
      [ "$owned_parent" != "$pinned_parent" ] || [ "$recovery_parent_identity" != "$pinned_parent" ] ||
      ! runtime_canonical_parent_matches "$path" "$pinned_parent" ||
      [ "$(runtime_state_identity "$pinned_path" 2>/dev/null || true)" != "$owned_identity" ] ||
      [ "$(runtime_state_identity "/proc/${BASHPID:-$$}/fd/$parent_fd/$base" 2>/dev/null || true)" != "$identity" ] ||
      ! runtime_regular_is_private "/proc/${BASHPID:-$$}/fd/$parent_fd/$base"; then
      log "committed runtime stage changed before cleanup; preserving recovery: $recovery"
      return 1
    fi
    if declare -F wahrwelt_before_runtime_stage_cleanup_hook >/dev/null 2>&1; then
      wahrwelt_before_runtime_stage_cleanup_hook "$path" "/proc/${BASHPID:-$$}/fd/$parent_fd/$base" || return 1
    fi
    if [ "$(runtime_state_identity "/proc/${BASHPID:-$$}/fd/$parent_fd/$base" 2>/dev/null || true)" != "$identity" ] ||
      ! runtime_regular_is_private "/proc/${BASHPID:-$$}/fd/$parent_fd/$base"; then
      log "committed runtime stage changed before cleanup; preserving recovery: $recovery"
      return 1
    fi
    log "committed runtime stage retained because unlink-by-name is unverifiable: $recovery"
  done
}

snapshot_verify_all_fields() {
  local snapshot_dir="$1"
  local fd key name identity
  local -a expected=()

  fd="$(wahrwelt_snapshot_directory_fd "$snapshot_dir")"
  [ -n "$fd" ] || return 1
  for key in "${!wahrwelt_snapshot_leaf_identities[@]}"; do
    case "$key" in
      "$snapshot_dir":*)
        name="${key#"$snapshot_dir":}"
        identity="${wahrwelt_snapshot_leaf_identities[$key]}"
        expected+=("$name" "$identity")
        ;;
    esac
  done
  python3 -I -S - "$fd" "${expected[@]}" <<'PY'
import hashlib
import os
import stat
import sys

fd = int(sys.argv[1])
items = sys.argv[2:]
if len(items) % 2:
    raise OSError("invalid snapshot journal")
expected = dict(zip(items[::2], items[1::2]))
if set(os.listdir(fd)) != set(expected):
    raise OSError("snapshot entries changed before retention")
for name, wanted in expected.items():
    child = os.open(
        os.fsencode(name),
        os.O_RDONLY | os.O_CLOEXEC | os.O_NOFOLLOW | os.O_NONBLOCK,
        dir_fd=fd,
    )
    try:
        before = os.fstat(child)
        if not stat.S_ISREG(before.st_mode) or before.st_nlink != 1:
            raise OSError("snapshot entry is not a private regular file")
        digest = hashlib.sha256()
        while True:
            chunk = os.read(child, 1024 * 1024)
            if not chunk:
                break
            digest.update(chunk)
        after = os.fstat(child)
        actual = ":".join(map(str, (
            after.st_dev, after.st_ino, after.st_mode, after.st_nlink,
            after.st_size, after.st_mtime_ns, after.st_ctime_ns,
        ))) + ":" + digest.hexdigest()
        stable = lambda info: (
            info.st_dev, info.st_ino, info.st_mode, info.st_nlink,
            info.st_size, info.st_mtime_ns, info.st_ctime_ns,
        )
        if stable(before) != stable(after) or actual != wanted:
            raise OSError("snapshot entry changed before retention")
    finally:
        os.close(child)
PY
}

remove_exact_path_snapshot() {
  local snapshot_dir="$1"
  shift

  : "$*"
  if ! cleanup_committed_runtime_stage_recoveries "$snapshot_dir"; then
    return 1
  fi
  if ! wahrwelt_verify_exact_snapshot "$snapshot_dir" || ! snapshot_verify_all_fields "$snapshot_dir" 2>/dev/null; then
    log_snapshot_recovery "transaction snapshot changed before retention handoff; preserving recovery" "$snapshot_dir" || true
    return 1
  fi
  if declare -F wahrwelt_before_snapshot_cleanup_delete_hook >/dev/null 2>&1; then
    wahrwelt_before_snapshot_cleanup_delete_hook "$snapshot_dir" || return 1
  fi
  if ! wahrwelt_verify_exact_snapshot "$snapshot_dir" || ! snapshot_verify_all_fields "$snapshot_dir" 2>/dev/null; then
    log_snapshot_recovery "transaction snapshot changed at cleanup barrier; preserving recovery" "$snapshot_dir" || true
    return 1
  fi
  if ! log_snapshot_recovery "transaction snapshot retained because recursive unlink is unverifiable" "$snapshot_dir"; then
    return 1
  fi
  wahrwelt_unregister_exact_snapshot "$snapshot_dir"
}

preflight_regular_file_target() {
  local path="$1"
  local parent

  preflight_exact_transaction_path "$path" || return 1
  if [ -L "$path" ]; then
    log "refusing symlink state publication collision: $path"
    return 1
  elif [ -e "$path" ] && [ ! -f "$path" ]; then
    log "refusing to replace non-regular state path: $path"
    return 1
  elif [ -f "$path" ] && ! runtime_regular_is_private "$path"; then
    log "refusing hardlinked state publication collision: $path"
    return 1
  fi
  parent="$(dirname -- "$path")"
  if [ ! -e "$parent" ] && [ ! -L "$parent" ] && transaction_path_requires_missing_parent_fail_closed "$path"; then
    log "state parent was absent at transaction begin; refusing unanchored publication: $parent"
    return 1
  fi
  if [ -e "$parent" ] && { [ -L "$parent" ] || [ ! -d "$parent" ]; }; then
    log "refusing non-directory state parent: $parent"
    return 1
  fi
  mkdir -p -- "$parent"
}

write_regular_file() {
  local path="$1"
  local content="$2"
  local opened parent hook_status=0 base anchored candidate candidate_identity
  local prior_type prior_identity moved_identity current_identity recovery=""
  local snapshot_ref snapshot_path snapshot_parent snapshot_owner_dir snapshot_owner_index
  local -a snapshot_fields=()

  preflight_regular_file_target "$path" || return 1
  open_pinned_runtime_parent "$path" || return 1
  base="${path##*/}"
  anchored="$runtime_pinned_parent_path/$base"
  if ! preflight_exact_transaction_path "$path" "$anchored" "$runtime_pinned_parent_identity"; then
    close_pinned_runtime_parent
    return 1
  fi
  if declare -F wahrwelt_after_runtime_preflight_hook >/dev/null 2>&1; then
    if ! wahrwelt_after_runtime_preflight_hook "$path" "$runtime_pinned_parent_path"; then
      close_pinned_runtime_parent
      return 1
    fi
  fi
  if ! runtime_canonical_parent_matches "$path" "$runtime_pinned_parent_identity"; then
    close_pinned_runtime_parent
    log "runtime parent changed before publication; preserving concurrent winner: $path"
    return 1
  fi
  prior_type="$(runtime_path_kind "$anchored")"
  case "$prior_type" in
    absent) prior_identity="" ;;
    regular)
      prior_identity="$(runtime_state_identity "$anchored" 2>/dev/null || true)"
      if [ -z "$prior_identity" ] || ! runtime_regular_is_private "$anchored"; then
        close_pinned_runtime_parent
        log "runtime publication target is not an ordinary private regular file: $path"
        return 1
      fi
      if runtime_regular_matches_content "$runtime_pinned_parent_fd" "$base" "$content" 2>/dev/null; then
        if ! runtime_canonical_parent_matches "$path" "$runtime_pinned_parent_identity"; then
          close_pinned_runtime_parent
          log "runtime parent changed during unchanged publication check; preserving concurrent winner: $path"
          return 1
        fi
        close_pinned_runtime_parent
        return 0
      fi
      ;;
    *)
      close_pinned_runtime_parent
      log "runtime publication target changed before candidate publish; preserving winner: $path"
      return 1
      ;;
  esac
  if [ "${switch_transaction_active:-0}" -eq 1 ] ||
    [ "${#wahrwelt_transaction_snapshot_dirs[@]}" -gt 0 ]; then
    snapshot_ref="$(transaction_pinned_snapshot_path "$path" 2>/dev/null || true)"
    if [ -z "$snapshot_ref" ]; then
      close_pinned_runtime_parent
      log "runtime mutation lacks a retained transaction recovery parent: $path"
      return 1
    fi
  fi
  if [ "$prior_type" = absent ]; then
    candidate_identity="$(runtime_publish_anonymous_regular "$runtime_pinned_parent_fd" "$base" 0644 content "$content" 2>/dev/null || true)"
    if [ -z "$candidate_identity" ] ||
      [ "$(runtime_state_identity "$anchored" 2>/dev/null || true)" != "$candidate_identity" ] ||
      ! runtime_regular_is_private "$anchored"; then
      close_pinned_runtime_parent
      log "runtime publication target appeared; preserving winner: $path"
      return 1
    fi
  else
    candidate=".wahrwelt-runtime-stage-$BASHPID-$RANDOM"
    candidate_identity="$(runtime_create_regular_candidate "$runtime_pinned_parent_fd" "$candidate" "$content" 2>/dev/null || true)"
    if [ -z "$candidate_identity" ] ||
      [ "$(runtime_state_identity "$runtime_pinned_parent_path/$candidate" 2>/dev/null || true)" != "$candidate_identity" ]; then
      close_pinned_runtime_parent
      log "failed to create an exact private runtime publication candidate: $path"
      return 1
    fi
    if declare -F wahrwelt_before_runtime_candidate_exchange_hook >/dev/null 2>&1; then
      if ! wahrwelt_before_runtime_candidate_exchange_hook "$path" "$runtime_pinned_parent_path/$candidate"; then
        close_pinned_runtime_parent
        log "runtime publication was interrupted before candidate exchange: $path"
        return 1
      fi
    fi
    if ! wahrwelt_exchange_pinned_names "$runtime_pinned_parent_fd" "$base" "$candidate" 2>/dev/null; then
      close_pinned_runtime_parent
      log "runtime publication target changed before atomic replacement; preserving winner: $path"
      return 1
    fi
    moved_identity="$(runtime_state_identity "$runtime_pinned_parent_path/$candidate" 2>/dev/null || true)"
    current_identity="$(runtime_state_identity "$anchored" 2>/dev/null || true)"
    if [ "$moved_identity" != "$prior_identity" ] || [ "$current_identity" != "$candidate_identity" ]; then
      if [ "$current_identity" = "$candidate_identity" ]; then
        wahrwelt_exchange_pinned_names "$runtime_pinned_parent_fd" "$base" "$candidate" 2>/dev/null || true
      fi
      close_pinned_runtime_parent
      log "runtime publication replacement lost ownership proof; preserving recovery: $path"
      return 1
    fi
    [ -n "$snapshot_ref" ] || snapshot_ref="$(transaction_pinned_snapshot_path "$path" 2>/dev/null || true)"
    if [ -z "$snapshot_ref" ]; then
      if [ "$(runtime_state_identity "$anchored" 2>/dev/null || true)" = "$candidate_identity" ] &&
        [ "$(runtime_state_identity "$runtime_pinned_parent_path/$candidate" 2>/dev/null || true)" = "$prior_identity" ]; then
        wahrwelt_exchange_pinned_names "$runtime_pinned_parent_fd" "$base" "$candidate" 2>/dev/null || true
      fi
      close_pinned_runtime_parent
      log "runtime replacement lacks a retained transaction recovery parent: $path"
      return 1
    fi
    IFS=$'\t' read -r -a snapshot_fields <<<"$snapshot_ref"
    snapshot_owner_dir="${snapshot_fields[0]:-}"
    snapshot_owner_index="${snapshot_fields[1]:-}"
    snapshot_path="${snapshot_fields[2]:-}"
    snapshot_parent="${snapshot_fields[3]:-}"
    if [ "$snapshot_parent" != "$runtime_pinned_parent_identity" ] ||
      [ "$(runtime_state_identity "$(dirname -- "$snapshot_path")/$candidate" 2>/dev/null || true)" != "$prior_identity" ]; then
      if [ "$(runtime_state_identity "$anchored" 2>/dev/null || true)" = "$candidate_identity" ] &&
        [ "$(runtime_state_identity "$runtime_pinned_parent_path/$candidate" 2>/dev/null || true)" = "$prior_identity" ]; then
        wahrwelt_exchange_pinned_names "$runtime_pinned_parent_fd" "$base" "$candidate" 2>/dev/null || true
      fi
      close_pinned_runtime_parent
      log "runtime replacement recovery changed before journaling; preserving collision: $path"
      return 1
    fi
    recovery="$(snapshot_runtime_recovery_path "$snapshot_owner_dir" "$snapshot_owner_index" "$candidate" 2>/dev/null || true)"
    if [ -z "$recovery" ]; then
      close_pinned_runtime_parent
      log "runtime replacement recovery path cannot be resolved through its pinned parent: $path"
      return 1
    fi
  fi
  parent="$runtime_pinned_parent_identity"
  opened="$candidate_identity"
  if declare -F wahrwelt_after_runtime_publication_hook >/dev/null 2>&1; then
    wahrwelt_after_runtime_publication_hook "$path" "$opened" "$parent" || hook_status=$?
  fi
  if ! record_exact_snapshot_mutation "$path" regular "$opened" "$parent" "$recovery"; then
    if [ "$prior_type" = regular ]; then
      if [ "$(runtime_state_identity "$anchored" 2>/dev/null || true)" = "$candidate_identity" ] &&
        [ "$(runtime_state_identity "$runtime_pinned_parent_path/$candidate" 2>/dev/null || true)" = "$prior_identity" ]; then
        wahrwelt_exchange_pinned_names "$runtime_pinned_parent_fd" "$base" "$candidate" 2>/dev/null || true
      fi
    else
      quarantine_exact_runtime_path "$anchored" "$candidate_identity" >/dev/null 2>&1 || true
    fi
    close_pinned_runtime_parent
    log "failed to record transaction-owned runtime result; restored or retained recovery: $path"
    return 1
  fi
  if ! runtime_canonical_parent_matches "$path" "$parent"; then
    close_pinned_runtime_parent
    log "runtime parent changed after publication; preserving concurrent winner: $path"
    return 1
  fi
  if [ "$(runtime_regular_inode "$anchored" 2>/dev/null || true)" != "$opened" ]; then
    close_pinned_runtime_parent
    log "runtime publication target changed before ownership record completed; preserving winner: $path"
    return 1
  fi
  close_pinned_runtime_parent
  return "$hook_status"
}

write_regular_file_if_changed() {
  local path="$1"
  local content="$2"
  local base anchored status=1

  if runtime_path_matches_content "$path" "$content"; then
    if preflight_regular_file_target "$path" && open_pinned_runtime_parent "$path"; then
      base="${path##*/}"
      anchored="$runtime_pinned_parent_path/$base"
      if preflight_exact_transaction_path "$path" "$anchored" "$runtime_pinned_parent_identity" &&
        runtime_canonical_parent_matches "$path" "$runtime_pinned_parent_identity" &&
        runtime_regular_matches_content "$runtime_pinned_parent_fd" "$base" "$content" 2>/dev/null; then
        status=0
      fi
      close_pinned_runtime_parent
    fi
  fi
  [ "$status" -eq 0 ] && return 0
  write_regular_file "$path" "$content"
}

runtime_file() {
  wahrwelt_runtime_file "$1"
}

runtime_shell_profile_content() {
  local dir

  dir="$(hypr_dir)"
  printf '%s' "-- Runtime shell launcher
hl.on(\"hyprland.start\", function()
    hl.exec_cmd(\"$dir/scripts/start-shell.sh\")
end)"
}

runtime_shell_launcher_content() {
  local launcher_module launcher_profile

  launcher_module="$(wahrwelt_shell_launcher_module "$profile")" || return 1
  launcher_profile="$profile"
  if [ "$(wahrwelt_shell_family "$profile")" = end4 ]; then
    launcher_profile=end4
  fi
  printf '%s' "-- Active shell launcher profile: $launcher_profile
require(\"$launcher_module\")"
}

runtime_shell_keybinds_content() {
  local adapter quickshell_path

  adapter="$(wahrwelt_shell_adapter "$profile")" || return 1
  if [ "$(wahrwelt_shell_family "$profile")" = end4 ]; then
    quickshell_path="$(wahrwelt_end4_quickshell_path "$profile")" || return 1
    printf '%s' "-- Wahrwelt shell adapter: $profile
require(\"$adapter\").load({ profile = \"$profile\", quickshell_config = \"$quickshell_path\" })"
    return 0
  fi
  printf '%s' "-- Wahrwelt shell adapter: $profile
require(\"$adapter\")"
}

runtime_hyprlock_content() {
  local dir

  dir="$(hypr_dir)"
  if [ "$(wahrwelt_shell_family "$profile")" = end4 ]; then
    printf '%s' "# Active Hyprlock profile: end4
source = $dir/end4/hyprlock.conf"
    return 0
  fi
  printf '%s' '# Active Hyprlock profile: shell-managed
# Caelestia and Noctalia use shell-native lock flows.'
}

runtime_hypridle_content() {
  local dir

  dir="$(hypr_dir)"
  if [ "$(wahrwelt_shell_family "$profile")" = end4 ]; then
    printf '%s' "# Active Hypridle profile: end4
source = $dir/end4/hypridle.conf"
    return 0
  fi
  printf '%s' '# Active Hypridle profile: shell-managed
# Caelestia and Noctalia use shell-native idle flows.'
}

ensure_wahrwelt_entrypoint() {
  local dir target

  dir="$(hypr_dir)"
  target="$dir/user/hyprland.lua"

  if [ -f "$target" ]; then
    return 0
  fi

  log "wahrwelt hypr entrypoint missing; rebuild or rerun wahrwelt apply: $target"
  return 1
}

sync_shell_launcher() {
  local content

  mkdir -p -- "$hypr_runtime_dir"
  content="$(runtime_shell_profile_content)" || return 1
  write_regular_file_if_changed "$(runtime_file shell-profile.lua)" "$content"
}

sync_shell_launcher_bindings() {
  local content dir profile_launcher launcher_module

  dir="$(hypr_dir)"
  launcher_module="$(wahrwelt_shell_launcher_module "$profile")" || return 1
  profile_launcher="$dir/${launcher_module%%.*}/launcher.lua"

  if [ ! -f "$profile_launcher" ]; then
    log "shell launcher profile missing: $profile_launcher"
    return 1
  fi

  content="$(runtime_shell_launcher_content)" || return 1
  write_regular_file_if_changed "$(runtime_file shell-launcher.lua)" "$content"
}

sync_shell_keybinds() {
  local content

  content="$(runtime_shell_keybinds_content)" || return 1
  write_regular_file_if_changed "$(runtime_file shell-keybinds.lua)" "$content"
}

sync_hypr_entrypoint() {
  local content

  content="$(wahrwelt_print_canonical_runtime_entrypoint)" || return 1

  write_regular_file_if_changed "$(runtime_file hyprland.lua)" "$content"
}

sync_hypr_lock_stack() {
  local content dir hyprlock_target hypridle_target

  dir="$(hypr_dir)"

  if [ "$(wahrwelt_shell_family "$profile")" = "end4" ]; then
    hyprlock_target="$dir/end4/hyprlock.conf"
    hypridle_target="$dir/end4/hypridle.conf"

    if [ ! -f "$hyprlock_target" ]; then
      log "hyprlock entrypoint missing for profile=$profile path=$hyprlock_target"
      return 1
    fi

    if [ ! -f "$hypridle_target" ]; then
      log "hypridle entrypoint missing for profile=$profile path=$hypridle_target"
      return 1
    fi

    content="$(runtime_hyprlock_content)" || return 1
    write_regular_file_if_changed "$(runtime_file hyprlock.conf)" "$content" || return 1
    content="$(runtime_hypridle_content)" || return 1
    write_regular_file_if_changed "$(runtime_file hypridle.conf)" "$content"
    return $?
  fi

  content="$(runtime_hyprlock_content)" || return 1
  write_regular_file_if_changed "$(runtime_file hyprlock.conf)" "$content" || return 1
  content="$(runtime_hypridle_content)" || return 1
  write_regular_file_if_changed "$(runtime_file hypridle.conf)" "$content"
}

sync_stable_lua_entrypoint() {
  local dir target content

  dir="$(hypr_dir)"
  target="$dir/hyprland.lua"
  if [ -r "$target" ] &&
    wahrwelt_print_stable_runtime_entrypoint "$hypr_runtime_dir/hyprland.lua" | cmp -s - "$target"; then
    return 0
  fi

  if [ -e "$target" ] || [ -L "$target" ]; then
    if [ -L "$target" ] || [ ! -f "$target" ]; then
      log "refusing unowned top-level Hyprland runtime collision: $target"
      return 1
    fi
    if ! wahrwelt_is_canonical_entrypoint "$target" &&
      ! wahrwelt_is_legacy_user_entrypoint "$target" &&
      ! wahrwelt_is_legacy_direct_end4_entrypoint "$target" "${XDG_CONFIG_HOME:-$HOME/.config}"; then
      log "refusing unowned top-level Hyprland runtime collision: $target"
      return 1
    fi
  fi

  content="$(wahrwelt_print_stable_runtime_entrypoint "$hypr_runtime_dir/hyprland.lua")" || return 1
  write_regular_file "$target" "$content"
}

legacy_hyprland_runtime_paths() {
  local dir

  dir="$(hypr_dir)"
  printf '%s\n' \
    "$dir/hyprland.conf" \
    "$dir/shell-profile.conf" \
    "$dir/shell-launcher.conf" \
    "$dir/shell-keybinds.conf"
  if [ -e "$dir/wahrwelt" ] || [ -L "$dir/wahrwelt" ]; then
    printf '%s\n' "$dir/wahrwelt/hyprland.conf"
  fi
  printf '%s\n' \
    "$(runtime_file hyprland.conf)" \
    "$(runtime_file shell-profile.conf)" \
    "$(runtime_file shell-launcher.conf)" \
    "$(runtime_file shell-keybinds.conf)"
}

legacy_runtime_payload_matches() {
  local path="$1"
  local name profile namespace runtime target

  [ -f "$path" ] && [ ! -L "$path" ] || return 1
  name="${path##*/}"
  runtime="$hypr_runtime_dir"
  case "$name" in
    hyprland.conf)
      wahrwelt_print_stable_runtime_entrypoint "$runtime/hyprland.conf" | cmp -s - "$path" && return 0
      for profile in caelestia noctalia end4 end4-pc; do
        for namespace in mysetup wahrwelt; do
          {
            printf '# Active Hyprland profile: %s (%s)\n' "$namespace" "$profile"
            printf 'source = %s\n' "$(hypr_dir)/$namespace/hyprland.conf"
            printf 'source = %s\n' "$runtime/shell-profile.conf"
          } | cmp -s - "$path" && return 0
        done
        case "$profile" in
          end4 | end4-pc)
            {
              printf '# Active Hyprland profile: %s\n' "$profile"
              printf 'source = %s\n' "$(hypr_dir)/end4/hyprland.conf"
              printf 'source = %s\n' "$runtime/shell-profile.conf"
            } | cmp -s - "$path" && return 0
            ;;
        esac
      done
      ;;
    shell-profile.conf)
      {
        printf '# Runtime shell launcher\n'
        printf 'exec-once = %s\n' "$(hypr_dir)/scripts/start-shell.sh"
      } | cmp -s - "$path" && return 0
      ;;
    shell-launcher.conf)
      for profile in caelestia noctalia end4 end4-pc; do
        {
          printf '# Active shell launcher profile: %s\n' "$profile"
          printf 'source = %s\n' "$(hypr_dir)/$profile/launcher.conf"
        } | cmp -s - "$path" && return 0
      done
      ;;
    shell-keybinds.conf)
      for profile in caelestia noctalia end4 end4-pc; do
        {
          printf '# Active shell keybind profile: %s\n' "$profile"
          printf 'source = %s\n' "$(hypr_dir)/$profile/keybinds.conf"
        } | cmp -s - "$path" && return 0
        case "$profile" in
          end4 | end4-pc)
            {
              printf '%s\n' "-- Active shell keybind profile: $profile"
              printf '%s\n' '-- end4 registers keybinds from its own Hyprland Lua modules.'
            } | cmp -s - "$path" && return 0
            ;;
        esac
      done
      ;;
  esac
  return 1
}

legacy_runtime_symlink_owned() {
  local path="$1"
  local target candidate

  target="$(absolute_symlink_target "$path" 2>/dev/null || true)"
  [ -n "$target" ] || return 1
  while IFS= read -r candidate; do
    [ "$target" = "$candidate" ] || continue
    [ "$candidate" != "$path" ] || continue
    legacy_runtime_payload_matches "$candidate" && return 0
  done < <(legacy_hyprland_runtime_paths)
  return 1
}

legacy_runtime_path_owned() {
  local path="$1"

  if [ -L "$path" ]; then
    legacy_runtime_symlink_owned "$path"
    return
  fi
  legacy_runtime_payload_matches "$path"
}

prune_legacy_hyprland_runtime_files() {
  local path recovery parent snapshot_ref snapshot_dir index actual_path expected_type expected_identity
  local -a expected=()

  while IFS= read -r path; do
    snapshot_ref="$(transaction_pinned_snapshot_path "$path" 2>/dev/null || true)"
    if [ -z "$snapshot_ref" ]; then
      if [ "$(runtime_path_kind "$path")" = absent ]; then
        continue
      fi
      log "legacy runtime path lacks a pinned transaction snapshot: $path"
      return 1
    fi
    IFS=$'\t' read -r snapshot_dir index actual_path parent <<<"$snapshot_ref"
    mapfile -t expected < <(snapshot_expected_state "$snapshot_dir" "$index")
    expected_type="${expected[0]:-}"
    expected_identity="${expected[1]:-}"
    if [ "$expected_type" = regular ] || [ "$expected_type" = symlink ]; then
      if [ "$(runtime_state_identity "$actual_path" 2>/dev/null || true)" != "$expected_identity" ]; then
        log "legacy runtime changed after snapshot; preserving concurrent winner: $path"
        return 1
      fi
      if ! legacy_runtime_path_owned "$actual_path"; then
        log "refusing unowned legacy runtime collision: $path"
        return 1
      fi
      recovery="$(quarantine_exact_runtime_path "$actual_path" "$expected_identity" 2>/dev/null || true)"
      [ -n "$recovery" ] || return 1
      record_exact_snapshot_mutation "$path" absent "" "$parent" "$recovery" || return 1
    elif [ "$expected_type" != absent ]; then
      log "refusing to remove non-regular legacy runtime path: $path"
      return 1
    fi
  done < <(legacy_hyprland_runtime_paths)
}

runtime_bundle_fast_path_ready() {
  local dir path stable_content launcher_content hypr_content

  dir="$(hypr_dir)"
  stable_content="$(wahrwelt_print_stable_runtime_entrypoint "$hypr_runtime_dir/hyprland.lua")" || return 1
  launcher_content="$(runtime_shell_profile_content)" || return 1
  hypr_content="$(wahrwelt_print_canonical_runtime_entrypoint)" || return 1

  stable_runtime_entrypoint_matches "$dir/hyprland.lua" "$stable_content" || return 1
  runtime_path_matches_content "$(runtime_file shell-profile.lua)" "$launcher_content" || return 1
  runtime_path_matches_content "$(runtime_file hyprland.lua)" "$hypr_content" || return 1
  while IFS= read -r path; do
    if [ -e "$path" ] || [ -L "$path" ]; then
      return 1
    fi
  done < <(legacy_hyprland_runtime_paths)
}

runtime_profile_mutation_paths() {
  local content path

  path="$(runtime_file shell-launcher.lua)"
  content="$(runtime_shell_launcher_content)" || return 1
  runtime_path_matches_content "$path" "$content" || printf '%s\n' "$path"

  path="$(runtime_file shell-keybinds.lua)"
  content="$(runtime_shell_keybinds_content)" || return 1
  runtime_path_matches_content "$path" "$content" || printf '%s\n' "$path"

  path="$(runtime_file hyprlock.conf)"
  content="$(runtime_hyprlock_content)" || return 1
  runtime_path_matches_content "$path" "$content" || printf '%s\n' "$path"

  path="$(runtime_file hypridle.conf)"
  content="$(runtime_hypridle_content)" || return 1
  runtime_path_matches_content "$path" "$content" || printf '%s\n' "$path"
}

runtime_full_bundle_paths() {
  local dir

  dir="$(hypr_dir)"
  printf '%s\n' \
    "$dir/hyprland.lua" \
    "$(runtime_file shell-profile.lua)" \
    "$(runtime_file shell-launcher.lua)" \
    "$(runtime_file shell-keybinds.lua)" \
    "$(runtime_file hyprland.lua)" \
    "$(runtime_file hyprlock.conf)" \
    "$(runtime_file hypridle.conf)"
  legacy_hyprland_runtime_paths
}

runtime_bundle_paths() {
  if runtime_bundle_fast_path_ready; then
    runtime_profile_mutation_paths
    return $?
  fi
  runtime_full_bundle_paths
}

runtime_union_path_if_needed() {
  local path="$1"
  local requested_content="$2"
  local fallback_content="$3"

  if [ "$requested_content" != "$fallback_content" ] ||
    ! runtime_path_matches_content "$path" "$requested_content"; then
    printf '%s\n' "$path"
  fi
  return 0
}

runtime_profile_union_mutation_paths() {
  local requested_profile="$1"
  local fallback_profile="$2"
  local path requested_content fallback_content
  local profile="$requested_profile"

  path="$(runtime_file shell-launcher.lua)"
  requested_content="$(runtime_shell_launcher_content)" || return 1
  profile="$fallback_profile"
  fallback_content="$(runtime_shell_launcher_content)" || return 1
  runtime_union_path_if_needed "$path" "$requested_content" "$fallback_content"

  profile="$requested_profile"
  path="$(runtime_file shell-keybinds.lua)"
  requested_content="$(runtime_shell_keybinds_content)" || return 1
  profile="$fallback_profile"
  fallback_content="$(runtime_shell_keybinds_content)" || return 1
  runtime_union_path_if_needed "$path" "$requested_content" "$fallback_content"

  profile="$requested_profile"
  path="$(runtime_file hyprlock.conf)"
  requested_content="$(runtime_hyprlock_content)" || return 1
  profile="$fallback_profile"
  fallback_content="$(runtime_hyprlock_content)" || return 1
  runtime_union_path_if_needed "$path" "$requested_content" "$fallback_content"

  profile="$requested_profile"
  path="$(runtime_file hypridle.conf)"
  requested_content="$(runtime_hypridle_content)" || return 1
  profile="$fallback_profile"
  fallback_content="$(runtime_hypridle_content)" || return 1
  runtime_union_path_if_needed "$path" "$requested_content" "$fallback_content"
}

runtime_switch_bundle_paths() {
  local requested_profile="$profile"

  if ! runtime_bundle_fast_path_ready; then
    runtime_full_bundle_paths
    return $?
  fi
  if wahrwelt_valid_shell_profile "${previous:-}" && [ "$previous" != "$requested_profile" ]; then
    runtime_profile_union_mutation_paths "$requested_profile" "$previous"
    return $?
  fi
  runtime_profile_mutation_paths
}

state_bundle_paths() {
  if ! runtime_path_matches_content "$persistent_state_file" "$profile"; then
    printf '%s\n' "$persistent_state_file"
  fi
  if wahrwelt_valid_end4_variant "$profile" &&
    ! runtime_path_matches_content "$wahrwelt_end4_variant_state" "$profile"; then
    printf '%s\n' "$wahrwelt_end4_variant_state"
  fi
}

sync_runtime_shell_files() {
  prune_legacy_hyprland_runtime_files || return 1
  sync_stable_lua_entrypoint || return 1
  sync_shell_launcher || return 1
  sync_shell_launcher_bindings || return 1
  sync_shell_keybinds || return 1
  sync_hypr_entrypoint || return 1
  sync_hypr_lock_stack
}

require_file() {
  local label="$1"
  local path="$2"

  if [ -f "$path" ]; then
    return 0
  fi

  log "$label missing for profile=$profile path=$path"
  return 1
}

require_command() {
  local command_name="$1"

  if command -v "$command_name" >/dev/null 2>&1; then
    return 0
  fi

  log "$command_name command not found for profile=$profile"
  return 1
}

validate_profile_ready() {
  local dir adapter launcher_module

  dir="$(hypr_dir)"
  ensure_wahrwelt_entrypoint || return 1
  require_file "shell lifecycle launcher" "$dir/scripts/start-shell.sh" || return 1
  launcher_module="$(wahrwelt_shell_launcher_module "$profile")" || return 1
  require_file "shell launcher profile" "$dir/${launcher_module%%.*}/launcher.lua" || return 1
  adapter="$(wahrwelt_shell_adapter "$profile")" || return 1

  case "$profile" in
    caelestia)
      require_file "shell keybind profile" "$dir/caelestia/keybinds.lua" || return 1
      command -v caelestia >/dev/null 2>&1 || require_command caelestia-shell || return 1
      ;;
    noctalia)
      require_file "shell keybind profile" "$dir/noctalia/keybinds.lua" || return 1
      if ! wahrwelt_noctalia_command >/dev/null; then
        log "noctalia command not found for profile=$profile"
        return 1
      fi
      ;;
    end4 | end4-pc)
      validate_end4_profile_tree || return 1
      require_file "end4 shell adapter" "$dir/$adapter.lua" || return 1
      require_file "end4 hypr lua entrypoint" "$dir/end4/hyprland.lua" || return 1
      require_file "end4 hyprlock entrypoint" "$dir/end4/hyprlock.conf" || return 1
      require_file "end4 hypridle entrypoint" "$dir/end4/hypridle.conf" || return 1
      require_file "end4 quickshell config" "$(wahrwelt_end4_quickshell_path "$profile")/shell.qml" || return 1
      require_command qs-end4 || return 1
      ;;
  esac
}

prepare_profile_or_fallback() {
  validate_profile_ready && sync_runtime_shell_files
}

persist_profile() {
  local planned_paths snapshot_dir status=0 owns_snapshot=1
  local state_paths=("$persistent_state_file" "$wahrwelt_end4_variant_state")

  if [ "${switch_transaction_active:-0}" -eq 1 ]; then
    wahrwelt_verify_exact_path_guards "${state_paths[@]}" || return 1
    if [ -z "${state_snapshot_dir:-}" ]; then
      state_path_list=()
      planned_paths="$(state_bundle_paths)" || return 1
      if [ -n "$planned_paths" ]; then
        mapfile -t state_path_list <<<"$planned_paths"
        if ! wahrwelt_begin_exact_snapshot "$wahrwelt_runtime_session_dir" .state-switch-rollback- state; then
          state_path_list=()
          return 1
        fi
        state_snapshot_dir="$wahrwelt_new_snapshot_dir"
        if ! snapshot_exact_paths "$state_snapshot_dir" "${state_path_list[@]}"; then
          remove_exact_path_snapshot "$state_snapshot_dir" "${state_path_list[@]}" ||
            log "state switch snapshot initialization failed; exact recovery retained"
          state_snapshot_dir=""
          state_path_list=()
          return 1
        fi
        if ! wahrwelt_snapshot_matches_exact_path_guards "$state_snapshot_dir" "${state_path_list[@]}"; then
          remove_exact_path_snapshot "$state_snapshot_dir" "${state_path_list[@]}" ||
            log "state switch ownership guard failed; exact recovery retained"
          state_snapshot_dir=""
          state_path_list=()
          return 1
        fi
      fi
    fi
    wahrwelt_verify_exact_path_guards "${state_paths[@]}" || return 1
    snapshot_dir="${state_snapshot_dir:-}"
    owns_snapshot=0
  else
    wahrwelt_begin_exact_snapshot "$wahrwelt_runtime_session_dir" .state-rollback- persist || return 1
    snapshot_dir="$wahrwelt_new_snapshot_dir"
    if ! snapshot_exact_paths "$snapshot_dir" "${state_paths[@]}"; then
      remove_exact_path_snapshot "$snapshot_dir" "${state_paths[@]}" ||
        log "state persistence snapshot initialization failed; exact recovery retained"
      return 1
    fi
  fi

  if ! preflight_regular_file_target "$persistent_state_file" ||
    ! preflight_regular_file_target "$wahrwelt_end4_variant_state"; then
    status=1
  fi

  if [ "$status" -eq 0 ] && wahrwelt_valid_end4_variant "$profile"; then
    write_regular_file_if_changed "$wahrwelt_end4_variant_state" "$profile" || status=1
  fi
  if [ "$status" -eq 0 ]; then
    write_regular_file_if_changed "$persistent_state_file" "$profile" || status=1
  fi

  if [ "$status" -ne 0 ]; then
    if [ "$owns_snapshot" -eq 1 ] && ! restore_exact_paths "$snapshot_dir" "${state_paths[@]}"; then
      log "failed to restore shell state transaction after persistence error; preserving private snapshot: $snapshot_dir"
      return 1
    fi
  fi
  if [ "$owns_snapshot" -eq 1 ]; then
    remove_exact_path_snapshot "$snapshot_dir" "${state_paths[@]}" ||
      log "state persistence snapshot cleanup refused; exact recovery retained"
  fi
  return "$status"
}

hypr_supports_lua_runtime() {
  local version major minor rest

  version="$(hyprctl version 2>/dev/null | awk 'NR == 1 { print $2 }')"
  version="${version#v}"
  major="${version%%.*}"
  rest="${version#*.}"
  minor="${rest%%.*}"

  case "$major" in
    "" | *[!0-9]*) return 1 ;;
  esac
  case "$minor" in
    "" | *[!0-9]*) minor=0 ;;
  esac

  [ "$major" -gt 0 ] || [ "$minor" -ge 55 ]
}

reload_hypr() {
  if command -v hyprctl >/dev/null 2>&1 && hyprctl monitors >/dev/null 2>&1; then
    if ! hypr_supports_lua_runtime; then
      log "skipping hyprctl reload; running Hyprland is older than 0.55 and cannot load Lua runtime"
      return 0
    fi
    if ! hyprctl reload >/dev/null 2>&1; then
      log "hyprctl reload failed after profile sync"
      return 1
    fi
  fi
}
