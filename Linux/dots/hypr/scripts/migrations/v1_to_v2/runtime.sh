#!/usr/bin/env bash
# Wahrwelt v1_to_v2 runtime recognizers.
# This file is sourced lazily by shell-runtime.sh only after exact v1 evidence.
# It relies on the canonical fd-based helpers already loaded by shell-runtime.sh.
# shellcheck disable=SC2154

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
