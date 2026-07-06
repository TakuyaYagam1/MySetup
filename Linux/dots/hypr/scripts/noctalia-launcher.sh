#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=Linux/dots/hypr/scripts/shell-runtime.sh
. "$script_dir/shell-runtime.sh"

state_dir="${XDG_RUNTIME_DIR:-/tmp}/mysetup-noctalia-launcher"
active_file="$state_dir/active"
interrupt_file="$state_dir/interrupted"
lock_dir="$state_dir/lock"
lock_pid_file="$lock_dir/pid"
lock_owner_file="$lock_dir/owner"

mkdir -p "$state_dir"

acquire_lock() {
  mysetup_acquire_lock \
    "$lock_dir" \
    "$lock_pid_file" \
    "$lock_owner_file" \
    "mysetup-noctalia-launcher" \
    '(^|[ /])noctalia-launcher\.sh([[:space:]]|$)' \
    20 \
    0.02 || exit 0
}

acquire_lock
trap 'rm -rf -- "$lock_dir" 2>/dev/null || true' EXIT

case "${1:-release}" in
  press)
    rm -f "$interrupt_file"
    : >"$active_file"
    ;;
  interrupt)
    if [ -e "$active_file" ]; then
      : >"$interrupt_file"
    fi
    ;;
  release)
    if [ -e "$active_file" ] && [ ! -e "$interrupt_file" ]; then
      mysetup_noctalia_msg launcher toggle >/dev/null 2>&1 || true
    fi
    rm -f "$active_file" "$interrupt_file"
    ;;
  *)
    printf 'usage: %s {press|interrupt|release}\n' "$0" >&2
    exit 2
    ;;
esac
