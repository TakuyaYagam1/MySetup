#!/usr/bin/env bash
# shellcheck disable=SC2034

wahrwelt_runtime_directory_token() {
  local path="$1"

  command -v python3 >/dev/null 2>&1 || return 1
  python3 -I -S - "$path" <<'PY'
import os
import stat
import sys

path = os.fsencode(sys.argv[1])
if not os.path.isabs(path):
    raise SystemExit("XDG_RUNTIME_DIR must be absolute")
if path.startswith(b"//") or os.path.normpath(path) != path:
    raise SystemExit("XDG_RUNTIME_DIR must use one canonical lexical path")
flags = os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC
try:
    fd = os.open(path, flags)
except OSError as error:
    raise SystemExit(f"cannot open XDG_RUNTIME_DIR without following links: {error}")
try:
    info = os.fstat(fd)
    visible = os.lstat(path)
    if not stat.S_ISDIR(info.st_mode):
        raise SystemExit("XDG_RUNTIME_DIR is not an ordinary directory")
    if (visible.st_dev, visible.st_ino) != (info.st_dev, info.st_ino):
        raise SystemExit("XDG_RUNTIME_DIR changed while validating")
    if info.st_uid != os.getuid():
        raise SystemExit("XDG_RUNTIME_DIR is not owned by the current user")
    if stat.S_IMODE(info.st_mode) != 0o700:
        raise SystemExit("XDG_RUNTIME_DIR mode is not 0700")
    print(f"{info.st_dev}:{info.st_ino}:{info.st_uid}:700")
finally:
    os.close(fd)
PY
}

wahrwelt_open_validated_runtime_directory() {
  local path="$1"
  local token pinned opened visible

  token="$(wahrwelt_runtime_directory_token "$path")" || return 1
  if declare -F wahrwelt_after_runtime_directory_token_hook >/dev/null 2>&1; then
    wahrwelt_after_runtime_directory_token_hook "$path" "$token" || return 1
  fi
  exec {wahrwelt_runtime_session_fd}<"$path" || return 1
  pinned="/proc/${BASHPID:-$$}/fd/$wahrwelt_runtime_session_fd"
  opened="$(stat -Lc '%d:%i:%u:%a' -- "$pinned" 2>/dev/null || true)"
  visible="$(stat -c '%d:%i:%u:%a' -- "$path" 2>/dev/null || true)"
  if [ "$opened" != "$token" ] || [ "$visible" != "$token" ] || [ -L "$path" ]; then
    exec {wahrwelt_runtime_session_fd}<&-
    wahrwelt_runtime_session_fd=""
    return 1
  fi
  wahrwelt_runtime_session_pinned_dir="$pinned"
  if declare -F wahrwelt_after_runtime_directory_pin_hook >/dev/null 2>&1; then
    wahrwelt_after_runtime_directory_pin_hook "$path" "$pinned" "$token" || {
      exec {wahrwelt_runtime_session_fd}<&-
      wahrwelt_runtime_session_fd=""
      wahrwelt_runtime_session_pinned_dir=""
      return 1
    }
  fi
}

wahrwelt_managed_regular_fd=""
wahrwelt_managed_regular_path=""
wahrwelt_managed_regular_identity=""
wahrwelt_managed_regular_marker_identity=""

# Create or open one managed regular leaf beneath a retained directory FD.
# Python performs the nofollow open and returns an inode token before Bash
# reopens the file without truncation. Only the retained /proc FD spelling is
# handed to callers, so later public-name replacement cannot redirect I/O.
wahrwelt_open_managed_regular_file() {
  local parent_fd="$1"
  local parent_pinned="$2"
  local name="$3"
  local kind="$4"
  local record identity created marker_identity extra opened_path opened visible

  wahrwelt_managed_regular_fd=""
  wahrwelt_managed_regular_path=""
  wahrwelt_managed_regular_identity=""
  wahrwelt_managed_regular_marker_identity=""
  command -v python3 >/dev/null 2>&1 || return 1
  record="$(
    python3 -I -S - "$parent_fd" "$name" "$kind" <<'PY'
import ctypes
import errno
import os
import re
import stat
import sys
import fcntl
import time

parent_fd = os.dup(int(sys.argv[1]))
name = sys.argv[2]
kind = sys.argv[3]
if not name or "/" in name or name in (".", ".."):
    raise SystemExit("invalid managed regular filename")
if not re.fullmatch(r"[A-Za-z0-9._-]+", kind):
    raise SystemExit("invalid managed regular kind")
marker = ".wahrwelt-owner." + name
flags = os.O_RDWR | os.O_CLOEXEC | os.O_NOFOLLOW | os.O_NONBLOCK
created = False

transaction_fd = os.open(
    ".", os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC, dir_fd=parent_fd
)
for attempt in range(200):
    try:
        fcntl.flock(transaction_fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        break
    except BlockingIOError:
        if attempt == 199:
            raise OSError("managed regular ownership transaction stayed busy")
        time.sleep(0.005)

def present(entry):
    try:
        os.stat(entry, dir_fd=parent_fd, follow_symlinks=False)
        return True
    except FileNotFoundError:
        return False

def token(info):
    return f"{info.st_dev}:{info.st_ino}:{info.st_uid}:{stat.S_IMODE(info.st_mode):o}:{info.st_nlink}"

libc = ctypes.CDLL(None, use_errno=True)
linkat = libc.linkat
linkat.argtypes = [
    ctypes.c_int,
    ctypes.c_char_p,
    ctypes.c_int,
    ctypes.c_char_p,
    ctypes.c_int,
]
linkat.restype = ctypes.c_int

try:
    for attempt in range(40):
        file_present = present(name)
        marker_present = present(marker)
        if file_present and marker_present:
            fd = os.open(name, flags, dir_fd=parent_fd)
            break
        if not file_present and not marker_present:
            try:
                fd = os.open(name, flags | os.O_CREAT | os.O_EXCL, 0o600, dir_fd=parent_fd)
                created = True
                break
            except FileExistsError:
                pass
        if attempt == 39:
            raise OSError("managed regular ownership marker collision")
        time.sleep(0.005)
    try:
        info = os.fstat(fd)
        visible = os.stat(name, dir_fd=parent_fd, follow_symlinks=False)
        mode = stat.S_IMODE(info.st_mode)
        if created:
            os.fchmod(fd, 0o600)
            info = os.fstat(fd)
            mode = stat.S_IMODE(info.st_mode)
        if (
            not stat.S_ISREG(info.st_mode)
            or not stat.S_ISREG(visible.st_mode)
            or info.st_uid != os.getuid()
            or info.st_nlink != 1
            or mode & 0o022
            or mode & 0o600 != 0o600
            or (visible.st_dev, visible.st_ino) != (info.st_dev, info.st_ino)
        ):
            raise OSError("managed regular file is not private and owner-controlled")
        file_token = token(info)
        marker_value = f"Wahrwelt managed regular v1\nkind={kind}\ninode={info.st_dev}:{info.st_ino}\n".encode()
        marker_flags = os.O_CLOEXEC | os.O_NOFOLLOW | os.O_NONBLOCK
        if created:
            marker_fd = os.open(
                ".",
                os.O_WRONLY | os.O_TMPFILE | os.O_CLOEXEC,
                0o600,
                dir_fd=parent_fd,
            )
            offset = 0
            while offset < len(marker_value):
                offset += os.write(marker_fd, marker_value[offset:])
            os.fchmod(marker_fd, 0o600)
            os.fsync(marker_fd)
            if os.environ.get("WAHRWELT_TEST_MANAGED_MARKER_FAIL_BEFORE_LINK_KIND") == kind:
                raise OSError("injected managed marker publication failure")
            if linkat(marker_fd, b"", parent_fd, os.fsencode(marker), 0x1000) == 0:
                marker_created = True
                os.fsync(parent_fd)
            else:
                error = ctypes.get_errno()
                os.close(marker_fd)
                marker_fd = None
                if error != errno.EEXIST:
                    raise OSError(error, os.strerror(error))
                marker_fd = os.open(marker, marker_flags | os.O_RDONLY, dir_fd=parent_fd)
                marker_created = False
        else:
            marker_fd = os.open(marker, marker_flags | os.O_RDONLY, dir_fd=parent_fd)
            marker_created = False
        try:
            marker_info = os.fstat(marker_fd)
            marker_visible = os.stat(marker, dir_fd=parent_fd, follow_symlinks=False)
            if not marker_created:
                value = os.read(marker_fd, 4097)
                if value != marker_value:
                    raise OSError("managed regular ownership marker content mismatch")
            if (
                not stat.S_ISREG(marker_info.st_mode)
                or not stat.S_ISREG(marker_visible.st_mode)
                or marker_info.st_uid != os.getuid()
                or marker_info.st_nlink != 1
                or stat.S_IMODE(marker_info.st_mode) != 0o600
                or (marker_visible.st_dev, marker_visible.st_ino)
                != (marker_info.st_dev, marker_info.st_ino)
            ):
                raise OSError("managed regular ownership marker is unsafe")
            print(f"{file_token}\t{1 if created else 0}\t{token(marker_info)}")
        finally:
            os.close(marker_fd)
    finally:
        os.close(fd)
finally:
    os.close(parent_fd)
PY
  )" || return 1
  IFS=$'\t' read -r identity created marker_identity extra <<<"$record"
  [ -n "$identity" ] && { [ "$created" = 0 ] || [ "$created" = 1 ]; } &&
    [ -n "$marker_identity" ] && [ -z "${extra:-}" ] || return 1
  if declare -F wahrwelt_after_managed_regular_token_hook >/dev/null 2>&1; then
    wahrwelt_after_managed_regular_token_hook "$kind" "$parent_pinned/$name" "$identity" "$created" || return 1
  fi
  # Opening read-write has no truncation side effect. A raced symlink or other
  # replacement is rejected before this descriptor is exposed to any writer.
  exec {wahrwelt_managed_regular_fd}<>"$parent_pinned/$name" || return 1
  opened_path="/proc/${BASHPID:-$$}/fd/$wahrwelt_managed_regular_fd"
  opened="$(stat -Lc '%d:%i:%u:%a:%h' -- "$opened_path" 2>/dev/null || true)"
  visible="$(stat -c '%d:%i:%u:%a:%h' -- "$parent_pinned/$name" 2>/dev/null || true)"
  if [ "$opened" != "$identity" ] || [ "$visible" != "$identity" ] || [ -L "$parent_pinned/$name" ] ||
    ! python3 -I -S - "$parent_fd" "$name" "$kind" "$identity" "$marker_identity" <<'PY'; then
import os
import stat
import sys

parent_fd = int(sys.argv[1])
name = sys.argv[2]
kind = sys.argv[3]
expected_file = sys.argv[4]
expected_marker = sys.argv[5]
marker = ".wahrwelt-owner." + name

def token(info):
    return f"{info.st_dev}:{info.st_ino}:{info.st_uid}:{stat.S_IMODE(info.st_mode):o}:{info.st_nlink}"

file_fd = os.open(name, os.O_RDONLY | os.O_CLOEXEC | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=parent_fd)
marker_fd = None
try:
    info = os.fstat(file_fd)
    visible = os.stat(name, dir_fd=parent_fd, follow_symlinks=False)
    if token(info) != expected_file or token(visible) != expected_file or not stat.S_ISREG(info.st_mode):
        raise OSError("managed regular changed before final proof")
    marker_fd = os.open(
        marker,
        os.O_RDONLY | os.O_CLOEXEC | os.O_NOFOLLOW | os.O_NONBLOCK,
        dir_fd=parent_fd,
    )
    marker_info = os.fstat(marker_fd)
    marker_visible = os.stat(marker, dir_fd=parent_fd, follow_symlinks=False)
    value = os.read(marker_fd, 4097)
    expected_value = f"Wahrwelt managed regular v1\nkind={kind}\ninode={info.st_dev}:{info.st_ino}\n".encode()
    if (
        token(marker_info) != expected_marker
        or token(marker_visible) != expected_marker
        or not stat.S_ISREG(marker_info.st_mode)
        or value != expected_value
    ):
        raise OSError("managed regular ownership proof changed")
finally:
    if marker_fd is not None:
        os.close(marker_fd)
    os.close(file_fd)
PY
    exec {wahrwelt_managed_regular_fd}<&-
    wahrwelt_managed_regular_fd=""
    return 1
  fi
  wahrwelt_managed_regular_path="$opened_path"
  wahrwelt_managed_regular_identity="$identity"
  wahrwelt_managed_regular_marker_identity="$marker_identity"
}

# Adopt the one unmarked regular file created by pre-marker releases. The
# allowlist is deliberately exact: this is not a generic ownership claim.
wahrwelt_adopt_legacy_managed_regular_file() {
  local parent_fd="$1"
  local parent_pinned="$2"
  local name="$3"
  local kind="$4"
  local record state identity extra

  [ "$name" = wahrwelt-shell.log ] && [ "$kind" = shell-log ] || return 1
  record="$(
    python3 -I -S - "$parent_fd" "$name" "$kind" preflight <<'PY'
import os
import stat
import sys

parent_fd = os.dup(int(sys.argv[1]))
name = sys.argv[2]
kind = sys.argv[3]
action = sys.argv[4]
marker = ".wahrwelt-owner." + name

if (name, kind) != ("wahrwelt-shell.log", "shell-log") or action != "preflight":
    raise SystemExit("invalid legacy managed regular adoption request")

def present(entry):
    try:
        os.stat(entry, dir_fd=parent_fd, follow_symlinks=False)
        return True
    except FileNotFoundError:
        return False

def token(info):
    return f"{info.st_dev}:{info.st_ino}:{info.st_uid}:{stat.S_IMODE(info.st_mode):o}:{info.st_nlink}"

try:
    file_present = present(name)
    marker_present = present(marker)
    if not file_present and not marker_present:
        print("absent")
    elif file_present and marker_present:
        print("managed")
    elif file_present:
        fd = os.open(name, os.O_RDWR | os.O_CLOEXEC | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=parent_fd)
        try:
            info = os.fstat(fd)
            visible = os.stat(name, dir_fd=parent_fd, follow_symlinks=False)
            mode = stat.S_IMODE(info.st_mode)
            if (
                not stat.S_ISREG(info.st_mode)
                or not stat.S_ISREG(visible.st_mode)
                or (visible.st_dev, visible.st_ino) != (info.st_dev, info.st_ino)
                or info.st_uid != os.getuid()
                or info.st_nlink != 1
                or mode & 0o7022
                or mode & 0o600 != 0o600
            ):
                raise OSError("legacy managed regular file is not an exact safe shape")
            print("legacy\t" + token(info))
        finally:
            os.close(fd)
    else:
        raise OSError("legacy managed regular marker exists without its file")
finally:
    os.close(parent_fd)
PY
  )" || return 1
  IFS=$'\t' read -r state identity extra <<<"$record"
  [ -z "${extra:-}" ] || return 1
  case "$state" in
    absent)
      if declare -F wahrwelt_after_legacy_managed_regular_preflight_hook >/dev/null 2>&1; then
        wahrwelt_after_legacy_managed_regular_preflight_hook \
          "$kind" "$parent_pinned/$name" "" absent || return 1
      fi
      return 0
      ;;
    managed) return 0 ;;
    legacy) [ -n "$identity" ] || return 1 ;;
    *) return 1 ;;
  esac

  if declare -F wahrwelt_after_legacy_managed_regular_preflight_hook >/dev/null 2>&1; then
    wahrwelt_after_legacy_managed_regular_preflight_hook \
      "$kind" "$parent_pinned/$name" "$identity" || return 1
  fi
  python3 -I -S - "$parent_fd" "$name" "$kind" commit "$identity" <<'PY'
import ctypes
import errno
import os
import stat
import sys
import fcntl
import time

parent_fd = os.dup(int(sys.argv[1]))
name = sys.argv[2]
kind = sys.argv[3]
action = sys.argv[4]
expected = sys.argv[5]
marker = ".wahrwelt-owner." + name
marker_fd = None
marker_identity = None
marker_created = False

if (name, kind) != ("wahrwelt-shell.log", "shell-log") or action != "commit":
    raise SystemExit("invalid legacy managed regular adoption request")

transaction_fd = os.open(
    ".", os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC, dir_fd=parent_fd
)
for attempt in range(200):
    try:
        fcntl.flock(transaction_fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        break
    except BlockingIOError:
        if attempt == 199:
            raise OSError("legacy managed regular adoption stayed busy")
        time.sleep(0.005)

def token(info):
    return f"{info.st_dev}:{info.st_ino}:{info.st_uid}:{stat.S_IMODE(info.st_mode):o}:{info.st_nlink}"

libc = ctypes.CDLL(None, use_errno=True)
linkat = libc.linkat
linkat.argtypes = [
    ctypes.c_int,
    ctypes.c_char_p,
    ctypes.c_int,
    ctypes.c_char_p,
    ctypes.c_int,
]
linkat.restype = ctypes.c_int

try:
    fd = os.open(name, os.O_RDWR | os.O_CLOEXEC | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=parent_fd)
    try:
        info = os.fstat(fd)
        visible = os.stat(name, dir_fd=parent_fd, follow_symlinks=False)
        mode = stat.S_IMODE(info.st_mode)
        if (
            token(info) != expected
            or token(visible) != expected
            or not stat.S_ISREG(info.st_mode)
            or info.st_uid != os.getuid()
            or info.st_nlink != 1
            or mode & 0o7022
            or mode & 0o600 != 0o600
        ):
            raise OSError("legacy managed regular file changed before adoption")
        marker_value = f"Wahrwelt managed regular v1\nkind={kind}\ninode={info.st_dev}:{info.st_ino}\n".encode()
        try:
            marker_fd = os.open(
                marker,
                os.O_RDONLY | os.O_CLOEXEC | os.O_NOFOLLOW | os.O_NONBLOCK,
                dir_fd=parent_fd,
            )
        except FileNotFoundError:
            marker_fd = os.open(
                ".",
                os.O_WRONLY | os.O_TMPFILE | os.O_CLOEXEC,
                0o600,
                dir_fd=parent_fd,
            )
            offset = 0
            while offset < len(marker_value):
                offset += os.write(marker_fd, marker_value[offset:])
            os.fchmod(marker_fd, 0o600)
            os.fsync(marker_fd)
            if linkat(marker_fd, b"", parent_fd, os.fsencode(marker), 0x1000) != 0:
                error = ctypes.get_errno()
                os.close(marker_fd)
                marker_fd = None
                if error != errno.EEXIST:
                    raise OSError(error, os.strerror(error))
                marker_fd = os.open(
                    marker,
                    os.O_RDONLY | os.O_CLOEXEC | os.O_NOFOLLOW | os.O_NONBLOCK,
                    dir_fd=parent_fd,
                )
            else:
                marker_created = True
                os.fsync(parent_fd)
        marker_info = os.fstat(marker_fd)
        marker_identity = (marker_info.st_dev, marker_info.st_ino)
        if not marker_created and os.read(marker_fd, 4097) != marker_value:
            raise OSError("legacy managed regular ownership winner is not exact")
        current = os.fstat(fd)
        current_visible = os.stat(name, dir_fd=parent_fd, follow_symlinks=False)
        marker_visible = os.stat(marker, dir_fd=parent_fd, follow_symlinks=False)
        if (
            token(current) != expected
            or token(current_visible) != expected
            or not stat.S_ISREG(marker_info.st_mode)
            or marker_info.st_uid != os.getuid()
            or marker_info.st_nlink != 1
            or stat.S_IMODE(marker_info.st_mode) != 0o600
            or (marker_visible.st_dev, marker_visible.st_ino) != marker_identity
        ):
            raise OSError("legacy managed regular changed during adoption")
    finally:
        os.close(fd)
except Exception:
    # Keep an exact marker published by this transaction on failure. Removing
    # it through the public name after a separate identity check would let a
    # same-UID racer substitute an unrelated inode between stat and unlink.
    # A later pass validates the atomically published marker before reuse.
    raise
finally:
    if marker_fd is not None:
        os.close(marker_fd)
    os.close(parent_fd)
PY
}

wahrwelt_config_home="${XDG_CONFIG_HOME:-$HOME/.config}"
wahrwelt_runtime_session_public_dir="${XDG_RUNTIME_DIR:-}"
if [ -z "$wahrwelt_runtime_session_public_dir" ]; then
  printf 'Wahrwelt runtime ownership collision: XDG_RUNTIME_DIR is required; TMPDIR fallback is disabled\n' >&2
  exit 1
fi
wahrwelt_runtime_session_fd=""
wahrwelt_runtime_session_pinned_dir=""
if ! wahrwelt_open_validated_runtime_directory "$wahrwelt_runtime_session_public_dir"; then
  printf 'Wahrwelt runtime ownership collision: unsafe XDG_RUNTIME_DIR preserved: %s\n' \
    "$wahrwelt_runtime_session_public_dir" >&2
  exit 1
fi
wahrwelt_runtime_session_dir="$wahrwelt_runtime_session_pinned_dir"
wahrwelt_state_home="${XDG_STATE_HOME:-$HOME/.local/state}"
wahrwelt_hypr_dir="$wahrwelt_config_home/hypr"
wahrwelt_state_dir="$wahrwelt_state_home/wahrwelt"
wahrwelt_hypr_runtime_dir="$wahrwelt_state_dir/hypr-runtime"
wahrwelt_active_shell_state="$wahrwelt_state_dir/active-shell"
wahrwelt_end4_variant_state="$wahrwelt_state_dir/end4-variant"
wahrwelt_log_fd=""
wahrwelt_log_file=""
if ! wahrwelt_adopt_legacy_managed_regular_file \
  "$wahrwelt_runtime_session_fd" "$wahrwelt_runtime_session_dir" wahrwelt-shell.log shell-log; then
  printf 'Wahrwelt runtime ownership collision: unsafe pre-marker log preserved without mutation: %s/wahrwelt-shell.log\n' \
    "$wahrwelt_runtime_session_public_dir" >&2
  exit 1
fi
if ! wahrwelt_open_managed_regular_file \
  "$wahrwelt_runtime_session_fd" "$wahrwelt_runtime_session_dir" wahrwelt-shell.log shell-log; then
  printf 'Wahrwelt runtime ownership collision: managed log preserved without mutation: %s/wahrwelt-shell.log\n' \
    "$wahrwelt_runtime_session_public_dir" >&2
  exit 1
fi
if ! exec {wahrwelt_log_fd}<&"$wahrwelt_managed_regular_fd"; then
  printf 'Wahrwelt runtime ownership collision: managed log descriptor could not be retained\n' >&2
  exit 1
fi
wahrwelt_log_file="/proc/${BASHPID:-$$}/fd/$wahrwelt_log_fd"
exec {wahrwelt_managed_regular_fd}<&-
wahrwelt_managed_regular_fd=""
wahrwelt_default_shell_profile="caelestia"

wahrwelt_selector_pattern='((^|[ /])(qs|quickshell)([[:space:]].*)?-c[[:space:]]wahrwelt-shell-selector([[:space:]]|$))|quickshell/wahrwelt-shell-selector([/[:space:]]|$)'
wahrwelt_noctalia_v4_pattern='(^|[ /])noctalia-shell([[:space:]]|$)|share/noctalia-shell'
wahrwelt_caelestia_pattern='share/caelestia-shell|caelestia-shell|(^|[ /])caelestia[[:space:]]+shell([[:space:]]|$)'
wahrwelt_noctalia_v4_env_pattern='^QS_CONFIG_PATH=.*/share/noctalia-shell$'
wahrwelt_end4_official_env_pattern='^WAHRWELT_END4_PROFILE=end4$'
wahrwelt_end4_pc_env_pattern='^WAHRWELT_END4_PROFILE=end4-pc$'
wahrwelt_end4_env_pattern='^WAHRWELT_END4_PROFILE=(end4|end4-pc)$'

wahrwelt_user_name="${USER:-}"
if [ -z "$wahrwelt_user_name" ]; then
  wahrwelt_user_name="$(id -un 2>/dev/null || printf '%s' user)"
fi

wahrwelt_hypr_dir_path() {
  printf '%s' "$wahrwelt_hypr_dir"
}

wahrwelt_runtime_file() {
  printf '%s/%s' "$wahrwelt_hypr_runtime_dir" "$1"
}

wahrwelt_valid_shell_profile() {
  case "$1" in
    caelestia | noctalia | end4 | end4-pc) return 0 ;;
    *) return 1 ;;
  esac
}

wahrwelt_valid_end4_variant() {
  case "$1" in
    end4 | end4-pc) return 0 ;;
    *) return 1 ;;
  esac
}

wahrwelt_shell_family() {
  case "$1" in
    end4 | end4-pc) printf '%s' end4 ;;
    *) printf '%s' "$1" ;;
  esac
}

wahrwelt_shell_adapter() {
  case "$1" in
    noctalia) printf '%s' noctalia.keybinds ;;
    caelestia) printf '%s' caelestia.keybinds ;;
    end4 | end4-pc) printf '%s' end4-adapter ;;
    *) return 1 ;;
  esac
}

wahrwelt_shell_launcher_module() {
  case "$1" in
    noctalia) printf '%s' noctalia.launcher ;;
    caelestia) printf '%s' caelestia.launcher ;;
    end4 | end4-pc) printf '%s' end4.launcher ;;
    *) return 1 ;;
  esac
}

wahrwelt_end4_quickshell_config() {
  case "$1" in
    end4) printf '%s' ii ;;
    end4-pc) printf '%s' end4-pC ;;
    *) return 1 ;;
  esac
}

wahrwelt_end4_quickshell_path() {
  local qs_config

  qs_config="$(wahrwelt_end4_quickshell_config "$1")" || return 1
  printf '%s/quickshell/%s' "$wahrwelt_config_home" "$qs_config"
}

wahrwelt_read_end4_variant() {
  local path="${1:-$wahrwelt_end4_variant_state}"
  local stored=""

  if [ -f "$path" ]; then
    IFS= read -r stored <"$path" || stored=""
    if ! printf '%s\n' "$stored" | cmp -s - "$path"; then
      stored=""
    fi
  fi

  if wahrwelt_valid_end4_variant "$stored"; then
    printf '%s' "$stored"
  else
    printf '%s' end4
  fi
}

wahrwelt_noctalia_command() {
  if command -v noctalia >/dev/null 2>&1; then
    printf '%s' noctalia
    return 0
  fi

  if command -v noctalia-shell >/dev/null 2>&1; then
    printf '%s' noctalia-shell
    return 0
  fi

  return 1
}

wahrwelt_noctalia_daemon_flag() {
  local command_name

  command_name="$(wahrwelt_noctalia_command 2>/dev/null || true)"
  case "${command_name##*/}" in
    noctalia-shell)
      printf '%s' --daemonize
      ;;
    *)
      printf '%s' --daemon
      ;;
  esac
}

wahrwelt_noctalia_msg() {
  local command_name

  command_name="$(wahrwelt_noctalia_command 2>/dev/null || true)"
  [ -n "$command_name" ] || return 1
  "$command_name" msg "$@"
}

wahrwelt_noctalia_is_v4() {
  local command_name

  command_name="$(wahrwelt_noctalia_command 2>/dev/null || true)"
  [ "${command_name##*/}" = "noctalia-shell" ]
}

# v4 (noctalia-shell) uses `msg <target> <function>`; v5 (noctalia) uses a
# flat hyphenated `msg <command>` vocabulary. Translate stable semantic
# action names to whichever syntax the running binary understands.
wahrwelt_noctalia_action() {
  local action="$1"

  case "$action" in
    launcher-toggle)
      if wahrwelt_noctalia_is_v4; then
        wahrwelt_noctalia_msg launcher toggle
      else
        wahrwelt_noctalia_msg panel-toggle launcher
      fi
      ;;
    session-menu-toggle)
      if wahrwelt_noctalia_is_v4; then
        wahrwelt_noctalia_msg sessionMenu toggle
      else
        wahrwelt_noctalia_msg panel-toggle session
      fi
      ;;
    control-center-toggle)
      if wahrwelt_noctalia_is_v4; then
        wahrwelt_noctalia_msg controlCenter toggle
      else
        wahrwelt_noctalia_msg panel-toggle control-center
      fi
      ;;
    notifications-clear)
      if wahrwelt_noctalia_is_v4; then
        wahrwelt_noctalia_msg notifications dismissAll
      else
        wahrwelt_noctalia_msg notification-clear-active
      fi
      ;;
    settings-toggle)
      if wahrwelt_noctalia_is_v4; then
        wahrwelt_noctalia_msg settings toggle
      else
        wahrwelt_noctalia_msg settings-toggle
      fi
      ;;
    lock)
      if wahrwelt_noctalia_is_v4; then
        wahrwelt_noctalia_msg lockScreen lock
      else
        wahrwelt_noctalia_msg session lock
      fi
      ;;
    brightness-up)
      if wahrwelt_noctalia_is_v4; then
        wahrwelt_noctalia_msg brightness increase
      else
        wahrwelt_noctalia_msg brightness-up
      fi
      ;;
    brightness-down)
      if wahrwelt_noctalia_is_v4; then
        wahrwelt_noctalia_msg brightness decrease
      else
        wahrwelt_noctalia_msg brightness-down
      fi
      ;;
    clipboard-toggle)
      if wahrwelt_noctalia_is_v4; then
        wahrwelt_noctalia_msg launcher clipboard
      else
        wahrwelt_noctalia_msg panel-toggle clipboard
      fi
      ;;
    emoji-toggle)
      if wahrwelt_noctalia_is_v4; then
        wahrwelt_noctalia_msg launcher emoji
      else
        wahrwelt_noctalia_msg panel-toggle launcher /emo
      fi
      ;;
    *)
      shift
      wahrwelt_noctalia_msg "$action" "$@"
      ;;
  esac
}

wahrwelt_pid_matches() {
  local pid="$1"
  local pattern="$2"

  [ -n "$pid" ] || return 1
  ps -p "$pid" -o args= 2>/dev/null | grep -qE "$pattern"
}

wahrwelt_pid_has_env_regex() {
  local pid="$1"
  local regex="$2"
  local env_file

  [ -n "$pid" ] || return 1
  env_file="/proc/$pid/environ"
  [ -r "$env_file" ] || return 1
  { tr '\0' '\n' <"$env_file"; } 2>/dev/null | grep -qE "$regex"
}

wahrwelt_quickshell_pids() {
  pgrep -u "$wahrwelt_user_name" -f '(^|[ /])(qs-end4|qs|\.?quickshell(-wrapped)?)([[:space:]]|$)' 2>/dev/null || true
}

wahrwelt_noctalia_pids() {
  local pid

  {
    pgrep -u "$wahrwelt_user_name" -x noctalia 2>/dev/null || true
    pgrep -u "$wahrwelt_user_name" -f '(^|/)noctalia([[:space:]]|$)' 2>/dev/null || true
    pgrep -u "$wahrwelt_user_name" -f "$wahrwelt_noctalia_v4_pattern" 2>/dev/null || true
    for pid in $(wahrwelt_quickshell_pids); do
      if wahrwelt_pid_has_env_regex "$pid" "$wahrwelt_noctalia_v4_env_pattern"; then
        printf '%s\n' "$pid"
      fi
    done
  } | sort -u
}

wahrwelt_noctalia_running() {
  wahrwelt_noctalia_pids | grep -q .
}

wahrwelt_end4_profile_pids() {
  local profile="$1"
  local pattern pid

  case "$profile" in
    end4) pattern="$wahrwelt_end4_official_env_pattern" ;;
    end4-pc) pattern="$wahrwelt_end4_pc_env_pattern" ;;
    *) return 1 ;;
  esac

  for pid in $(wahrwelt_quickshell_pids); do
    if wahrwelt_pid_has_env_regex "$pid" "$pattern"; then
      printf '%s\n' "$pid"
    fi
  done
}

wahrwelt_end4_profile_running() {
  wahrwelt_end4_profile_pids "$1" | grep -q .
}

# Validate one PID/start-time/config token emitted by the exact Home Manager
# direct-End4 migration. Normal runtime detection never calls this path.
wahrwelt_pid_is_legacy_end4_upgrade_token() {
  local token="$1"

  python3 -I -S - "$token" <<'PY'
import os
import re
import sys

match = re.fullmatch(r"([1-9][0-9]*):([1-9][0-9]*):(ii|end4-pC)", sys.argv[1])
if match is None:
    raise SystemExit(1)
pid_text, expected_start, config = match.groups()
pid = int(pid_text)
proc = f"/proc/{pid}"
try:
    info = os.stat(proc, follow_symlinks=False)
    if info.st_uid != os.getuid():
        raise SystemExit(1)
    with open(f"{proc}/stat", "rb", buffering=0) as stream:
        stat_value = stream.read(65536)
    fields = stat_value.rsplit(b") ", 1)[1].split()
    if len(fields) <= 19 or fields[19].decode("ascii") != expected_start:
        raise SystemExit(1)
    with open(f"{proc}/cmdline", "rb", buffering=0) as stream:
        argv = [item for item in stream.read(65536).split(b"\0") if item]
    with open(f"{proc}/environ", "rb", buffering=0) as stream:
        environment = set(item for item in stream.read(1048576).split(b"\0") if item)
except (FileNotFoundError, PermissionError, ProcessLookupError, IndexError, ValueError):
    raise SystemExit(1)

executables = {b"qs-end4", b"qs", b"quickshell", b"quickshell-wrapped", b".quickshell-wrapped"}
if (
    len(argv) != 5
    or os.path.basename(argv[0]) not in executables
    or argv[1:] != [b"-n", b"-d", b"-c", os.fsencode(config)]
    or b"WAHRWELT_END4_PROFILE=end4" in environment
    or b"WAHRWELT_END4_PROFILE=end4-pc" in environment
):
    raise SystemExit(1)
PY
}

wahrwelt_legacy_end4_upgrade_pids() {
  local tokens="${1:-}"
  local old_ifs="$IFS"
  local token

  [ -n "$tokens" ] || return 0
  IFS=','
  for token in $tokens; do
    IFS="$old_ifs"
    if wahrwelt_pid_is_legacy_end4_upgrade_token "$token"; then
      printf '%s\n' "${token%%:*}"
    fi
    IFS=','
  done
  IFS="$old_ifs"
}

wahrwelt_cmdline_has_adjacent_args() {
  local cmdline_file="$1"
  local first="$2"
  local second="$3"
  local previous=""
  local argument

  [ -r "$cmdline_file" ] || return 1
  while IFS= read -r -d '' argument; do
    if [ "$previous" = "$first" ] && [ "$argument" = "$second" ]; then
      return 0
    fi
    previous="$argument"
  done <"$cmdline_file"
  return 1
}

wahrwelt_pid_has_adjacent_args() {
  local pid="$1"

  [ -n "$pid" ] || return 1
  wahrwelt_cmdline_has_adjacent_args "/proc/$pid/cmdline" "$2" "$3"
}

wahrwelt_detect_shell_adapter() {
  local path="$1"
  local marker=""

  [ -r "$path" ] || return 1
  IFS= read -r marker <"$path" || true
  case "$marker" in
    '-- Wahrwelt shell adapter: noctalia') printf '%s' noctalia ;;
    '-- Wahrwelt shell adapter: caelestia') printf '%s' caelestia ;;
    '-- Wahrwelt shell adapter: end4') printf '%s' end4 ;;
    '-- Wahrwelt shell adapter: end4-pc') printf '%s' end4-pc ;;
    *) return 1 ;;
  esac
}

wahrwelt_print_legacy_end4_entrypoint() {
  local profile="$1"

  wahrwelt_valid_end4_variant "$profile" || return 1
  cat <<EOF
-- Active Hyprland profile: $profile
local home = os.getenv("HOME")
if home == nil then
    error("HOME is not set; cannot locate end4 Hyprland config")
end

local config_home = os.getenv("XDG_CONFIG_HOME") or (home .. "/.config")
local hypr_root = config_home .. "/hypr"
local end4_root = hypr_root .. "/end4"
package.path = end4_root .. "/?.lua;" .. end4_root .. "/?/init.lua;" .. hypr_root .. "/?.lua;" .. hypr_root .. "/?/init.lua;" .. package.path
dofile(end4_root .. "/hyprland.lua")
EOF
}

wahrwelt_print_canonical_runtime_entrypoint() {
  cat <<'EOF'
-- Wahrwelt canonical Hyprland runtime entrypoint
local home = os.getenv("HOME")
if home == nil then
    error("HOME is not set; cannot locate Wahrwelt Hyprland config")
end

local config_home = os.getenv("XDG_CONFIG_HOME") or (home .. "/.config")
local hypr_root = config_home .. "/hypr"
package.path = hypr_root .. "/?.lua;" .. hypr_root .. "/?/init.lua;" .. package.path
dofile(hypr_root .. "/user/hyprland.lua")
EOF
}

wahrwelt_print_legacy_user_entrypoint() {
  cat <<'EOF'
-- Wahrwelt canonical Hyprland runtime entrypoint
local home = os.getenv("HOME")
if home == nil then
    error("HOME is not set; cannot locate Wahrwelt Hyprland config")
end

local config_home = os.getenv("XDG_CONFIG_HOME") or (home .. "/.config")
local hypr_root = config_home .. "/hypr"
package.path = hypr_root .. "/?.lua;" .. hypr_root .. "/?/init.lua;" .. package.path
dofile(hypr_root .. "/wahrwelt/hyprland.lua")
EOF
}

wahrwelt_print_legacy_home_manager_user_entrypoint() {
  cat <<EOF
-- Active Hyprland profile: wahrwelt ($wahrwelt_default_shell_profile)
local home = os.getenv("HOME")
if home == nil then
    error("HOME is not set; cannot locate Wahrwelt Hyprland config")
end

local config_home = os.getenv("XDG_CONFIG_HOME") or (home .. "/.config")
local hypr_root = config_home .. "/hypr"
package.path = hypr_root .. "/?.lua;" .. hypr_root .. "/?/init.lua;" .. package.path
dofile(hypr_root .. "/wahrwelt/hyprland.lua")
EOF
}

wahrwelt_print_home_manager_initial_entrypoint() {
  cat <<EOF
-- Active Hyprland profile: wahrwelt ($wahrwelt_default_shell_profile)
local home = os.getenv("HOME")
if home == nil then
    error("HOME is not set; cannot locate Wahrwelt Hyprland config")
end

local config_home = os.getenv("XDG_CONFIG_HOME") or (home .. "/.config")
local hypr_root = config_home .. "/hypr"
package.path = hypr_root .. "/?.lua;" .. hypr_root .. "/?/init.lua;" .. package.path
dofile(hypr_root .. "/user/hyprland.lua")
EOF
}

wahrwelt_print_stable_runtime_entrypoint() {
  local runtime_entrypoint="${1:-$wahrwelt_hypr_runtime_dir/hyprland.lua}"

  cat <<EOF
-- Generated by Wahrwelt: stable Hyprland Lua runtime entrypoint
local home = os.getenv("HOME")
if home == nil then
    error("HOME is not set; cannot locate Wahrwelt Hyprland runtime")
end

local config_home = os.getenv("XDG_CONFIG_HOME") or (home .. "/.config")
local hypr_root = config_home .. "/hypr"
package.path = hypr_root .. "/?.lua;" .. hypr_root .. "/?/init.lua;" .. package.path
dofile("$runtime_entrypoint")
EOF
}

wahrwelt_is_canonical_entrypoint() {
  local path="$1"

  [ -r "$path" ] || return 1
  wahrwelt_print_canonical_runtime_entrypoint | cmp -s - "$path" ||
    wahrwelt_print_home_manager_initial_entrypoint | cmp -s - "$path" ||
    wahrwelt_print_stable_runtime_entrypoint | cmp -s - "$path"
}

wahrwelt_is_legacy_user_entrypoint() {
  local path="$1"

  [ -r "$path" ] || return 1
  wahrwelt_print_legacy_user_entrypoint | cmp -s - "$path" ||
    wahrwelt_print_legacy_home_manager_user_entrypoint | cmp -s - "$path"
}

wahrwelt_is_legacy_direct_end4_entrypoint() {
  local path="$1"
  local config_home="$2"

  [ -r "$path" ] || return 1
  : "$config_home"
  wahrwelt_print_legacy_end4_entrypoint end4 | cmp -s - "$path" ||
    wahrwelt_print_legacy_end4_entrypoint end4-pc | cmp -s - "$path"
}

wahrwelt_read_active_shell() {
  local path="${1:-$wahrwelt_active_shell_state}"
  local stored=""

  if [ -f "$path" ]; then
    stored="$(tr -d '[:space:]' <"$path" 2>/dev/null || true)"
  fi

  if wahrwelt_valid_shell_profile "$stored"; then
    printf '%s' "$stored"
    return 0
  fi

  return 1
}

wahrwelt_lock_owner_running() {
  local owner_pid="$1"
  local owner_file="$2"
  local owner_name="$3"
  local owner_pattern="$4"
  local active_owner lock_dir

  lock_dir="$(dirname -- "$owner_file")"
  active_owner="$(wahrwelt_read_known_lock_field "$lock_dir" "${owner_file##*/}" 2>/dev/null || true)"
  [ "$active_owner" = "$owner_name" ] || return 1
  wahrwelt_pid_matches "$owner_pid" "$owner_pattern"
}

wahrwelt_path_identity() {
  local path="$1"

  [ -e "$path" ] || [ -L "$path" ] || return 1
  stat -Lc '%d:%i:%f:%s:%Y:%Z' -- "$path" 2>/dev/null
}

wahrwelt_lock_identity() {
  local path="$1"

  # /proc/<pid>/fd/<fd> is the deliberate descriptor-backed spelling used for
  # a directory we already pinned.  Every ordinary namespace path must still
  # reject a final symlink without following it.
  if [[ "$path" =~ ^/proc/[0-9]+/fd/[0-9]+$ ]]; then
    [ -d "$path" ] || return 1
  else
    [ -d "$path" ] && [ ! -L "$path" ] || return 1
  fi
  stat -Lc '%d:%i' -- "$path" 2>/dev/null
}

wahrwelt_opened_directory_identity() {
  local path="$1"

  [ -d "$path" ] || return 1
  stat -Lc '%d:%i' -- "$path" 2>/dev/null
}

wahrwelt_created_directory_name=""
wahrwelt_created_directory_identity=""
wahrwelt_created_directory_fd=""
wahrwelt_created_directory_path=""
wahrwelt_lock_recovery_fd="${wahrwelt_lock_recovery_fd:-}"
wahrwelt_lock_recovery_fd_path="${wahrwelt_lock_recovery_fd_path:-}"
wahrwelt_lock_recovery_exact_path="${wahrwelt_lock_recovery_exact_path:-}"
wahrwelt_lock_recovery_identity="${wahrwelt_lock_recovery_identity:-}"
wahrwelt_lock_recovery_public_path="${wahrwelt_lock_recovery_public_path:-}"
wahrwelt_lock_recovery_public_identity="${wahrwelt_lock_recovery_public_identity:-}"
wahrwelt_lock_collision_path="${wahrwelt_lock_collision_path:-}"
wahrwelt_lock_collision_identity="${wahrwelt_lock_collision_identity:-}"

wahrwelt_close_lock_recovery() {
  if [ -n "$wahrwelt_lock_recovery_fd" ]; then
    exec {wahrwelt_lock_recovery_fd}<&-
  fi
  wahrwelt_lock_recovery_fd=""
  wahrwelt_lock_recovery_fd_path=""
  wahrwelt_lock_recovery_exact_path=""
  wahrwelt_lock_recovery_identity=""
  wahrwelt_lock_recovery_public_path=""
  wahrwelt_lock_recovery_public_identity=""
  wahrwelt_lock_collision_path=""
  wahrwelt_lock_collision_identity=""
}

wahrwelt_nofollow_node_identity() {
  local path="$1"

  [ -e "$path" ] || [ -L "$path" ] || return 1
  stat -c '%d:%i:%f:%s:%Y:%Z' -- "$path" 2>/dev/null
}

# Convert a live recovery FD into a durable, user-actionable path. readlink is
# only a hint until the resolved ordinary directory is lstat-checked against
# the FD identity. The public recovery and canonical collision names are
# recorded separately because either may already name an unknown winner.
wahrwelt_refresh_lock_recovery_report() {
  local expected_identity="$1"
  local resolved resolved_identity

  wahrwelt_lock_recovery_exact_path=""
  wahrwelt_lock_recovery_identity=""
  wahrwelt_lock_recovery_public_identity="$(
    wahrwelt_nofollow_node_identity "$wahrwelt_lock_recovery_public_path" 2>/dev/null || true
  )"
  wahrwelt_lock_collision_identity="$(
    wahrwelt_nofollow_node_identity "$wahrwelt_lock_collision_path" 2>/dev/null || true
  )"
  [ -n "$wahrwelt_lock_recovery_fd_path" ] || return 1
  [ "$(wahrwelt_opened_directory_identity "$wahrwelt_lock_recovery_fd_path" 2>/dev/null || true)" = "$expected_identity" ] ||
    return 1
  wahrwelt_lock_recovery_identity="$expected_identity"
  resolved="$(readlink -- "$wahrwelt_lock_recovery_fd_path" 2>/dev/null || true)"
  case "$resolved" in "" | *' (deleted)') return 1 ;; esac
  resolved_identity="$(wahrwelt_lock_identity "$resolved" 2>/dev/null || true)"
  [ "$resolved_identity" = "$expected_identity" ] || return 1
  wahrwelt_lock_recovery_exact_path="$resolved"
}

# Create and pin a private directory beneath an already pinned parent. The
# creator process owns mkdir/open/fstat as one handoff and returns the exact
# inode token before the shell reopens it. A replacement is never chmodded or
# removed by this helper.
wahrwelt_create_pinned_private_directory() {
  local parent_fd="$1"
  local naming="$2"
  local value="$3"
  local kind="$4"
  local parent_pinned record name identity extra opened_path

  wahrwelt_created_directory_name=""
  wahrwelt_created_directory_identity=""
  wahrwelt_created_directory_fd=""
  wahrwelt_created_directory_path=""
  command -v python3 >/dev/null 2>&1 || return 1
  parent_pinned="/proc/${BASHPID:-$$}/fd/$parent_fd"
  record="$(
    python3 -I -S - "$parent_fd" "$naming" "$value" <<'PY'
import os
import secrets
import stat
import sys

parent_fd = os.dup(int(sys.argv[1]))
naming = sys.argv[2]
value = sys.argv[3]
try:
    parent_info = os.fstat(parent_fd)
    if not stat.S_ISDIR(parent_info.st_mode):
        raise OSError("private-directory parent is not a directory")
    if naming not in ("exact", "prefix"):
        raise OSError("invalid private-directory naming mode")
    if not value or "/" in value or value in (".", ".."):
        raise OSError("invalid private-directory name")
    names = (value,) if naming == "exact" else (
        value + secrets.token_hex(12) for _ in range(128)
    )
    for name in names:
        try:
            os.mkdir(name, 0o700, dir_fd=parent_fd)
        except FileExistsError:
            if naming == "exact":
                raise
            continue
        fd = os.open(
            name,
            os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC,
            dir_fd=parent_fd,
        )
        try:
            info = os.fstat(fd)
            visible = os.stat(name, dir_fd=parent_fd, follow_symlinks=False)
            if (
                not stat.S_ISDIR(info.st_mode)
                or not stat.S_ISDIR(visible.st_mode)
                or (visible.st_dev, visible.st_ino) != (info.st_dev, info.st_ino)
            ):
                raise OSError("private directory changed before creator handoff")
            os.fchmod(fd, 0o700)
            info = os.fstat(fd)
            print(f"{name}\t{info.st_dev}:{info.st_ino}")
            break
        finally:
            os.close(fd)
    else:
        raise OSError("private-directory creator exhausted names")
finally:
    os.close(parent_fd)
PY
  )" || return 1
  IFS=$'\t' read -r name identity extra <<<"$record"
  [ -n "$name" ] && [ -n "$identity" ] && [ -z "${extra:-}" ] || return 1
  case "$name" in
    "" | . | .. | */*) return 1 ;;
  esac
  if declare -F wahrwelt_after_private_directory_creator_hook >/dev/null 2>&1; then
    wahrwelt_after_private_directory_creator_hook "$kind" "$parent_pinned/$name" "$identity" || return 1
  fi
  exec {wahrwelt_created_directory_fd}<"$parent_pinned/$name" || return 1
  opened_path="/proc/${BASHPID:-$$}/fd/$wahrwelt_created_directory_fd"
  if [ "$(wahrwelt_opened_directory_identity "$opened_path" 2>/dev/null || true)" != "$identity" ] ||
    [ "$(wahrwelt_lock_identity "$parent_pinned/$name" 2>/dev/null || true)" != "$identity" ] ||
    [ "$(stat -Lc '%a:%u' -- "$opened_path" 2>/dev/null || true)" != "700:${UID}" ]; then
    exec {wahrwelt_created_directory_fd}<&-
    wahrwelt_created_directory_fd=""
    return 1
  fi
  wahrwelt_created_directory_name="$name"
  wahrwelt_created_directory_identity="$identity"
  wahrwelt_created_directory_path="$opened_path"
}

wahrwelt_private_state_directory_name=""
wahrwelt_private_state_directory_identity=""
wahrwelt_private_state_directory_fd=""
wahrwelt_private_state_directory_path=""
wahrwelt_private_state_directory_marker_identity=""

# Create or validate an application state directory directly below the pinned
# XDG runtime root. Existing nodes are never chmodded. The returned descriptor
# path remains authoritative if the public child name is replaced later.
wahrwelt_open_private_state_directory() {
  local name="$1"
  local kind="$2"
  local parent_fd="$wahrwelt_runtime_session_fd"
  local parent_pinned="$wahrwelt_runtime_session_dir"
  local record identity created marker_identity extra opened_path opened visible

  wahrwelt_private_state_directory_name=""
  wahrwelt_private_state_directory_identity=""
  wahrwelt_private_state_directory_fd=""
  wahrwelt_private_state_directory_path=""
  wahrwelt_private_state_directory_marker_identity=""
  command -v python3 >/dev/null 2>&1 || return 1
  record="$(
    python3 -I -S - "$parent_fd" "$name" "$kind" <<'PY'
import ctypes
import errno
import fcntl
import os
import re
import secrets
import stat
import sys
import time

parent_fd = os.dup(int(sys.argv[1]))
name = sys.argv[2]
kind = sys.argv[3]
if not name or "/" in name or name in (".", ".."):
    raise SystemExit("invalid private state directory name")
if not re.fullmatch(r"[A-Za-z0-9._-]+", kind):
    raise SystemExit("invalid private state directory kind")
marker = ".wahrwelt-state-owner"
created = False

transaction_fd = os.open(
    ".", os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC, dir_fd=parent_fd
)
for attempt in range(200):
    try:
        fcntl.flock(transaction_fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        break
    except BlockingIOError:
        if attempt == 199:
            raise OSError("private state ownership transaction stayed busy")
        time.sleep(0.005)

def token(info):
    return f"{info.st_dev}:{info.st_ino}:{info.st_uid}:{stat.S_IMODE(info.st_mode):o}:{info.st_nlink}"

libc = ctypes.CDLL(None, use_errno=True)
renameat2 = libc.renameat2
renameat2.argtypes = [
    ctypes.c_int,
    ctypes.c_char_p,
    ctypes.c_int,
    ctypes.c_char_p,
    ctypes.c_uint,
]
renameat2.restype = ctypes.c_int

def open_existing():
    return os.open(
        name,
        os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC,
        dir_fd=parent_fd,
    )

def validate_directory(fd, require_public):
    info = os.fstat(fd)
    if not stat.S_ISDIR(info.st_mode):
        raise OSError("private state node is not a directory")
    if require_public:
        visible = os.stat(name, dir_fd=parent_fd, follow_symlinks=False)
        if not stat.S_ISDIR(visible.st_mode) or (
            visible.st_dev,
            visible.st_ino,
        ) != (info.st_dev, info.st_ino):
            raise OSError("private state directory changed before validation")
    if info.st_uid != os.getuid() or stat.S_IMODE(info.st_mode) != 0o700:
        raise OSError("private state directory is not owner-controlled mode 0700")
    return info

def create_unpublished_marker(fd, info):
    marker_value = (
        f"Wahrwelt private state v1\nkind={kind}\n"
        f"inode={info.st_dev}:{info.st_ino}\n"
    ).encode()
    marker_fd = os.open(
        marker,
        os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_CLOEXEC | os.O_NOFOLLOW | os.O_NONBLOCK,
        0o600,
        dir_fd=fd,
    )
    try:
        offset = 0
        while offset < len(marker_value):
            offset += os.write(marker_fd, marker_value[offset:])
        os.fchmod(marker_fd, 0o600)
        os.fsync(marker_fd)
    finally:
        os.close(marker_fd)

try:
    try:
        fd = open_existing()
    except FileNotFoundError:
        fd = None
        for _ in range(128):
            pending = f".wahrwelt-state-pending.{name}.{secrets.token_hex(12)}"
            try:
                os.mkdir(pending, 0o700, dir_fd=parent_fd)
            except FileExistsError:
                continue
            candidate_fd = os.open(
                pending,
                os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC,
                dir_fd=parent_fd,
            )
            info = validate_directory(candidate_fd, False)
            create_unpublished_marker(candidate_fd, info)
            os.fsync(candidate_fd)
            if renameat2(
                parent_fd,
                os.fsencode(pending),
                parent_fd,
                os.fsencode(name),
                1,
            ) == 0:
                fd = candidate_fd
                created = True
                os.fsync(parent_fd)
                break
            error = ctypes.get_errno()
            os.close(candidate_fd)
            if error == errno.EEXIST:
                fd = open_existing()
                break
            raise OSError(error, os.strerror(error))
        if fd is None:
            raise OSError("private state directory staging names were exhausted")
    try:
        info = validate_directory(fd, True)
        marker_value = f"Wahrwelt private state v1\nkind={kind}\ninode={info.st_dev}:{info.st_ino}\n".encode()
        marker_flags = os.O_CLOEXEC | os.O_NOFOLLOW | os.O_NONBLOCK
        marker_fd = os.open(marker, marker_flags | os.O_RDONLY, dir_fd=fd)
        try:
            marker_info = os.fstat(marker_fd)
            marker_visible = os.stat(marker, dir_fd=fd, follow_symlinks=False)
            if os.read(marker_fd, 4097) != marker_value:
                raise OSError("private state ownership marker content mismatch")
            if (
                not stat.S_ISREG(marker_info.st_mode)
                or not stat.S_ISREG(marker_visible.st_mode)
                or marker_info.st_uid != os.getuid()
                or marker_info.st_nlink != 1
                or stat.S_IMODE(marker_info.st_mode) != 0o600
                or (marker_visible.st_dev, marker_visible.st_ino)
                != (marker_info.st_dev, marker_info.st_ino)
            ):
                raise OSError("private state ownership marker is unsafe")
            print(f"{info.st_dev}:{info.st_ino}\t{1 if created else 0}\t{token(marker_info)}")
        finally:
            os.close(marker_fd)
    finally:
        os.close(fd)
finally:
    os.close(parent_fd)
PY
  )" || return 1
  IFS=$'\t' read -r identity created marker_identity extra <<<"$record"
  [ -n "$identity" ] && { [ "$created" = 0 ] || [ "$created" = 1 ]; } &&
    [ -n "$marker_identity" ] && [ -z "${extra:-}" ] || return 1
  if declare -F wahrwelt_after_private_state_directory_token_hook >/dev/null 2>&1; then
    wahrwelt_after_private_state_directory_token_hook \
      "$kind" "$parent_pinned/$name" "$identity" "$created" || return 1
  fi
  exec {wahrwelt_private_state_directory_fd}<"$parent_pinned/$name" || return 1
  opened_path="/proc/${BASHPID:-$$}/fd/$wahrwelt_private_state_directory_fd"
  opened="$(stat -Lc '%d:%i:%u:%a' -- "$opened_path" 2>/dev/null || true)"
  visible="$(stat -c '%d:%i:%u:%a' -- "$parent_pinned/$name" 2>/dev/null || true)"
  if [ "$opened" != "$identity:${UID}:700" ] || [ "$visible" != "$identity:${UID}:700" ] ||
    [ -L "$parent_pinned/$name" ] ||
    ! python3 -I -S - "$wahrwelt_private_state_directory_fd" "$kind" "$identity" "$marker_identity" <<'PY'; then
import os
import stat
import sys

directory_fd = int(sys.argv[1])
kind = sys.argv[2]
directory_identity = sys.argv[3]
expected_marker = sys.argv[4]
marker = ".wahrwelt-state-owner"

def token(info):
    return f"{info.st_dev}:{info.st_ino}:{info.st_uid}:{stat.S_IMODE(info.st_mode):o}:{info.st_nlink}"

directory_info = os.fstat(directory_fd)
if f"{directory_info.st_dev}:{directory_info.st_ino}" != directory_identity:
    raise OSError("private state directory identity changed")
fd = os.open(marker, os.O_RDONLY | os.O_CLOEXEC | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=directory_fd)
try:
    info = os.fstat(fd)
    visible = os.stat(marker, dir_fd=directory_fd, follow_symlinks=False)
    expected_value = (
        f"Wahrwelt private state v1\nkind={kind}\n"
        f"inode={directory_info.st_dev}:{directory_info.st_ino}\n"
    ).encode()
    if (
        token(info) != expected_marker
        or token(visible) != expected_marker
        or not stat.S_ISREG(info.st_mode)
        or os.read(fd, 4097) != expected_value
    ):
        raise OSError("private state ownership proof changed")
finally:
    os.close(fd)
PY
    exec {wahrwelt_private_state_directory_fd}<&-
    wahrwelt_private_state_directory_fd=""
    return 1
  fi
  wahrwelt_private_state_directory_name="$name"
  wahrwelt_private_state_directory_identity="$identity"
  wahrwelt_private_state_directory_path="$opened_path"
  wahrwelt_private_state_directory_marker_identity="$marker_identity"
}

wahrwelt_end4_upgrade_state_name="wahrwelt-end4-upgrade"
wahrwelt_end4_upgrade_state_fd=""
wahrwelt_end4_upgrade_state_path=""
wahrwelt_end4_upgrade_state_identity=""
wahrwelt_end4_upgrade_state_marker_identity=""

# Retain exact pre-marker End4 process identities in the session-scoped XDG
# runtime tree. A reboot drops the state together with the processes it names.
wahrwelt_open_end4_upgrade_state() {
  wahrwelt_end4_upgrade_state_fd=""
  wahrwelt_end4_upgrade_state_path=""
  wahrwelt_end4_upgrade_state_identity=""
  wahrwelt_end4_upgrade_state_marker_identity=""

  wahrwelt_open_private_state_directory \
    "$wahrwelt_end4_upgrade_state_name" end4-upgrade-state || return 1
  wahrwelt_end4_upgrade_state_fd="$wahrwelt_private_state_directory_fd"
  wahrwelt_end4_upgrade_state_path="$wahrwelt_private_state_directory_path"
  wahrwelt_end4_upgrade_state_identity="$wahrwelt_private_state_directory_identity"
  wahrwelt_end4_upgrade_state_marker_identity="$wahrwelt_private_state_directory_marker_identity"
}

wahrwelt_end4_upgrade_state_transaction() {
  local operation="$1"
  local requested_tokens="${2:-}"
  local transaction_fd result status preflight token identity extra remove_plan=""

  case "$operation" in
    read | merge | remove) ;;
    *) return 1 ;;
  esac
  [ -n "$wahrwelt_end4_upgrade_state_fd" ] &&
    [ -n "$wahrwelt_end4_upgrade_state_path" ] &&
    [ -n "$wahrwelt_end4_upgrade_state_identity" ] &&
    [ -n "$wahrwelt_end4_upgrade_state_marker_identity" ] || return 1
  command -v flock >/dev/null 2>&1 || return 1
  command -v python3 >/dev/null 2>&1 || return 1

  # This is a new open file description, not a dup of the retained state FD.
  # Closing it below therefore always releases this transaction's flock.
  exec {transaction_fd}<"$wahrwelt_end4_upgrade_state_path/." || return 1
  if ! flock -x -w 2 "$transaction_fd"; then
    exec {transaction_fd}<&-
    return 1
  fi
  if declare -F wahrwelt_after_end4_upgrade_state_lock_hook >/dev/null 2>&1; then
    if ! wahrwelt_after_end4_upgrade_state_lock_hook "$operation"; then
      exec {transaction_fd}<&-
      return 1
    fi
  fi

  if [ "$operation" = remove ] &&
    declare -F wahrwelt_before_end4_upgrade_token_remove_hook >/dev/null 2>&1; then
    if ! preflight="$(
      python3 -I -S - "$wahrwelt_end4_upgrade_state_fd" "$requested_tokens" <<'PY'
import os
import re
import stat
import sys

state_fd = int(sys.argv[1])
requested = sys.argv[2]
pattern = re.compile(r"([1-9][0-9]*):([1-9][0-9]*):(ii|end4-pC)")
values = requested.split(",") if requested else []
if len(values) > 256 or any(pattern.fullmatch(value) is None for value in values):
    raise OSError("invalid End4 upgrade removal preflight")
for value in sorted(set(values)):
    try:
        fd = os.open(
            value,
            os.O_RDONLY | os.O_CLOEXEC | os.O_NOFOLLOW | os.O_NONBLOCK,
            dir_fd=state_fd,
        )
    except FileNotFoundError:
        continue
    try:
        info = os.fstat(fd)
        visible = os.stat(value, dir_fd=state_fd, follow_symlinks=False)
        expected = f"Wahrwelt End4 upgrade process v1\n{value}\n".encode()
        if (
            not stat.S_ISREG(info.st_mode)
            or not stat.S_ISREG(visible.st_mode)
            or info.st_uid != os.getuid()
            or info.st_nlink != 1
            or stat.S_IMODE(info.st_mode) != 0o600
            or (visible.st_dev, visible.st_ino) != (info.st_dev, info.st_ino)
            or os.read(fd, 4097) != expected
        ):
            raise OSError("unsafe End4 upgrade removal preflight leaf")
        print(f"{value}\t{info.st_dev}:{info.st_ino}")
    finally:
        os.close(fd)
PY
    )"; then
      exec {transaction_fd}<&-
      return 1
    fi
    while IFS=$'\t' read -r token identity extra; do
      [ -n "$token" ] || continue
      [ -n "$identity" ] && [ -z "${extra:-}" ] || {
        exec {transaction_fd}<&-
        return 1
      }
      if ! wahrwelt_before_end4_upgrade_token_remove_hook \
        "$token" "$wahrwelt_end4_upgrade_state_path/$token" "$identity"; then
        exec {transaction_fd}<&-
        return 1
      fi
      remove_plan+="${remove_plan:+,}$token=$identity"
    done <<<"$preflight"
  fi

  result="$({
    python3 -I -S - \
      "$wahrwelt_runtime_session_fd" \
      "$wahrwelt_end4_upgrade_state_fd" \
      "$wahrwelt_end4_upgrade_state_name" \
      "$wahrwelt_end4_upgrade_state_identity" \
      "$wahrwelt_end4_upgrade_state_marker_identity" \
      "$operation" \
      "$requested_tokens" \
      "$remove_plan" <<'PY'
import ctypes
import errno
import os
import re
import secrets
import stat
import sys

runtime_fd = int(sys.argv[1])
state_fd = int(sys.argv[2])
state_name = sys.argv[3]
expected_state = sys.argv[4]
expected_marker = sys.argv[5]
operation = sys.argv[6]
requested_value = sys.argv[7]
remove_plan_value = sys.argv[8]
marker_name = ".wahrwelt-state-owner"
token_pattern = re.compile(r"([1-9][0-9]*):([1-9][0-9]*):(ii|end4-pC)")


def inode(info):
    return f"{info.st_dev}:{info.st_ino}"


def file_token(info):
    return (
        f"{info.st_dev}:{info.st_ino}:{info.st_uid}:"
        f"{stat.S_IMODE(info.st_mode):o}:{info.st_nlink}"
    )


def parse_tokens(value):
    if not value:
        return set()
    if len(value) > 65536:
        raise OSError("End4 upgrade process state is too large")
    values = value.split(",")
    if len(values) > 256:
        raise OSError("too many End4 upgrade process tokens")
    if any(token_pattern.fullmatch(value) is None for value in values):
        raise OSError("invalid End4 upgrade process token")
    return set(values)


def sort_key(value):
    match = token_pattern.fullmatch(value)
    if match is None:
        raise OSError("invalid End4 upgrade process token")
    pid, start, variant = match.groups()
    return int(pid), int(start), variant


def token_value(value):
    return f"Wahrwelt End4 upgrade process v1\n{value}\n".encode()


def validate_marker(directory_info):
    fd = os.open(
        marker_name,
        os.O_RDONLY | os.O_CLOEXEC | os.O_NOFOLLOW | os.O_NONBLOCK,
        dir_fd=state_fd,
    )
    try:
        info = os.fstat(fd)
        visible = os.stat(marker_name, dir_fd=state_fd, follow_symlinks=False)
        expected_value = (
            "Wahrwelt private state v1\nkind=end4-upgrade-state\n"
            f"inode={directory_info.st_dev}:{directory_info.st_ino}\n"
        ).encode()
        if (
            not stat.S_ISREG(info.st_mode)
            or file_token(info) != expected_marker
            or file_token(visible) != expected_marker
            or os.read(fd, 4097) != expected_value
        ):
            raise OSError("End4 upgrade state ownership marker changed")
    finally:
        os.close(fd)


def validate_leaf(name, value=None):
    if value is None:
        value = name
    if token_pattern.fullmatch(value) is None:
        raise OSError("End4 upgrade state contains an unknown entry")
    fd = os.open(
        name,
        os.O_RDONLY | os.O_CLOEXEC | os.O_NOFOLLOW | os.O_NONBLOCK,
        dir_fd=state_fd,
    )
    try:
        info = os.fstat(fd)
        visible = os.stat(name, dir_fd=state_fd, follow_symlinks=False)
        if (
            not stat.S_ISREG(info.st_mode)
            or not stat.S_ISREG(visible.st_mode)
            or info.st_uid != os.getuid()
            or info.st_nlink != 1
            or stat.S_IMODE(info.st_mode) != 0o600
            or (visible.st_dev, visible.st_ino) != (info.st_dev, info.st_ino)
            or os.read(fd, 4097) != token_value(value)
        ):
            raise OSError("End4 upgrade process token leaf is unsafe")
        return info.st_dev, info.st_ino
    finally:
        os.close(fd)


def validate_tree():
    directory_info = os.fstat(state_fd)
    visible = os.stat(state_name, dir_fd=runtime_fd, follow_symlinks=False)
    if (
        not stat.S_ISDIR(directory_info.st_mode)
        or not stat.S_ISDIR(visible.st_mode)
        or inode(directory_info) != expected_state
        or inode(visible) != expected_state
        or directory_info.st_uid != os.getuid()
        or stat.S_IMODE(directory_info.st_mode) != 0o700
    ):
        raise OSError("End4 upgrade state directory changed")
    validate_marker(directory_info)
    values = set()
    for entry in os.listdir(state_fd):
        if entry == marker_name:
            continue
        if token_pattern.fullmatch(entry) is not None:
            validate_leaf(entry)
            values.add(entry)
            continue
        if entry.startswith(".consumed."):
            body = entry[len(".consumed."):]
            value, separator, nonce = body.rpartition(".")
            if (
                separator
                and re.fullmatch(r"[0-9a-f]{24}", nonce) is not None
                and token_pattern.fullmatch(value) is not None
            ):
                validate_leaf(entry, value)
                continue
        raise OSError("End4 upgrade state contains an unknown entry")
    return values


libc = ctypes.CDLL(None, use_errno=True)
renameat2 = libc.renameat2
renameat2.argtypes = [
    ctypes.c_int,
    ctypes.c_char_p,
    ctypes.c_int,
    ctypes.c_char_p,
    ctypes.c_uint,
]
renameat2.restype = ctypes.c_int
linkat = libc.linkat
linkat.argtypes = [
    ctypes.c_int,
    ctypes.c_char_p,
    ctypes.c_int,
    ctypes.c_char_p,
    ctypes.c_int,
]
linkat.restype = ctypes.c_int


def rename_noreplace(source, target):
    if renameat2(
        state_fd,
        os.fsencode(source),
        state_fd,
        os.fsencode(target),
        1,
    ) != 0:
        error = ctypes.get_errno()
        raise OSError(error, os.strerror(error))


def move_to_unique_recovery(source, prefix):
    for _ in range(128):
        target = f".{prefix}.{source}.{secrets.token_hex(12)}"
        try:
            rename_noreplace(source, target)
            return target
        except FileExistsError:
            continue
    raise OSError("End4 upgrade recovery naming exhausted")


def restore_recovery(source, target):
    try:
        rename_noreplace(source, target)
    except OSError:
        # Both names are preserved for explicit collision recovery.
        return False
    return True


def visible_identity(name):
    info = os.stat(name, dir_fd=state_fd, follow_symlinks=False)
    return info.st_dev, info.st_ino


def create_leaf(value):
    # Keep incomplete bytes anonymous. Linking the completed inode into its
    # exact final name is one atomic no-replace operation, so an exception has
    # no public leaf to clean up and cannot unlink an uncooperative replacement.
    fd = os.open(
        ".",
        os.O_WRONLY | os.O_TMPFILE | os.O_CLOEXEC,
        0o600,
        dir_fd=state_fd,
    )
    try:
        payload = token_value(value)
        offset = 0
        while offset < len(payload):
            offset += os.write(fd, payload[offset:])
        os.fchmod(fd, 0o600)
        os.fsync(fd)
        if linkat(fd, b"", state_fd, os.fsencode(value), 0x1000) != 0:
            error = ctypes.get_errno()
            if error == errno.EEXIST:
                validate_leaf(value)
                return
            raise OSError(error, os.strerror(error))
    finally:
        os.close(fd)
    validate_leaf(value)


def remove_leaf(value, expected=None):
    current_identity = validate_leaf(value)
    if expected is None:
        expected = current_identity
    recovery = move_to_unique_recovery(value, "consumed")
    try:
        moved = visible_identity(recovery)
        if moved != expected:
            raise OSError("End4 upgrade process token changed before removal")
        validate_leaf(recovery, value)
    except Exception:
        restore_recovery(recovery, value)
        os.fsync(state_fd)
        raise


requested = parse_tokens(requested_value)
expected_removals = {}
if remove_plan_value:
    for record in remove_plan_value.split(","):
        value, separator, identity = record.partition("=")
        if (
            not separator
            or value not in requested
            or token_pattern.fullmatch(value) is None
            or re.fullmatch(r"[0-9]+:[0-9]+", identity) is None
            or value in expected_removals
        ):
            raise OSError("invalid End4 upgrade removal identity plan")
        expected_removals[value] = tuple(int(part) for part in identity.split(":"))
current = validate_tree()
if operation == "read":
    if requested:
        raise OSError("read transaction received process tokens")
elif operation == "merge":
    for token in sorted(requested - current, key=sort_key):
        create_leaf(token)
elif operation == "remove":
    for token in sorted(requested & current, key=sort_key):
        remove_leaf(token, expected_removals.get(token))
else:
    raise OSError("invalid End4 upgrade state transaction")
os.fsync(state_fd)
current = validate_tree()
print(",".join(sorted(current, key=sort_key)))
PY
  } 2>&1)"
  status=$?
  exec {transaction_fd}<&-
  if [ "$status" -ne 0 ]; then
    [ -z "$result" ] || printf '%s\n' "$result" >&2
    return "$status"
  fi
  printf '%s\n' "$result"
}

wahrwelt_read_end4_upgrade_tokens() {
  wahrwelt_end4_upgrade_state_transaction read
}

wahrwelt_merge_end4_upgrade_tokens() {
  wahrwelt_end4_upgrade_state_transaction merge "${1:-}"
}

wahrwelt_remove_end4_upgrade_tokens() {
  wahrwelt_end4_upgrade_state_transaction remove "${1:-}"
}

# Adopt only the exact unmarked runtime-state trees created by releases
# before ownership markers existed. All contents are validated through pinned
# descriptors before any mode or marker is changed.
wahrwelt_adopt_legacy_private_state_directory() {
  local name="$1"
  local kind="$2"
  local public_path="$wahrwelt_runtime_session_dir/$name"
  local identity

  case "$name:$kind" in
    wahrwelt-noctalia-launcher:noctalia-launcher-state | wahrwelt-recording:record-toggle-state | wahrwelt-shell-selector:shell-selector-state) ;;
    *) return 1 ;;
  esac

  if [ ! -e "$public_path" ] && [ ! -L "$public_path" ]; then
    return 0
  fi
  identity="$(wahrwelt_lock_identity "$public_path" 2>/dev/null || true)"
  [ -n "$identity" ] || return 1
  if declare -F wahrwelt_after_legacy_state_preflight_hook >/dev/null 2>&1; then
    wahrwelt_after_legacy_state_preflight_hook "$kind" "$public_path" "$identity" || return 1
  fi

  python3 -I -S - "$wahrwelt_runtime_session_fd" "$name" "$kind" "$identity" <<'PY'
import ctypes
import fcntl
import os
import re
import stat
import sys
import time

parent_fd = os.dup(int(sys.argv[1]))
name = sys.argv[2]
kind = sys.argv[3]
expected_directory = sys.argv[4]
state_marker = ".wahrwelt-state-owner"

profiles = {
    ("wahrwelt-noctalia-launcher", "noctalia-launcher-state"): {
        "leaves": {
            "active": ("noctalia-active", "empty"),
            "interrupted": ("noctalia-interrupted", "empty"),
        },
        "lock_owner": "wahrwelt-noctalia-launcher",
        "lock_script": "noctalia-launcher.sh",
    },
    ("wahrwelt-recording", "record-toggle-state"): {
        "leaves": {
            "gpu-screen-recorder.pid": ("recorder-pid", "pid"),
            "gpu-screen-recorder.path": ("recorder-path", "path"),
            "gpu-screen-recorder.log": ("recorder-log", "log"),
        },
        "lock_owner": "wahrwelt-record-toggle",
        "lock_script": "record-toggle.sh",
    },
    ("wahrwelt-shell-selector", "shell-selector-state"): {
        "leaves": {},
        "lock_owner": "wahrwelt-shell-selector",
        "lock_script": "shell-selector.sh",
    },
}
profile = profiles.get((name, kind))
if profile is None:
    raise SystemExit("invalid legacy private state adoption request")

created_markers = []
directory_fd = None
original_mode = None

transaction_fd = os.open(
    ".", os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC, dir_fd=parent_fd
)
for attempt in range(200):
    try:
        fcntl.flock(transaction_fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        break
    except BlockingIOError:
        if attempt == 199:
            raise OSError("legacy private state adoption stayed busy")
        time.sleep(0.005)

def inode(info):
    return f"{info.st_dev}:{info.st_ino}"

def file_token(info):
    return f"{info.st_dev}:{info.st_ino}:{info.st_uid}:{stat.S_IMODE(info.st_mode):o}:{info.st_nlink}"

def validate_safe_mode(info, directory=False):
    mode = stat.S_IMODE(info.st_mode)
    required = 0o700 if directory else 0o600
    if info.st_uid != os.getuid() or mode & 0o7022 or mode & required != required:
        raise OSError("legacy runtime state has unsafe ownership or mode")

def open_regular(directory, entry, shape, allow_empty=False):
    fd = os.open(
        entry,
        os.O_RDONLY | os.O_CLOEXEC | os.O_NOFOLLOW | os.O_NONBLOCK,
        dir_fd=directory,
    )
    try:
        info = os.fstat(fd)
        visible = os.stat(entry, dir_fd=directory, follow_symlinks=False)
        if (
            not stat.S_ISREG(info.st_mode)
            or not stat.S_ISREG(visible.st_mode)
            or (visible.st_dev, visible.st_ino) != (info.st_dev, info.st_ino)
            or info.st_nlink != 1
        ):
            raise OSError("legacy runtime state leaf is not one ordinary file")
        validate_safe_mode(info)
        if shape == "log":
            return info, None
        value = os.read(fd, 4097)
        if allow_empty and value == b"":
            return info, value
        if shape == "empty" and value != b"":
            raise OSError("legacy noctalia presence marker is not empty")
        if shape == "pid" and re.fullmatch(rb"[1-9][0-9]*\n", value) is None:
            raise OSError("legacy recorder PID has an invalid shape")
        if shape == "path" and (
            len(value) > 4096
            or not value.startswith(b"/")
            or not value.endswith(b"\n")
            or b"\n" in value[:-1]
            or b"\x00" in value
        ):
            raise OSError("legacy recorder path has an invalid shape")
        return info, value
    finally:
        os.close(fd)

def process_is_live_legacy_owner(pid, script):
    try:
        process = os.stat(f"/proc/{pid}", follow_symlinks=False)
        if process.st_uid != os.getuid():
            return False
        with open(f"/proc/{pid}/cmdline", "rb", buffering=0) as stream:
            arguments = [item for item in stream.read(65536).split(b"\0") if item]
    except (FileNotFoundError, PermissionError, ProcessLookupError):
        return False
    script_bytes = os.fsencode(script)
    return any(os.path.basename(argument) == script_bytes for argument in arguments)

def validate_lock(directory):
    try:
        lock_info = os.stat("lock", dir_fd=directory, follow_symlinks=False)
    except FileNotFoundError:
        return None
    if not stat.S_ISDIR(lock_info.st_mode):
        raise OSError("legacy runtime lock is not an ordinary directory")
    lock_fd = os.open("lock", os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC, dir_fd=directory)
    try:
        opened = os.fstat(lock_fd)
        visible = os.stat("lock", dir_fd=directory, follow_symlinks=False)
        if (opened.st_dev, opened.st_ino) != (visible.st_dev, visible.st_ino):
            raise OSError("legacy runtime lock changed during validation")
        validate_safe_mode(opened, directory=True)
        if set(os.listdir(lock_fd)) != {"pid", "owner"}:
            raise OSError("legacy runtime lock contains unknown entries")
        pid_info, pid_value = open_regular(lock_fd, "pid", "pid")
        owner_info, _ = open_regular(lock_fd, "owner", "log")
        owner_fd = os.open("owner", os.O_RDONLY | os.O_CLOEXEC | os.O_NOFOLLOW, dir_fd=lock_fd)
        try:
            owner_value = os.read(owner_fd, 4097)
        finally:
            os.close(owner_fd)
        expected_owner = (profile["lock_owner"] + "\n").encode()
        if owner_value != expected_owner:
            raise OSError("legacy runtime lock has an unknown owner")
        pid = int(pid_value[:-1])
        if process_is_live_legacy_owner(pid, profile["lock_script"]):
            raise OSError("legacy runtime lock still has a live owner")
        return (
            inode(opened),
            file_token(pid_info),
            file_token(owner_info),
            pid_value,
            owner_value,
        )
    finally:
        os.close(lock_fd)

def validate_tree(directory, markers=False, allow_empty_unmarked=False):
    directory_info = os.fstat(directory)
    public = os.stat(name, dir_fd=parent_fd, follow_symlinks=False)
    if (
        not stat.S_ISDIR(directory_info.st_mode)
        or not stat.S_ISDIR(public.st_mode)
        or inode(directory_info) != expected_directory
        or inode(public) != expected_directory
    ):
        raise OSError("legacy runtime state directory changed")
    validate_safe_mode(directory_info, directory=True)
    allowed = set(profile["leaves"]) | {"lock"}
    if markers:
        allowed.add(state_marker)
        allowed.update(".wahrwelt-owner." + leaf for leaf in profile["leaves"])
    entries = set(os.listdir(directory))
    if not entries <= allowed:
        raise OSError("legacy runtime state contains unknown entries")
    leaves = {}
    for leaf, (_, shape) in profile["leaves"].items():
        if leaf in entries:
            marker_present = ".wahrwelt-owner." + leaf in entries
            effective_shape = "log" if markers and marker_present else shape
            info, value = open_regular(
                directory,
                leaf,
                effective_shape,
                allow_empty=markers and not marker_present and allow_empty_unmarked,
            )
            leaves[leaf] = (file_token(info), value)
    lock = validate_lock(directory) if "lock" in entries else None
    return directory_info, leaves, lock

libc = ctypes.CDLL(None, use_errno=True)
linkat = libc.linkat
linkat.argtypes = [
    ctypes.c_int,
    ctypes.c_char_p,
    ctypes.c_int,
    ctypes.c_char_p,
    ctypes.c_int,
]
linkat.restype = ctypes.c_int

def create_marker(directory, marker_name, value):
    fd = os.open(
        ".",
        os.O_WRONLY | os.O_TMPFILE | os.O_CLOEXEC,
        0o600,
        dir_fd=directory,
    )
    try:
        offset = 0
        while offset < len(value):
            offset += os.write(fd, value[offset:])
        os.fchmod(fd, 0o600)
        os.fsync(fd)
        info = os.fstat(fd)
        if linkat(fd, b"", directory, os.fsencode(marker_name), 0x1000) != 0:
            error = ctypes.get_errno()
            raise OSError(error, os.strerror(error))
        created_markers.append((marker_name, (info.st_dev, info.st_ino), value))
    finally:
        os.close(fd)

def validate_marker(directory, marker_name, expected_identity, expected_value):
    fd = os.open(marker_name, os.O_RDONLY | os.O_CLOEXEC | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=directory)
    try:
        info = os.fstat(fd)
        visible = os.stat(marker_name, dir_fd=directory, follow_symlinks=False)
        if (
            not stat.S_ISREG(info.st_mode)
            or info.st_uid != os.getuid()
            or info.st_nlink != 1
            or stat.S_IMODE(info.st_mode) != 0o600
            or (info.st_dev, info.st_ino) != expected_identity
            or (visible.st_dev, visible.st_ino) != expected_identity
            or os.read(fd, 4097) != expected_value
        ):
            raise OSError("legacy runtime ownership marker changed")
    finally:
        os.close(fd)

def validate_existing_marker(directory, marker_name, expected_value):
    fd = os.open(marker_name, os.O_RDONLY | os.O_CLOEXEC | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=directory)
    try:
        info = os.fstat(fd)
        visible = os.stat(marker_name, dir_fd=directory, follow_symlinks=False)
        if (
            not stat.S_ISREG(info.st_mode)
            or info.st_uid != os.getuid()
            or info.st_nlink != 1
            or stat.S_IMODE(info.st_mode) != 0o600
            or (visible.st_dev, visible.st_ino) != (info.st_dev, info.st_ino)
            or os.read(fd, 4097) != expected_value
        ):
            raise OSError("legacy runtime ownership winner is not exact")
    finally:
        os.close(fd)

def validate_resumable_empty_leaf(directory, leaf):
    fd = os.open(
        leaf,
        os.O_RDONLY | os.O_CLOEXEC | os.O_NOFOLLOW | os.O_NONBLOCK,
        dir_fd=directory,
    )
    try:
        info = os.fstat(fd)
        visible = os.stat(leaf, dir_fd=directory, follow_symlinks=False)
        if (
            not stat.S_ISREG(info.st_mode)
            or not stat.S_ISREG(visible.st_mode)
            or info.st_uid != os.getuid()
            or info.st_nlink != 1
            or stat.S_IMODE(info.st_mode) != 0o600
            or (visible.st_dev, visible.st_ino) != (info.st_dev, info.st_ino)
            or os.read(fd, 1) != b""
        ):
            raise OSError("unmarked managed runtime leaf is not an exact empty creation")
    finally:
        os.close(fd)

def validate_completed_tree(directory):
    after, leaves, lock = validate_tree(directory, markers=True)
    if stat.S_IMODE(after.st_mode) != 0o700:
        raise OSError("managed runtime state directory mode is not 0700")
    expected_markers = {
        state_marker: (
            f"Wahrwelt private state v1\nkind={kind}\n"
            f"inode={after.st_dev}:{after.st_ino}\n"
        ).encode(),
    }
    for leaf, (leaf_kind, _) in profile["leaves"].items():
        if leaf not in leaves:
            continue
        leaf_inode = ":".join(leaves[leaf][0].split(":", 2)[:2])
        expected_markers[".wahrwelt-owner." + leaf] = (
            f"Wahrwelt managed regular v1\nkind={leaf_kind}\ninode={leaf_inode}\n"
        ).encode()
    entries = set(os.listdir(directory))
    required_entries = set(profile["leaves"]) & entries
    required_entries |= {"lock"} & entries
    required_entries |= set(expected_markers)
    if entries != required_entries:
        raise OSError("managed runtime state has incomplete or unknown entries")
    for marker_name, marker_value in expected_markers.items():
        validate_existing_marker(directory, marker_name, marker_value)
    return after, leaves, lock

def resume_committed_tree(directory):
    directory_info = os.fstat(directory)
    if stat.S_IMODE(directory_info.st_mode) != 0o700:
        raise OSError("managed runtime state directory mode is not 0700")
    state_value = (
        f"Wahrwelt private state v1\nkind={kind}\n"
        f"inode={directory_info.st_dev}:{directory_info.st_ino}\n"
    ).encode()
    validate_existing_marker(directory, state_marker, state_value)
    entries = set(os.listdir(directory))
    for leaf in profile["leaves"]:
        if leaf in entries and ".wahrwelt-owner." + leaf not in entries:
            validate_resumable_empty_leaf(directory, leaf)
    before, leaves_before, lock_before = validate_tree(
        directory,
        markers=True,
        allow_empty_unmarked=True,
    )
    expected_leaf_markers = {}
    for leaf, (leaf_kind, _) in profile["leaves"].items():
        if leaf not in leaves_before:
            continue
        leaf_inode = ":".join(leaves_before[leaf][0].split(":", 2)[:2])
        expected_leaf_markers[".wahrwelt-owner." + leaf] = (
            f"Wahrwelt managed regular v1\nkind={leaf_kind}\ninode={leaf_inode}\n"
        ).encode()
    entries = set(os.listdir(directory))
    for marker_name, marker_value in expected_leaf_markers.items():
        if marker_name in entries:
            validate_existing_marker(directory, marker_name, marker_value)
        else:
            create_marker(directory, marker_name, marker_value)
    os.fsync(directory)
    after, leaves_after, lock_after = validate_completed_tree(directory)
    before_tokens = {leaf: value[0] for leaf, value in leaves_before.items()}
    after_tokens = {leaf: value[0] for leaf, value in leaves_after.items()}
    if (
        inode(before) != inode(after)
        or before_tokens != after_tokens
        or lock_before != lock_after
    ):
        raise OSError("managed runtime state changed during marker recovery")
    return after, leaves_after, lock_after

try:
    directory_fd = os.open(name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC, dir_fd=parent_fd)
    if state_marker in set(os.listdir(directory_fd)):
        resume_committed_tree(directory_fd)
    else:
        before, leaves_before, lock_before = validate_tree(directory_fd, markers=True)
        expected_leaf_markers = {}
        for leaf, (leaf_kind, _) in profile["leaves"].items():
            if leaf not in leaves_before:
                continue
            leaf_inode = ":".join(leaves_before[leaf][0].split(":", 2)[:2])
            expected_leaf_markers[".wahrwelt-owner." + leaf] = (
                f"Wahrwelt managed regular v1\nkind={leaf_kind}\ninode={leaf_inode}\n"
            ).encode()
        present_leaf_markers = {
            entry
            for entry in os.listdir(directory_fd)
            if entry.startswith(".wahrwelt-owner.")
        }
        if not present_leaf_markers <= set(expected_leaf_markers):
            raise OSError("legacy runtime state has a marker for an absent leaf")
        for marker_name in present_leaf_markers:
            validate_existing_marker(
                directory_fd,
                marker_name,
                expected_leaf_markers[marker_name],
            )
        original_mode = stat.S_IMODE(before.st_mode)
        os.fchmod(directory_fd, 0o700)
        for marker_name, marker_value in expected_leaf_markers.items():
            if marker_name in present_leaf_markers:
                continue
            create_marker(
                directory_fd,
                marker_name,
                marker_value,
            )
        # Leaf markers are durable before the state marker commits the tree.
        # A crash before this point leaves an exact resumable marker subset.
        os.fsync(directory_fd)
        create_marker(
            directory_fd,
            state_marker,
            f"Wahrwelt private state v1\nkind={kind}\ninode={before.st_dev}:{before.st_ino}\n".encode(),
        )
        after, leaves_after, lock_after = validate_completed_tree(directory_fd)
        before_tokens = {leaf: value[0] for leaf, value in leaves_before.items()}
        after_tokens = {leaf: value[0] for leaf, value in leaves_after.items()}
        if before_tokens != after_tokens or lock_after != lock_before:
            raise OSError("legacy runtime state changed during adoption")
        for marker_name, marker_identity, marker_value in created_markers:
            validate_marker(directory_fd, marker_name, marker_identity, marker_value)
        os.fsync(directory_fd)
except Exception:
    if directory_fd is not None:
        # Keep exact markers already published by this transaction. A
        # stat-then-unlink cleanup through their public names would be subject
        # to same-UID replacement races. Exact partial leaf-marker sets are
        # resumable because the state marker is published last as the commit.
        managed_winner = False
        try:
            validate_completed_tree(directory_fd)
            managed_winner = True
        except OSError:
            pass
        if not managed_winner:
            try:
                if inode(os.fstat(directory_fd)) == expected_directory and original_mode is not None:
                    os.fchmod(directory_fd, original_mode)
            except OSError:
                pass
    raise
finally:
    if directory_fd is not None:
        os.close(directory_fd)
    os.close(parent_fd)
PY
}

wahrwelt_acquired_lock_dir="${wahrwelt_acquired_lock_dir:-}"
wahrwelt_acquired_lock_identity="${wahrwelt_acquired_lock_identity:-}"
wahrwelt_known_lock_identity=""
wahrwelt_new_lock_parent_fd=""
wahrwelt_new_lock_fd=""
wahrwelt_new_lock_parent_identity=""
wahrwelt_new_lock_identity=""
wahrwelt_new_lock_parent_path=""
wahrwelt_new_lock_path=""
wahrwelt_new_lock_target_path=""
wahrwelt_new_lock_staging_name=""
wahrwelt_new_lock_staging_parent_identity=""
wahrwelt_new_lock_publish_state=""

# Lock metadata is attacker-controlled until it has been checked through a
# pinned directory descriptor.  Do not use cat here: opening a FIFO would
# otherwise block the shell that is trying to reject the collision.
wahrwelt_read_pinned_regular_file() {
  local dir_fd="$1"
  local name="$2"
  local limit="${3:-4096}"

  command -v python3 >/dev/null 2>&1 || return 1
  python3 - "$dir_fd" "$name" "$limit" <<'PY'
import os
import stat
import sys

parent_fd = int(sys.argv[1])
name = os.fsencode(sys.argv[2])
limit = int(sys.argv[3])
flags = os.O_RDONLY | os.O_CLOEXEC | os.O_NONBLOCK | os.O_NOFOLLOW
fd = os.open(name, flags, dir_fd=parent_fd)
try:
    info = os.fstat(fd)
    if not stat.S_ISREG(info.st_mode) or info.st_nlink != 1:
        raise OSError("lock metadata is not a private regular file")
    chunks = []
    remaining = limit + 1
    while remaining:
        chunk = os.read(fd, remaining)
        if not chunk:
            break
        chunks.append(chunk)
        remaining -= len(chunk)
    value = b"".join(chunks)
    if len(value) > limit:
        raise OSError("lock metadata is too large")
    sys.stdout.buffer.write(value)
finally:
    os.close(fd)
PY
}

wahrwelt_write_new_pinned_regular_file() {
  local dir_fd="$1"
  local name="$2"
  local value="$3"

  command -v python3 >/dev/null 2>&1 || return 1
  python3 - "$dir_fd" "$name" "$value" <<'PY'
import os
import stat
import sys

parent_fd = int(sys.argv[1])
name = os.fsencode(sys.argv[2])
value = os.fsencode(sys.argv[3])
flags = os.O_WRONLY | os.O_CLOEXEC | os.O_NOFOLLOW | os.O_CREAT | os.O_EXCL
fd = os.open(name, flags, 0o600, dir_fd=parent_fd)
try:
    info = os.fstat(fd)
    if not stat.S_ISREG(info.st_mode) or info.st_nlink != 1:
        raise OSError("new lock metadata is not a private regular file")
    offset = 0
    while offset < len(value):
        offset += os.write(fd, value[offset:])
    os.fsync(fd)
finally:
    os.close(fd)
PY
}

# Return a complete, exact journal for a small private directory.  The caller
# supplies every allowed basename; anything else is a collision.  These
# helpers intentionally never recurse, so an injected subdirectory can only
# make cleanup fail closed rather than become a deletion target.
wahrwelt_snapshot_pinned_regular_entries() {
  local dir_fd="$1"
  shift

  command -v python3 >/dev/null 2>&1 || return 1
  python3 - "$dir_fd" "$@" <<'PY'
import os
import stat
import sys

fd = int(sys.argv[1])
names = sys.argv[2:]
if len(set(names)) != len(names) or set(os.listdir(fd)) != set(names):
    raise OSError("unexpected private-directory entries")
for name in names:
    info = os.stat(name, dir_fd=fd, follow_symlinks=False)
    if not stat.S_ISREG(info.st_mode) or info.st_nlink != 1:
        raise OSError("private-directory entry is not a private regular file")
    identity = ":".join(map(str, (
        info.st_dev, info.st_ino, info.st_mode, info.st_nlink, info.st_size,
        info.st_mtime_ns, info.st_ctime_ns,
    )))
    print(f"{name}\t{identity}")
PY
}

wahrwelt_delete_exact_pinned_regular_entries() {
  # A same-UID process can replace a journal name after its identity check and
  # before unlinkat. Keep the complete recovery instead of deleting through a
  # mutable namespace one leaf at a time.
  : "$@"
  return 1
}

wahrwelt_close_new_lock_directory() {
  if [ -n "$wahrwelt_new_lock_fd" ]; then
    exec {wahrwelt_new_lock_fd}<&-
  fi
  if [ -n "$wahrwelt_new_lock_parent_fd" ]; then
    exec {wahrwelt_new_lock_parent_fd}<&-
  fi
  wahrwelt_new_lock_parent_fd=""
  wahrwelt_new_lock_fd=""
  wahrwelt_new_lock_parent_identity=""
  wahrwelt_new_lock_identity=""
  wahrwelt_new_lock_parent_path=""
  wahrwelt_new_lock_path=""
  wahrwelt_new_lock_target_path=""
  wahrwelt_new_lock_staging_name=""
  wahrwelt_new_lock_staging_parent_identity=""
  wahrwelt_new_lock_publish_state=""
}

# Create a lock in an unpublished staging directory below the pinned runtime
# root. The caller writes every metadata leaf through wahrwelt_new_lock_fd;
# finish atomically publishes the complete directory below the target parent.
# A crash can therefore leave only a hidden recovery at the runtime root, not
# an incomplete public lock inside a managed application-state namespace.
wahrwelt_begin_new_lock_directory() {
  local lock_dir="$1"
  local parent base parent_identity opened_identity leaf_identity root_identity

  wahrwelt_close_new_lock_directory
  wahrwelt_new_lock_publish_state="staging"
  parent="$(dirname -- "$lock_dir")"
  base="${lock_dir##*/}"
  case "$base" in "" | . | .. | */*) return 1 ;; esac
  parent_identity="$(wahrwelt_lock_identity "$parent" 2>/dev/null || true)"
  [ -n "$parent_identity" ] || return 1
  exec {wahrwelt_new_lock_parent_fd}<"$parent" || return 1
  wahrwelt_new_lock_parent_path="/proc/${BASHPID:-$$}/fd/$wahrwelt_new_lock_parent_fd"
  opened_identity="$(wahrwelt_opened_directory_identity "$wahrwelt_new_lock_parent_path" 2>/dev/null || true)"
  if [ "$opened_identity" != "$parent_identity" ]; then
    wahrwelt_close_new_lock_directory
    return 1
  fi
  if [ -e "$wahrwelt_new_lock_parent_path/$base" ] ||
    [ -L "$wahrwelt_new_lock_parent_path/$base" ]; then
    wahrwelt_close_new_lock_directory
    return 1
  fi
  root_identity="$(wahrwelt_opened_directory_identity "$wahrwelt_runtime_session_dir" 2>/dev/null || true)"
  [ -n "$root_identity" ] || {
    wahrwelt_close_new_lock_directory
    return 1
  }
  if ! wahrwelt_create_pinned_private_directory \
    "$wahrwelt_runtime_session_fd" prefix ".wahrwelt-lock-staging-${base}-" lock-staging; then
    wahrwelt_close_new_lock_directory
    return 1
  fi
  wahrwelt_new_lock_fd="$wahrwelt_created_directory_fd"
  wahrwelt_new_lock_path="$wahrwelt_created_directory_path"
  wahrwelt_new_lock_staging_name="$wahrwelt_created_directory_name"
  leaf_identity="$wahrwelt_created_directory_identity"
  if declare -F wahrwelt_after_lock_directory_create_hook >/dev/null 2>&1; then
    wahrwelt_after_lock_directory_create_hook "$lock_dir" "$wahrwelt_new_lock_path" || {
      wahrwelt_close_new_lock_directory
      return 1
    }
  fi
  if [ "$(wahrwelt_opened_directory_identity "$wahrwelt_new_lock_path" 2>/dev/null || true)" != "$leaf_identity" ] ||
    [ "$(wahrwelt_lock_identity "$wahrwelt_runtime_session_dir/$wahrwelt_new_lock_staging_name" 2>/dev/null || true)" != "$leaf_identity" ] ||
    [ "$(wahrwelt_opened_directory_identity "$wahrwelt_runtime_session_dir" 2>/dev/null || true)" != "$root_identity" ] ||
    [ "$(wahrwelt_opened_directory_identity "$wahrwelt_new_lock_parent_path" 2>/dev/null || true)" != "$parent_identity" ] ||
    [ -e "$wahrwelt_new_lock_parent_path/$base" ] ||
    [ -L "$wahrwelt_new_lock_parent_path/$base" ]; then
    wahrwelt_close_new_lock_directory
    return 1
  fi
  wahrwelt_new_lock_parent_identity="$parent_identity"
  wahrwelt_new_lock_identity="$leaf_identity"
  wahrwelt_new_lock_target_path="$lock_dir"
  wahrwelt_new_lock_staging_parent_identity="$root_identity"
}

wahrwelt_finish_new_lock_directory() {
  local lock_dir="$1"
  local base rename_status

  [ -n "$wahrwelt_new_lock_fd" ] && [ -n "$wahrwelt_new_lock_parent_fd" ] || return 1
  [ "$lock_dir" = "$wahrwelt_new_lock_target_path" ] || return 1
  base="${lock_dir##*/}"
  if [ "$(wahrwelt_opened_directory_identity "$wahrwelt_new_lock_path" 2>/dev/null || true)" != "$wahrwelt_new_lock_identity" ] ||
    [ "$(wahrwelt_lock_identity "$wahrwelt_runtime_session_dir/$wahrwelt_new_lock_staging_name" 2>/dev/null || true)" != "$wahrwelt_new_lock_identity" ] ||
    [ "$(wahrwelt_opened_directory_identity "$wahrwelt_runtime_session_dir" 2>/dev/null || true)" != "$wahrwelt_new_lock_staging_parent_identity" ] ||
    [ "$(wahrwelt_opened_directory_identity "$wahrwelt_new_lock_parent_path" 2>/dev/null || true)" != "$wahrwelt_new_lock_parent_identity" ]; then
    return 1
  fi
  if [ -e "$wahrwelt_new_lock_parent_path/$base" ] ||
    [ -L "$wahrwelt_new_lock_parent_path/$base" ]; then
    wahrwelt_new_lock_publish_state="collision"
    return 1
  fi
  if declare -F wahrwelt_before_lock_directory_publish_hook >/dev/null 2>&1; then
    wahrwelt_before_lock_directory_publish_hook "$lock_dir" "$wahrwelt_new_lock_path" || return 1
  fi
  wahrwelt_sync_pinned_directory "$wahrwelt_new_lock_fd" || return 1
  rename_status=0
  wahrwelt_rename_noreplace_between_pinned_directories \
    "$wahrwelt_runtime_session_fd" "$wahrwelt_new_lock_staging_name" \
    "$wahrwelt_new_lock_parent_fd" "$base" 2>/dev/null || rename_status=$?
  if [ "$rename_status" -ne 0 ]; then
    if [ "$rename_status" -eq 17 ]; then
      wahrwelt_new_lock_publish_state="collision"
    fi
    return 1
  fi
  wahrwelt_new_lock_publish_state="published"
  wahrwelt_sync_pinned_directory "$wahrwelt_runtime_session_fd" || return 1
  wahrwelt_sync_pinned_directory "$wahrwelt_new_lock_parent_fd" || return 1
  if [ "$(wahrwelt_opened_directory_identity "$wahrwelt_new_lock_path" 2>/dev/null || true)" != "$wahrwelt_new_lock_identity" ] ||
    [ "$(wahrwelt_opened_directory_identity "$wahrwelt_new_lock_parent_path" 2>/dev/null || true)" != "$wahrwelt_new_lock_parent_identity" ] ||
    [ "$(wahrwelt_lock_identity "$wahrwelt_new_lock_parent_path/$base" 2>/dev/null || true)" != "$wahrwelt_new_lock_identity" ]; then
    return 1
  fi
  wahrwelt_acquired_lock_dir="$lock_dir"
  wahrwelt_acquired_lock_identity="$wahrwelt_new_lock_identity"
  wahrwelt_close_new_lock_directory
}

wahrwelt_read_known_lock_field() {
  local lock_dir="$1"
  local field="$2"
  local expected fd pinned value

  expected="$(wahrwelt_lock_identity "$lock_dir" 2>/dev/null || true)"
  [ -n "$expected" ] || return 1
  exec {fd}<"$lock_dir" 2>/dev/null || return 1
  pinned="/proc/${BASHPID:-$$}/fd/$fd"
  if [ "$(wahrwelt_opened_directory_identity "$pinned" 2>/dev/null || true)" != "$expected" ]; then
    exec {fd}<&-
    return 1
  fi
  value="$(wahrwelt_read_pinned_regular_file "$fd" "$field" 2>/dev/null || true)"
  if [ "$(wahrwelt_lock_identity "$lock_dir" 2>/dev/null || true)" != "$expected" ]; then
    exec {fd}<&-
    return 1
  fi
  exec {fd}<&-
  printf '%s' "$value"
}

wahrwelt_known_lock_directory() {
  local lock_dir="$1"
  local pid_file="$2"
  local owner_file="$3"
  local owner_name="$4"
  local allowed_entries="${5:-}"
  local entry allowed expected fd pinned entries pinned_input=0

  wahrwelt_known_lock_identity=""
  case "$lock_dir" in
    "/proc/${BASHPID:-$$}/fd/"*)
      pinned_input=1
      expected="$(wahrwelt_opened_directory_identity "$lock_dir" 2>/dev/null || true)"
      ;;
    *) expected="$(wahrwelt_lock_identity "$lock_dir" 2>/dev/null || true)" ;;
  esac
  [ -n "$expected" ] || return 1
  exec {fd}<"$lock_dir" 2>/dev/null || return 1
  pinned="/proc/${BASHPID:-$$}/fd/$fd"
  if [ "$(wahrwelt_opened_directory_identity "$pinned" 2>/dev/null || true)" != "$expected" ]; then
    exec {fd}<&-
    return 1
  fi
  [ -f "$pinned/${pid_file##*/}" ] && [ ! -L "$pinned/${pid_file##*/}" ] || {
    exec {fd}<&-
    return 1
  }
  [ -f "$pinned/${owner_file##*/}" ] && [ ! -L "$pinned/${owner_file##*/}" ] || {
    exec {fd}<&-
    return 1
  }
  if [ "$(wahrwelt_read_pinned_regular_file "$fd" "${owner_file##*/}" 2>/dev/null || true)" != "$owner_name" ]; then
    exec {fd}<&-
    return 1
  fi
  if ! entries="$(find -P "$pinned/." -mindepth 1 -maxdepth 1 -printf '%f:%y\n' 2>/dev/null)"; then
    exec {fd}<&-
    return 1
  fi
  while IFS= read -r entry; do
    [ -n "$entry" ] || continue
    case "$entry" in
      "$(basename -- "$pid_file")":f | "$(basename -- "$owner_file")":f) ;;
      *)
        for allowed in $allowed_entries; do
          [ "$entry" = "$allowed" ] && break
        done
        if [ "${allowed:-}" != "$entry" ]; then
          exec {fd}<&-
          return 1
        fi
        ;;
    esac
  done <<<"$entries"
  if [ "$(wahrwelt_opened_directory_identity "$pinned" 2>/dev/null || true)" != "$expected" ] ||
    { [ "$pinned_input" -eq 0 ] && [ "$(wahrwelt_lock_identity "$lock_dir" 2>/dev/null || true)" != "$expected" ]; }; then
    exec {fd}<&-
    return 1
  fi
  exec {fd}<&-
  wahrwelt_known_lock_identity="$expected"
}

wahrwelt_known_managed_lock_directory() {
  local lock_dir="$1"
  local owner

  for owner in wahrwelt-noctalia-launcher wahrwelt-record-toggle wahrwelt-shell-selector; do
    if wahrwelt_known_lock_directory "$lock_dir" "$lock_dir/pid" "$lock_dir/owner" "$owner"; then
      return 0
    fi
  done
  wahrwelt_known_lock_directory "$lock_dir" "$lock_dir/pid" "$lock_dir/owner" \
    wahrwelt-start-shell "profile:f"
}

wahrwelt_exchange_pinned_names() {
  local parent_fd="$1"
  local left="$2"
  local right="$3"

  command -v python3 >/dev/null 2>&1 || return 1
  python3 - "$parent_fd" "$left" "$right" <<'PY'
import ctypes
import os
import sys

parent_fd = int(sys.argv[1])
left = os.fsencode(sys.argv[2])
right = os.fsencode(sys.argv[3])
libc = ctypes.CDLL(None, use_errno=True)
renameat2 = libc.renameat2
renameat2.argtypes = [ctypes.c_int, ctypes.c_char_p, ctypes.c_int, ctypes.c_char_p, ctypes.c_uint]
renameat2.restype = ctypes.c_int
if renameat2(parent_fd, left, parent_fd, right, 2) != 0:  # RENAME_EXCHANGE
    raise OSError(ctypes.get_errno(), os.strerror(ctypes.get_errno()))
PY
}

wahrwelt_rename_noreplace_pinned_names() {
  local parent_fd="$1"
  local source="$2"
  local target="$3"

  command -v python3 >/dev/null 2>&1 || return 1
  python3 - "$parent_fd" "$source" "$target" <<'PY'
import ctypes
import os
import sys

parent_fd = int(sys.argv[1])
source = os.fsencode(sys.argv[2])
target = os.fsencode(sys.argv[3])
libc = ctypes.CDLL(None, use_errno=True)
renameat2 = libc.renameat2
renameat2.argtypes = [ctypes.c_int, ctypes.c_char_p, ctypes.c_int, ctypes.c_char_p, ctypes.c_uint]
renameat2.restype = ctypes.c_int
if renameat2(parent_fd, source, parent_fd, target, 1) != 0:  # RENAME_NOREPLACE
    raise OSError(ctypes.get_errno(), os.strerror(ctypes.get_errno()))
PY
}

wahrwelt_rename_noreplace_between_pinned_directories() {
  local source_parent_fd="$1"
  local source="$2"
  local target_parent_fd="$3"
  local target="$4"

  command -v python3 >/dev/null 2>&1 || return 1
  python3 -I -S - "$source_parent_fd" "$source" "$target_parent_fd" "$target" <<'PY'
import ctypes
import errno
import os
import sys

source_parent_fd = int(sys.argv[1])
source = os.fsencode(sys.argv[2])
target_parent_fd = int(sys.argv[3])
target = os.fsencode(sys.argv[4])
for name in (source, target):
    if not name or b"/" in name or name in (b".", b".."):
        raise OSError("invalid pinned rename basename")
libc = ctypes.CDLL(None, use_errno=True)
renameat2 = libc.renameat2
renameat2.argtypes = [ctypes.c_int, ctypes.c_char_p, ctypes.c_int, ctypes.c_char_p, ctypes.c_uint]
renameat2.restype = ctypes.c_int
if renameat2(source_parent_fd, source, target_parent_fd, target, 1) != 0:  # RENAME_NOREPLACE
    error = ctypes.get_errno()
    if error == errno.EEXIST:
        raise SystemExit(errno.EEXIST)
    raise OSError(error, os.strerror(error))
PY
}

wahrwelt_sync_pinned_directory() {
  local fd="$1"

  command -v python3 >/dev/null 2>&1 || return 1
  python3 -I -S - "$fd" <<'PY'
import os
import sys

fd = os.dup(int(sys.argv[1]))
try:
    os.fsync(fd)
finally:
    os.close(fd)
PY
}

wahrwelt_quarantine_owned_lock() {
  local lock_dir="$1"
  local expected_identity="$2"
  local parent base parent_identity fd pinned attempt candidate moved_identity recovery_pinned root_identity

  wahrwelt_close_lock_recovery
  parent="$(dirname -- "$lock_dir")"
  base="${lock_dir##*/}"
  parent_identity="$(wahrwelt_lock_identity "$parent" 2>/dev/null || true)"
  [ -n "$parent_identity" ] || return 1
  exec {fd}<"$parent" || return 1
  pinned="/proc/${BASHPID:-$$}/fd/$fd"
  root_identity="$(wahrwelt_opened_directory_identity "$wahrwelt_runtime_session_dir" 2>/dev/null || true)"
  if [ "$(wahrwelt_opened_directory_identity "$pinned" 2>/dev/null || true)" != "$parent_identity" ] ||
    [ -z "$root_identity" ] ||
    [ "$(wahrwelt_lock_identity "$pinned/$base" 2>/dev/null || true)" != "$expected_identity" ]; then
    exec {fd}<&-
    return 1
  fi
  for attempt in $(seq 1 64); do
    candidate=".wahrwelt-lock-quarantine-${BASHPID:-$$}-${RANDOM}-${attempt}"
    if declare -F wahrwelt_before_lock_exchange_hook >/dev/null 2>&1; then
      wahrwelt_before_lock_exchange_hook "$lock_dir" || {
        exec {fd}<&-
        return 1
      }
    fi
    if declare -F wahrwelt_before_lock_recovery_publish_hook >/dev/null 2>&1; then
      wahrwelt_before_lock_recovery_publish_hook \
        "$lock_dir" "$wahrwelt_runtime_session_dir/$candidate" || {
        exec {fd}<&-
        return 1
      }
    fi
    if ! wahrwelt_rename_noreplace_between_pinned_directories \
      "$fd" "$base" "$wahrwelt_runtime_session_fd" "$candidate" 2>/dev/null; then
      if [ "$(wahrwelt_opened_directory_identity "$pinned" 2>/dev/null || true)" != "$parent_identity" ] ||
        [ "$(wahrwelt_opened_directory_identity "$wahrwelt_runtime_session_dir" 2>/dev/null || true)" != "$root_identity" ] ||
        [ "$(wahrwelt_lock_identity "$pinned/$base" 2>/dev/null || true)" != "$expected_identity" ]; then
        exec {fd}<&-
        return 1
      fi
      continue
    fi
    moved_identity="$(wahrwelt_lock_identity "$wahrwelt_runtime_session_dir/$candidate" 2>/dev/null || true)"
    if [ "$moved_identity" = "$expected_identity" ]; then
      if exec {wahrwelt_lock_recovery_fd}<"$wahrwelt_runtime_session_dir/$candidate"; then
        recovery_pinned="/proc/${BASHPID:-$$}/fd/$wahrwelt_lock_recovery_fd"
        if [ "$(wahrwelt_opened_directory_identity "$recovery_pinned" 2>/dev/null || true)" = "$expected_identity" ]; then
          wahrwelt_lock_recovery_fd_path="$recovery_pinned"
          wahrwelt_lock_recovery_public_path="$wahrwelt_runtime_session_dir/$candidate"
          wahrwelt_lock_collision_path="$lock_dir"
        else
          exec {wahrwelt_lock_recovery_fd}<&-
          wahrwelt_lock_recovery_fd=""
        fi
      fi
      if [ -n "$wahrwelt_lock_recovery_fd_path" ] &&
        wahrwelt_sync_pinned_directory "$fd" &&
        wahrwelt_sync_pinned_directory "$wahrwelt_runtime_session_fd" &&
        [ "$(wahrwelt_opened_directory_identity "$pinned" 2>/dev/null || true)" = "$parent_identity" ] &&
        [ "$(wahrwelt_opened_directory_identity "$wahrwelt_runtime_session_dir" 2>/dev/null || true)" = "$root_identity" ] &&
        [ "$(wahrwelt_opened_directory_identity "$wahrwelt_lock_recovery_fd_path" 2>/dev/null || true)" = "$expected_identity" ] &&
        wahrwelt_refresh_lock_recovery_report "$expected_identity"; then
        exec {fd}<&-
        return 0
      fi
      exec {fd}<&-
      return 1
    fi
    # A racer replaced the classified source. Restore the moved unknown node
    # only with an atomic no-replace rename; if the public name is occupied,
    # retain the unknown node at the runtime-root recovery name.
    wahrwelt_rename_noreplace_between_pinned_directories \
      "$wahrwelt_runtime_session_fd" "$candidate" "$fd" "$base" 2>/dev/null || true
    wahrwelt_sync_pinned_directory "$fd" 2>/dev/null || true
    wahrwelt_sync_pinned_directory "$wahrwelt_runtime_session_fd" 2>/dev/null || true
    exec {fd}<&-
    return 1
  done
  exec {fd}<&-
  return 1
}

wahrwelt_release_owned_lock() {
  local lock_dir="$1"
  local expected_identity="$2"
  local recovery recovery_fd_path public_recovery

  [ -n "$expected_identity" ] || return 1
  if ! wahrwelt_quarantine_owned_lock "$lock_dir" "$expected_identity" 2>/dev/null; then
    if [ -n "$wahrwelt_lock_recovery_fd_path" ]; then
      printf 'Wahrwelt lock recovery retained at live descriptor %s identity %s; durable path %s; public collision %s identity %s\n' \
        "$wahrwelt_lock_recovery_fd_path" "${wahrwelt_lock_recovery_identity:-$expected_identity}" \
        "${wahrwelt_lock_recovery_exact_path:-unproven}" "${wahrwelt_lock_collision_path:-$lock_dir}" \
        "${wahrwelt_lock_collision_identity:-absent}" >&2
    fi
    return 1
  fi
  recovery="$wahrwelt_lock_recovery_exact_path"
  recovery_fd_path="$wahrwelt_lock_recovery_fd_path"
  public_recovery="$wahrwelt_lock_recovery_public_path"
  [ -n "$recovery" ] && [ -n "$recovery_fd_path" ] || return 1
  if [ "$(wahrwelt_opened_directory_identity "$recovery_fd_path" 2>/dev/null || true)" != "$expected_identity" ] ||
    ! wahrwelt_known_managed_lock_directory "$recovery_fd_path"; then
    return 1
  fi
  if declare -F wahrwelt_before_lock_release_delete_hook >/dev/null 2>&1; then
    wahrwelt_before_lock_release_delete_hook "$recovery_fd_path" || {
      return 1
    }
  fi
  if [ "$(wahrwelt_opened_directory_identity "$recovery_fd_path" 2>/dev/null || true)" != "$expected_identity" ] ||
    ! wahrwelt_known_managed_lock_directory "$recovery_fd_path" ||
    ! wahrwelt_refresh_lock_recovery_report "$expected_identity"; then
    return 1
  fi
  recovery="$wahrwelt_lock_recovery_exact_path"
  printf 'Wahrwelt lock recovery retained at %s identity %s; live descriptor %s; public recovery name %s\n' \
    "$recovery" "$wahrwelt_lock_recovery_identity" "$recovery_fd_path" "$public_recovery" >&2
}

wahrwelt_lock_path_absent() {
  local lock_dir="$1"

  [ ! -e "$lock_dir" ] && [ ! -L "$lock_dir" ]
}

wahrwelt_acquire_lock() {
  local lock_dir="$1"
  local pid_file="$2"
  local owner_file="$3"
  local owner_name="$4"
  local owner_pattern="$5"
  local attempts="${6:-20}"
  local delay="${7:-0.02}"
  local owner_pid lock_identity recovery publish_state

  wahrwelt_acquired_lock_dir=""
  wahrwelt_acquired_lock_identity=""

  for _ in $(seq 1 "$attempts"); do
    if wahrwelt_begin_new_lock_directory "$lock_dir"; then
      if ! wahrwelt_write_new_pinned_regular_file "$wahrwelt_new_lock_fd" "${pid_file##*/}" "$$
" ||
        ! wahrwelt_write_new_pinned_regular_file "$wahrwelt_new_lock_fd" "${owner_file##*/}" "$owner_name
"; then
        wahrwelt_close_new_lock_directory
        return 1
      fi
      if wahrwelt_finish_new_lock_directory "$lock_dir"; then
        if wahrwelt_known_lock_directory "$lock_dir" "$pid_file" "$owner_file" "$owner_name"; then
          return 0
        fi
        return 1
      fi
      publish_state="$wahrwelt_new_lock_publish_state"
      wahrwelt_close_new_lock_directory
      [ "$publish_state" = collision ] || return 1
    fi

    if declare -F wahrwelt_after_new_lock_begin_failed_hook >/dev/null 2>&1; then
      wahrwelt_after_new_lock_begin_failed_hook "$lock_dir" || return 1
    fi

    if ! wahrwelt_known_lock_directory "$lock_dir" "$pid_file" "$owner_file" "$owner_name"; then
      wahrwelt_lock_path_absent "$lock_dir" && continue
      return 1
    fi
    owner_pid="$(wahrwelt_read_known_lock_field "$lock_dir" "${pid_file##*/}" 2>/dev/null || true)"
    if declare -F wahrwelt_after_lock_owner_read_hook >/dev/null 2>&1; then
      wahrwelt_after_lock_owner_read_hook "$lock_dir" "$owner_pid" || return 1
    fi
    if wahrwelt_lock_owner_running "$owner_pid" "$owner_file" "$owner_name" "$owner_pattern"; then
      sleep "$delay"
      continue
    fi

    if ! wahrwelt_known_lock_directory "$lock_dir" "$pid_file" "$owner_file" "$owner_name"; then
      wahrwelt_lock_path_absent "$lock_dir" && continue
      return 1
    fi
    lock_identity="$wahrwelt_known_lock_identity"
    [ -n "$lock_identity" ] || return 1
    if declare -F wahrwelt_after_lock_classification_hook >/dev/null 2>&1; then
      wahrwelt_after_lock_classification_hook "$lock_dir" "$lock_identity" || return 1
    fi
    if ! wahrwelt_quarantine_owned_lock "$lock_dir" "$lock_identity" 2>/dev/null; then
      if wahrwelt_lock_path_absent "$lock_dir"; then
        continue
      fi
      [ -z "$wahrwelt_lock_recovery_fd_path" ] ||
        printf 'Wahrwelt stale lock retained at live descriptor %s identity %s; durable path %s; public collision %s identity %s\n' \
          "$wahrwelt_lock_recovery_fd_path" "${wahrwelt_lock_recovery_identity:-$lock_identity}" \
          "${wahrwelt_lock_recovery_exact_path:-unproven}" "${wahrwelt_lock_collision_path:-$lock_dir}" \
          "${wahrwelt_lock_collision_identity:-absent}" >&2
      return 1
    fi
    recovery="$wahrwelt_lock_recovery_exact_path"
    [ -n "$recovery" ] || return 1
  done

  return 1
}
