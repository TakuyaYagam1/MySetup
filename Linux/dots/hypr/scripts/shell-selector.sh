#!/usr/bin/env bash
set -euo pipefail

script_dir="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=Linux/dots/hypr/scripts/shell-runtime.sh
. "$script_dir/shell-runtime.sh"

action="${1:-toggle}"
action_arg="${2:-}"
requested_profile=""
selector_monitor_override=""

case "$action" in
  switch)
    requested_profile="$action_arg"
    ;;
  toggle)
    selector_monitor_override="$action_arg"
    ;;
esac

config_home="$mysetup_config_home"
runtime_dir="$mysetup_runtime_session_dir"
state_dir="$runtime_dir/mysetup-shell-selector"
log_file="$mysetup_log_file"
lock_dir="$state_dir/lock"
lock_pid_file="$lock_dir/pid"
lock_owner_file="$lock_dir/owner"
selector_name="mysetup-shell-selector"
selector_pattern="$mysetup_selector_pattern"
end4_pattern="$mysetup_end4_pattern"
noctalia_pattern="$mysetup_noctalia_pattern"
caelestia_pattern="$mysetup_caelestia_pattern"
noctalia_env_pattern="$mysetup_noctalia_env_pattern"
active_shell_state="$mysetup_active_shell_state"
start_shell_script="$config_home/hypr/scripts/start-shell.sh"

mkdir -p "$state_dir"

log() {
  printf '[%s] [shell-selector] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*" >>"$log_file"
}

log "invoked action=${action:-empty} requested_profile=${requested_profile:-empty} monitor_override=${selector_monitor_override:-empty} pid=$$"

noctalia_running() {
  local pid

  if pgrep -u "${USER:-$(id -un)}" -f "$noctalia_pattern" >/dev/null 2>&1; then
    return 0
  fi

  for pid in $(mysetup_quickshell_pids); do
    if mysetup_pid_has_env_regex "$pid" "$noctalia_env_pattern"; then
      return 0
    fi
  done

  return 1
}

acquire_lock() {
  mysetup_acquire_lock \
    "$lock_dir" \
    "$lock_pid_file" \
    "$lock_owner_file" \
    "mysetup-shell-selector" \
    '(^|[ /])shell-selector\.sh([[:space:]]|$)' \
    20 \
    0.02 || exit 0
}

read_stored_active_shell() {
  mysetup_read_active_shell "$active_shell_state"
}

detect_shell_from_processes() {
  if pgrep -u "${USER:-$(id -un)}" -f "$end4_pattern" >/dev/null 2>&1; then
    printf '%s' end4
    return 0
  fi

  if noctalia_running; then
    printf '%s' noctalia
    return 0
  fi

  if pgrep -u "${USER:-$(id -un)}" -f "$caelestia_pattern" >/dev/null 2>&1; then
    printf '%s' caelestia
    return 0
  fi

  return 1
}

detect_shell_from_keybinds() {
  local keybinds_path="$mysetup_hypr_runtime_dir/shell-keybinds.lua"

  if [ ! -r "$keybinds_path" ]; then
    keybinds_path="$config_home/hypr/shell-keybinds.lua"
    [ -r "$keybinds_path" ] || return 1
  fi

  if grep -qE 'noctalia[/.]keybinds|noctalia-shell ipc call|noctalia-launcher\.sh' "$keybinds_path"; then
    printf '%s' noctalia
    return 0
  fi

  if grep -qE 'caelestia[/.]keybinds|caelestia:launcher' "$keybinds_path"; then
    printf '%s' caelestia
    return 0
  fi

  return 1
}

detect_shell_from_entrypoint() {
  local entrypoint_path="$mysetup_hypr_runtime_dir/hyprland.lua"

  if [ ! -r "$entrypoint_path" ]; then
    entrypoint_path="$config_home/hypr/hyprland.lua"
    [ -r "$entrypoint_path" ] || return 1
  fi

  if grep -q 'end4/hyprland.lua' "$entrypoint_path"; then
    printf '%s' end4
    return 0
  fi

  if grep -q 'mysetup/hyprland.lua' "$entrypoint_path"; then
    detect_shell_from_keybinds
    return $?
  fi

  return 1
}

detect_active_shell() {
  if detect_shell_from_processes; then
    return 0
  fi

  if detect_shell_from_entrypoint; then
    return 0
  fi

  if read_stored_active_shell; then
    return 0
  fi

  printf '%s' caelestia
}

detect_focused_monitor() {
  if ! command -v hyprctl >/dev/null 2>&1 || ! command -v jq >/dev/null 2>&1; then
    return 0
  fi

  hyprctl monitors -j 2>/dev/null | jq -r '.[] | select(.focused == true) | .name' | head -n 1
}

selector_running() {
  pgrep -u "${USER:-$(id -un)}" -f "$selector_pattern" >/dev/null 2>&1
}

stop_selector() {
  pkill -u "${USER:-$(id -un)}" -f "$selector_pattern" >/dev/null 2>&1 || true
}

wait_for_selector_spawn() {
  local _

  for _ in $(seq 1 30); do
    if selector_running; then
      return 0
    fi
    sleep 0.05
  done

  return 1
}

start_selector() {
  local monitor active_shell

  if ! command -v qs >/dev/null 2>&1; then
    log "qs command not found; selector cannot start"
    exit 1
  fi

  monitor="${selector_monitor_override:-$(detect_focused_monitor)}"
  active_shell="$(detect_active_shell)"
  log "starting selector monitor=${monitor:-auto} active=${active_shell:-unknown}"

  env \
    MYSETUP_SHELL_SELECTOR_MONITOR="$monitor" \
    MYSETUP_ACTIVE_SHELL="$active_shell" \
    MYSETUP_SHELL_SELECTOR_SCRIPT="$config_home/hypr/scripts/shell-selector.sh" \
    qs -c "$selector_name" >/dev/null 2>&1 &

  wait_for_selector_spawn || true
}

switch_shell() {
  local profile="$1"

  if ! mysetup_valid_shell_profile "$profile"; then
    log "rejecting invalid profile switch request: ${profile:-empty}"
    exit 1
  fi

  if [ ! -f "$start_shell_script" ]; then
    log "start-shell script missing: $start_shell_script"
    exit 1
  fi

  log "dispatching shell switch profile=$profile script=$start_shell_script"
  bash "$start_shell_script" "$profile" >>"$log_file" 2>&1 &
  stop_selector
}

case "$action" in
  switch)
    log "switch requested profile=${requested_profile:-empty}"
    switch_shell "$requested_profile"
    exit 0
    ;;
esac

acquire_lock
trap 'rm -rf -- "$lock_dir" 2>/dev/null || true' EXIT

case "$action" in
  toggle)
    log "toggle requested"
    if selector_running; then
      log "selector already running; closing existing instance"
      stop_selector
      exit 0
    fi
    start_selector
    ;;
  close)
    log "close requested"
    stop_selector
    ;;
  *)
    log "unknown action requested: ${action:-empty}"
    exit 1
    ;;
esac
