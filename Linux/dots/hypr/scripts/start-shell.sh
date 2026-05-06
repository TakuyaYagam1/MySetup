#!/usr/bin/env bash
set -uo pipefail

profile="${1:-caelestia}"
runtime_dir="${XDG_RUNTIME_DIR:-/tmp}"
state_file="$runtime_dir/mysetup-hypr-shell-profile"
log_file="$runtime_dir/mysetup-shell.log"
lock_dir="$runtime_dir/mysetup-shell.lock"
lock_owner_file="$lock_dir/owner"
user_name="${USER:-}"

if [ -z "$user_name" ]; then
  user_name="$(id -un 2>/dev/null || printf '%s' user)"
fi

log() {
  printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*" >>"$log_file"
}

case "$profile" in
  caelestia|noctalia) ;;
  *)
    log "unknown shell profile before lock: $profile"
    exit 1
    ;;
esac

pid_matches() {
  local pid="$1"
  local pattern="$2"

  [ -n "$pid" ] || return 1
  ps -p "$pid" -o args= 2>/dev/null | grep -qE "$pattern"
}

hypr_dir() {
  local script_dir

  script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd -P)"
  cd -- "$script_dir/.." >/dev/null 2>&1 && pwd -P
}

write_regular_file() {
  local path="$1"
  local content="$2"

  if [ -L "$path" ]; then
    rm -f -- "$path"
  elif [ -e "$path" ] && [ ! -f "$path" ]; then
    log "refusing to overwrite non-regular config file: $path"
    return 1
  fi

  chmod u+w "$path" >/dev/null 2>&1 || true
  printf '%s\n' "$content" >"$path"
}

sync_hypr_profile() {
  local dir profile_keybinds

  dir="$(hypr_dir)"
  profile_keybinds="$dir/$profile/keybinds.conf"

  if [ ! -f "$profile_keybinds" ]; then
    log "shell keybind profile missing: $profile_keybinds"
    return 1
  fi

  write_regular_file "$dir/shell-profile.conf" "# Active shell profile: $profile
exec-once = $dir/scripts/start-shell.sh $profile" || return 1
  write_regular_file "$dir/shell-keybinds.conf" "# Active shell keybind profile: $profile
source = $profile_keybinds" || return 1

  if command -v hyprctl >/dev/null 2>&1 && hyprctl monitors >/dev/null 2>&1; then
    hyprctl reload >/dev/null 2>&1 || log "hyprctl reload failed after profile sync"
  fi
}

acquire_lock() {
  local attempt lock_owner lock_pid lock_profile

  for attempt in $(seq 1 80); do
    if mkdir "$lock_dir" 2>/dev/null; then
      printf '%s\n' "$$" >"$lock_dir/pid"
      printf '%s\n' "$profile" >"$lock_dir/profile"
      printf '%s\n' "mysetup-start-shell" >"$lock_owner_file"
      return 0
    fi

    lock_owner="$(cat "$lock_owner_file" 2>/dev/null || true)"
    lock_pid="$(cat "$lock_dir/pid" 2>/dev/null || true)"
    lock_profile="$(cat "$lock_dir/profile" 2>/dev/null || true)"
    if [ "$lock_owner" = "mysetup-start-shell" ] && pid_matches "$lock_pid" '(^|[ /])start-shell\.sh([[:space:]]|$)'; then
      if [ "$lock_profile" = "$profile" ]; then
        log "another start-shell instance is already running for profile=$profile pid=$lock_pid"
        exit 0
      fi

      log "waiting for start-shell profile switch lock; requested=$profile active=${lock_profile:-unknown} pid=$lock_pid"
      sleep 0.25
      continue
    fi

    log "removing stale start-shell lock; profile=$profile pid=${lock_pid:-unknown}"
    rm -rf "$lock_dir"
  done

  log "failed to acquire start-shell lock; profile=$profile"
  exit 1
}

acquire_lock
trap 'rm -rf "$lock_dir" 2>/dev/null || true' EXIT

is_running() {
  pgrep -u "$user_name" -f "$1" >/dev/null 2>&1
}

running_count() {
  pgrep -u "$user_name" -f "$1" 2>/dev/null | wc -l | tr -d '[:space:]'
}

dedupe_shell() {
  local name="$1"
  local pattern="$2"
  local stop_func="$3"
  local count

  count="$(running_count "$pattern")"
  if [ "${count:-0}" -le 1 ]; then
    return 0
  fi

  log "found duplicate $name instances count=$count; restarting requested profile"
  "$stop_func"
  sleep 0.2
  return 1
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

stop_caelestia() {
  log "stopping caelestia shell"

  if command -v caelestia >/dev/null 2>&1; then
    caelestia shell -k >/dev/null 2>&1 || true
  fi

  pkill -u "$user_name" -f 'share/caelestia-shell|caelestia-shell' >/dev/null 2>&1 || true
  pkill -u "$user_name" -f 'caelestia resizer' >/dev/null 2>&1 || true
}

stop_noctalia() {
  log "stopping noctalia shell"
  pkill -u "$user_name" -f 'noctalia-shell|share/noctalia-shell' >/dev/null 2>&1 || true
}

stop_quickshells() {
  log "stopping existing quickshell instances"

  stop_caelestia
  stop_noctalia

  if command -v qs >/dev/null 2>&1; then
    qs kill --any-display >/dev/null 2>&1 || true
  fi
}

stop_inactive_shells() {
  case "$1" in
    caelestia)
      stop_noctalia
      ;;
    noctalia)
      stop_caelestia
      ;;
  esac
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
    ("$@" >>"$log_file" 2>&1 &)
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
sync_hypr_profile || true

previous=""
if [ -f "$state_file" ]; then
  previous="$(cat "$state_file" 2>/dev/null || true)"
fi

if [ "$previous" != "$profile" ]; then
  stop_quickshells
  sleep 0.2
else
  stop_inactive_shells "$profile"
fi
printf '%s\n' "$profile" > "$state_file"

case "$profile" in
  caelestia)
    dedupe_shell "caelestia" 'share/caelestia-shell|caelestia-shell' stop_caelestia || true

    if command -v caelestia >/dev/null 2>&1; then
      start_with_retry "caelestia" 'share/caelestia-shell|caelestia-shell' caelestia shell -d || true
    elif command -v caelestia-shell >/dev/null 2>&1; then
      start_with_retry "caelestia-shell" 'share/caelestia-shell|caelestia-shell' caelestia-shell -d || true
    else
      log "caelestia command not found"
    fi

    if command -v caelestia >/dev/null 2>&1 && ! is_running 'caelestia resizer'; then
      (caelestia resizer -d >>"$log_file" 2>&1 &)
    fi
    ;;

  noctalia)
    dedupe_shell "noctalia" 'noctalia-shell|share/noctalia-shell' stop_noctalia || true

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
