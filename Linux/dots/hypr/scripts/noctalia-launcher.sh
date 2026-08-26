#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=Linux/dots/hypr/scripts/shell-runtime.sh
. "$script_dir/shell-runtime.sh"

if ! wahrwelt_adopt_legacy_private_state_directory wahrwelt-noctalia-launcher noctalia-launcher-state; then
  printf 'Wahrwelt noctalia launcher ownership collision: unsafe pre-marker state preserved\n' >&2
  exit 1
fi
if ! wahrwelt_open_private_state_directory wahrwelt-noctalia-launcher noctalia-launcher-state; then
  printf 'Wahrwelt noctalia launcher ownership collision: %s/wahrwelt-noctalia-launcher\n' \
    "$wahrwelt_runtime_session_public_dir" >&2
  exit 1
fi
state_dir="$wahrwelt_private_state_directory_path"
state_dir_fd="$wahrwelt_private_state_directory_fd"
if ! wahrwelt_open_managed_regular_file "$state_dir_fd" "$state_dir" active noctalia-active; then
  printf 'Wahrwelt noctalia launcher ownership collision: active marker preserved\n' >&2
  exit 1
fi
exec {active_fd}<&"$wahrwelt_managed_regular_fd"
active_file="/proc/${BASHPID:-$$}/fd/$active_fd"
exec {wahrwelt_managed_regular_fd}<&-
wahrwelt_managed_regular_fd=""
if ! wahrwelt_open_managed_regular_file "$state_dir_fd" "$state_dir" interrupted noctalia-interrupted; then
  printf 'Wahrwelt noctalia launcher ownership collision: interrupted marker preserved\n' >&2
  exit 1
fi
exec {interrupt_fd}<&"$wahrwelt_managed_regular_fd"
interrupt_file="/proc/${BASHPID:-$$}/fd/$interrupt_fd"
exec {wahrwelt_managed_regular_fd}<&-
wahrwelt_managed_regular_fd=""
lock_dir="$state_dir/lock"
lock_pid_file="$lock_dir/pid"
lock_owner_file="$lock_dir/owner"
lock_identity=""

acquire_lock() {
  wahrwelt_acquire_lock \
    "$lock_dir" \
    "$lock_pid_file" \
    "$lock_owner_file" \
    "wahrwelt-noctalia-launcher" \
    '(^|[ /])noctalia-launcher\.sh([[:space:]]|$)' \
    20 \
    0.02 || exit 0
  lock_identity="$wahrwelt_acquired_lock_identity"
  [ -n "$lock_identity" ] || exit 1
}

cleanup_lock() {
  local recovery

  [ -n "$lock_identity" ] || return 0
  if ! wahrwelt_release_owned_lock "$lock_dir" "$lock_identity" 2>/dev/null; then
    recovery="${wahrwelt_lock_recovery_exact_path:-$lock_dir}"
    printf 'wahrwelt noctalia launcher lock changed during cleanup; preserving recovery/collision: %s\n' \
      "$recovery" >&2
  fi
}

acquire_lock
trap cleanup_lock EXIT

case "${1:-release}" in
  press)
    printf '%s\n' 0 >"$interrupt_file"
    printf '%s\n' 1 >"$active_file"
    ;;
  interrupt)
    if [ "$(cat "$active_file" 2>/dev/null || true)" = 1 ]; then
      printf '%s\n' 1 >"$interrupt_file"
    fi
    ;;
  release)
    if [ "$(cat "$active_file" 2>/dev/null || true)" = 1 ] &&
      [ "$(cat "$interrupt_file" 2>/dev/null || true)" != 1 ]; then
      wahrwelt_noctalia_action launcher-toggle >/dev/null 2>&1 || true
    fi
    printf '%s\n' 0 >"$active_file"
    printf '%s\n' 0 >"$interrupt_file"
    ;;
  *)
    printf 'usage: %s {press|interrupt|release}\n' "$0" >&2
    exit 2
    ;;
esac
