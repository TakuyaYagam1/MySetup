#!/usr/bin/env bash
set -Eeuo pipefail

state_dir="${XDG_RUNTIME_DIR:-/tmp}/mysetup-noctalia-launcher"
active_file="$state_dir/active"
interrupt_file="$state_dir/interrupted"
lock_dir="$state_dir/lock"
lock_pid_file="$lock_dir/pid"
lock_owner_file="$lock_dir/owner"

mkdir -p "$state_dir"

lock_owner_running() {
  local owner_pid="$1"
  local owner_name

  owner_name="$(cat "$lock_owner_file" 2>/dev/null || true)"
  [ "$owner_name" = "mysetup-noctalia-launcher" ] || return 1
  [ -n "$owner_pid" ] || return 1
  ps -p "$owner_pid" -o args= 2>/dev/null | grep -qE '(^|[ /])noctalia-launcher\.sh([[:space:]]|$)'
}

acquire_lock() {
  local owner_pid

  for _ in $(seq 1 20); do
    if mkdir "$lock_dir" 2>/dev/null; then
      printf '%s\n' "$$" >"$lock_pid_file"
      printf '%s\n' "mysetup-noctalia-launcher" >"$lock_owner_file"
      return 0
    fi

    owner_pid="$(cat "$lock_pid_file" 2>/dev/null || true)"
    if lock_owner_running "$owner_pid"; then
      sleep 0.02
      continue
    fi

    rm -rf -- "$lock_dir"
  done

  exit 0
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
      noctalia-shell ipc call launcher toggle >/dev/null 2>&1 || true
    fi
    rm -f "$active_file" "$interrupt_file"
    ;;
  *)
    printf 'usage: %s {press|interrupt|release}\n' "$0" >&2
    exit 2
    ;;
esac
