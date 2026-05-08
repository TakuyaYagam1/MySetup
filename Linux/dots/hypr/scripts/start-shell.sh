#!/usr/bin/env bash
set -uo pipefail

requested_profile="${1:-}"
config_home="${XDG_CONFIG_HOME:-$HOME/.config}"
runtime_dir="${XDG_RUNTIME_DIR:-/tmp}"
state_home="${XDG_STATE_HOME:-$HOME/.local/state}"
persistent_state_file="$state_home/mysetup/active-shell"
log_file="$runtime_dir/mysetup-shell.log"
lock_dir="$runtime_dir/mysetup-shell.lock"
lock_owner_file="$lock_dir/owner"
user_name="${USER:-}"
selector_pattern='qs([[:space:]].*)?-c[[:space:]]mysetup-shell-selector([[:space:]]|$)|quickshell/mysetup-shell-selector/shell\.qml'
end4_pattern='qs([[:space:]].*)?-c[[:space:]]ii([[:space:]]|$)|quickshell/ii/shell\.qml'

if [ -z "$user_name" ]; then
  user_name="$(id -un 2>/dev/null || printf '%s' user)"
fi

log() {
  printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*" >>"$log_file"
}

resolve_profile() {
  local requested="$1"
  local stored=""

  if [ -n "$requested" ]; then
    printf '%s' "$requested"
    return 0
  fi

  if [ -f "$persistent_state_file" ]; then
    stored="$(tr -d '[:space:]' <"$persistent_state_file" 2>/dev/null || true)"
  fi

  case "$stored" in
    caelestia|noctalia|end4)
      printf '%s' "$stored"
      ;;
    *)
      printf '%s' caelestia
      ;;
  esac
}

profile="$(resolve_profile "$requested_profile")"

case "$profile" in
  caelestia|noctalia|end4) ;;
  *)
    log "unknown shell before lock: $profile"
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
  printf '%s' "$config_home/hypr"
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

ensure_mysetup_entrypoint() {
  local dir target source_path

  dir="$(hypr_dir)"
  target="$dir/mysetup/hyprland.conf"
  source_path="$dir/hyprland.conf"

  if [ -f "$target" ]; then
    return 0
  fi

  mkdir -p -- "$(dirname "$target")"
  if [ -f "$source_path" ] && ! grep -qE 'source *= .*/(mysetup|end4)/hyprland\.conf' "$source_path"; then
    cp -- "$source_path" "$target"
    return 0
  fi

  log "mysetup hypr entrypoint missing: $target"
  return 1
}

sync_shell_launcher() {
  local dir

  dir="$(hypr_dir)"
  write_regular_file "$dir/shell-profile.conf" "# Runtime shell launcher
exec-once = $dir/scripts/start-shell.sh"
}

sync_shell_keybinds() {
  local dir profile_keybinds

  case "$profile" in
    end4)
      return 0
      ;;
  esac

  dir="$(hypr_dir)"
  profile_keybinds="$dir/$profile/keybinds.conf"

  if [ ! -f "$profile_keybinds" ]; then
    log "shell keybind profile missing: $profile_keybinds"
    return 1
  fi

  write_regular_file "$dir/shell-keybinds.conf" "# Active shell keybind profile: $profile
source = $profile_keybinds"
}

sync_hypr_entrypoint() {
  local dir target label

  dir="$(hypr_dir)"

  if [ "$profile" = "end4" ]; then
    target="$dir/end4/hyprland.conf"
    label="end4"
  else
    ensure_mysetup_entrypoint || return 1
    target="$dir/mysetup/hyprland.conf"
    label="mysetup ($profile)"
  fi

  if [ ! -f "$target" ]; then
    log "hypr entrypoint missing for profile=$profile path=$target"
    return 1
  fi

  write_regular_file "$dir/hyprland.conf" "# Active Hyprland profile: $label
source = $target"
}

sync_hypr_lock_stack() {
  local dir hyprlock_target hypridle_target

  dir="$(hypr_dir)"

  if [ "$profile" = "end4" ]; then
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

    write_regular_file "$dir/hyprlock.conf" "# Active Hyprlock profile: end4
source = $hyprlock_target" || return 1
    write_regular_file "$dir/hypridle.conf" "# Active Hypridle profile: end4
source = $hypridle_target"
    return $?
  fi

  write_regular_file "$dir/hyprlock.conf" "# Active Hyprlock profile: shell-managed ($profile)
# Caelestia and Noctalia use shell-native lock flows." || return 1
  write_regular_file "$dir/hypridle.conf" "# Active Hypridle profile: shell-managed ($profile)
# Caelestia and Noctalia use shell-native idle flows."
}

sync_runtime_shell_files() {
  sync_shell_launcher || return 1
  sync_shell_keybinds || return 1
  sync_hypr_entrypoint || return 1
  sync_hypr_lock_stack
}

persist_profile() {
  mkdir -p -- "$(dirname "$persistent_state_file")"
  write_regular_file "$persistent_state_file" "$profile"
}

reload_hypr() {
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

      log "waiting for start-shell switch lock; requested=$profile active=${lock_profile:-unknown} pid=$lock_pid"
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

stop_shell_selector() {
  log "stopping shell selector"
  pkill -u "$user_name" -f "$selector_pattern" >/dev/null 2>&1 || true
}

stop_end4() {
  log "stopping end4 shell"
  pkill -u "$user_name" -f "$end4_pattern" >/dev/null 2>&1 || true
}

stop_end4_idle() {
  log "stopping end4 hypridle"
  pkill -u "$user_name" -x hypridle >/dev/null 2>&1 || true
}

stop_quickshells() {
  log "stopping existing shell instances"

  stop_shell_selector
  stop_caelestia
  stop_noctalia
  stop_end4
}

stop_inactive_shells() {
  stop_shell_selector
  case "$1" in
    caelestia)
      stop_noctalia
      stop_end4
      ;;
    noctalia)
      stop_caelestia
      stop_end4
      ;;
    end4)
      stop_caelestia
      stop_noctalia
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

ensure_end4_idle() {
  local dir idle_config

  dir="$(hypr_dir)"
  idle_config="$dir/hypridle.conf"

  if [ ! -f "$idle_config" ]; then
    log "end4 hypridle config missing: $idle_config"
    return 1
  fi

  if command -v hypridle >/dev/null 2>&1; then
    start_with_retry "end4 hypridle" '(^|/)hypridle([[:space:]]|$)' hypridle || return 1
    return 0
  fi

  log "hypridle command not found"
  return 1
}

log "requested profile=$profile input=${requested_profile:-auto}"
wait_for_session

previous=""
if [ -f "$persistent_state_file" ]; then
  previous="$(tr -d '[:space:]' <"$persistent_state_file" 2>/dev/null || true)"
fi

if [ "$previous" != "$profile" ]; then
  stop_quickshells
  sleep 0.2
else
  stop_inactive_shells "$profile"
fi

if [ "$profile" != "end4" ]; then
  stop_end4_idle
fi

sync_runtime_shell_files || log "runtime shell file sync failed for profile=$profile"
persist_profile || log "failed to persist runtime shell state for profile=$profile"
reload_hypr

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

  end4)
    ensure_end4_idle || true
    dedupe_shell "end4" "$end4_pattern" stop_end4 || true

    if command -v qs >/dev/null 2>&1; then
      start_with_retry "end4" "$end4_pattern" qs -c ii || true
    else
      log "qs command not found"
    fi
    ;;
esac
