#!/usr/bin/env bash
set -uo pipefail

profile="${1:-caelestia}"
runtime_dir="${XDG_RUNTIME_DIR:-/tmp}"
state_file="$runtime_dir/mysetup-hypr-shell-profile"
log_file="$runtime_dir/mysetup-shell.log"
lock_dir="$runtime_dir/mysetup-shell.lock"
user_name="${USER:-}"

if [ -z "$user_name" ]; then
  user_name="$(id -un 2>/dev/null || printf '%s' user)"
fi

log() {
  printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*" >>"$log_file"
}

if ! mkdir "$lock_dir" 2>/dev/null; then
  lock_pid="$(cat "$lock_dir/pid" 2>/dev/null || true)"
  if [ -n "$lock_pid" ] && kill -0 "$lock_pid" 2>/dev/null; then
    log "another start-shell instance is already running; profile=$profile pid=$lock_pid"
    exit 0
  fi

  log "removing stale start-shell lock; profile=$profile pid=${lock_pid:-unknown}"
  rm -rf "$lock_dir"
  if ! mkdir "$lock_dir" 2>/dev/null; then
    log "failed to acquire start-shell lock after stale cleanup; profile=$profile"
    exit 0
  fi
fi
printf '%s\n' "$$" >"$lock_dir/pid"
trap 'rm -rf "$lock_dir" 2>/dev/null || true' EXIT

is_running() {
  pgrep -u "$user_name" -f "$1" >/dev/null 2>&1
}

wait_for_session() {
  local attempt
  for attempt in $(seq 1 40); do
    if [ -n "${WAYLAND_DISPLAY:-}" ] && [ -n "${HYPRLAND_INSTANCE_SIGNATURE:-}" ]; then
      if command -v hyprctl >/dev/null 2>&1; then
        hyprctl monitors >/dev/null 2>&1 && return 0
      else
        return 0
      fi
    fi
    sleep 0.25
  done
  log "session readiness timeout; continuing anyway"
  return 0
}

stop_quickshells() {
  log "stopping existing quickshell instances"

  if command -v caelestia >/dev/null 2>&1; then
    caelestia shell -k >/dev/null 2>&1 || true
  fi

  if command -v qs >/dev/null 2>&1; then
    qs kill --any-display >/dev/null 2>&1 || true
  fi

  pkill -u "$user_name" -f 'noctalia-shell|share/noctalia-shell|share/caelestia-shell' >/dev/null 2>&1 || true
}

start_with_retry() {
  local name="$1"
  local pattern="$2"
  shift 2

  if is_running "$pattern"; then
    log "$name already running"
    return 0
  fi

  local attempt
  for attempt in 1 2 3 4 5; do
    log "starting $name attempt $attempt: $*"
    "$@" >>"$log_file" 2>&1 || true
    sleep 0.7

    if is_running "$pattern"; then
      log "$name started"
      return 0
    fi
  done

  log "failed to observe running $name after retries"
  return 1
}

log "requested profile=$profile"
wait_for_session

previous=""
if [ -f "$state_file" ]; then
  previous="$(cat "$state_file" 2>/dev/null || true)"
fi

if [ "$previous" != "$profile" ]; then
  stop_quickshells
  printf '%s\n' "$profile" > "$state_file"
  sleep 0.2
fi

case "$profile" in
  caelestia)
    if command -v caelestia >/dev/null 2>&1; then
      start_with_retry "caelestia" 'share/caelestia-shell|caelestia-shell' caelestia shell -d || true
    elif command -v caelestia-shell >/dev/null 2>&1; then
      start_with_retry "caelestia-shell" 'share/caelestia-shell|caelestia-shell' caelestia-shell -d || true
    else
      log "caelestia command not found"
    fi

    if command -v caelestia >/dev/null 2>&1; then
      caelestia resizer -d >>"$log_file" 2>&1 || true
    fi
    ;;

  noctalia)
    if command -v noctalia-shell >/dev/null 2>&1; then
      start_with_retry "noctalia" 'noctalia-shell|share/noctalia-shell' noctalia-shell -d || true
    else
      log "noctalia-shell command not found"
    fi
    ;;

  *)
    log "unknown shell profile: $profile"
    exit 1
    ;;

esac
