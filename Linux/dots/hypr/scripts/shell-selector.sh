#!/usr/bin/env bash
set -euo pipefail

script_dir="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=Linux/dots/hypr/scripts/shell-runtime.sh
. "$script_dir/shell-runtime.sh"

wahrwelt_enter_runtime_lock_v2 \
  wahrwelt-shell-selector-v2.lock 400 0 "$0" "$@"

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

config_home="$wahrwelt_config_home"
if ! wahrwelt_adopt_legacy_private_state_directory wahrwelt-shell-selector shell-selector-state; then
  printf 'Wahrwelt selector ownership collision: %s/wahrwelt-shell-selector\n' \
    "$wahrwelt_runtime_session_public_dir" >&2
  exit 1
fi
if ! wahrwelt_open_private_state_directory wahrwelt-shell-selector shell-selector-state; then
  printf 'Wahrwelt selector ownership collision: %s/wahrwelt-shell-selector\n' \
    "$wahrwelt_runtime_session_public_dir" >&2
  exit 1
fi
log_file="$wahrwelt_log_file"
selector_name="wahrwelt-shell-selector"
selector_pattern="$wahrwelt_selector_pattern"
end4_official_env_pattern="$wahrwelt_end4_official_env_pattern"
end4_pc_env_pattern="$wahrwelt_end4_pc_env_pattern"
caelestia_pattern="$wahrwelt_caelestia_pattern"
active_shell_state="$wahrwelt_active_shell_state"
start_shell_script="$config_home/hypr/scripts/start-shell.sh"

log() {
  printf '[%s] [shell-selector] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*" >>"$log_file"
}

log "invoked action=${action:-empty} requested_profile=${requested_profile:-empty} monitor_override=${selector_monitor_override:-empty} pid=$$"

noctalia_running() {
  wahrwelt_noctalia_running
}

read_stored_active_shell() {
  wahrwelt_read_active_shell "$active_shell_state"
}

detect_shell_from_processes() {
  local pid

  for pid in $(wahrwelt_quickshell_pids); do
    if wahrwelt_pid_has_env_regex "$pid" "$end4_pc_env_pattern"; then
      printf '%s' end4-pc
      return 0
    fi
  done

  for pid in $(wahrwelt_quickshell_pids); do
    if wahrwelt_pid_has_env_regex "$pid" "$end4_official_env_pattern"; then
      printf '%s' end4
      return 0
    fi
  done

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
  local keybinds_path="$wahrwelt_hypr_runtime_dir/shell-keybinds.lua"

  if [ ! -r "$keybinds_path" ]; then
    keybinds_path="$config_home/hypr/shell-keybinds.lua"
    [ -r "$keybinds_path" ] || return 1
  fi

  wahrwelt_detect_shell_adapter "$keybinds_path"
}

detect_shell_from_entrypoint() {
  local entrypoint_path="$wahrwelt_hypr_runtime_dir/hyprland.lua"

  if [ ! -r "$entrypoint_path" ]; then
    entrypoint_path="$config_home/hypr/hyprland.lua"
    [ -r "$entrypoint_path" ] || return 1
  fi

  if wahrwelt_is_canonical_entrypoint "$entrypoint_path"; then
    detect_shell_from_keybinds
    return $?
  fi

  if wahrwelt_is_legacy_user_entrypoint "$entrypoint_path"; then
    detect_shell_from_keybinds
    return $?
  fi

  if wahrwelt_is_legacy_direct_end4_entrypoint "$entrypoint_path" "$config_home"; then
    wahrwelt_read_end4_variant
    return 0
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
  local monitor active_shell remembered_end4_variant

  if ! command -v qs >/dev/null 2>&1; then
    log "qs command not found; selector cannot start"
    exit 1
  fi

  monitor="${selector_monitor_override:-$(detect_focused_monitor)}"
  active_shell="$(detect_active_shell)"
  remembered_end4_variant="$(wahrwelt_read_end4_variant)"
  log "starting selector monitor=${monitor:-auto} active=${active_shell:-unknown} end4_variant=$remembered_end4_variant"

  env \
    WAHRWELT_SHELL_SELECTOR_MONITOR="$monitor" \
    WAHRWELT_ACTIVE_SHELL="$active_shell" \
    WAHRWELT_END4_VARIANT="$remembered_end4_variant" \
    WAHRWELT_SHELL_SELECTOR_SCRIPT="$config_home/hypr/scripts/shell-selector.sh" \
    qs -c "$selector_name" >/dev/null 2>&1 &

  wait_for_selector_spawn || true
}

switch_shell() {
  local profile="$1"

  if ! wahrwelt_valid_shell_profile "$profile"; then
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
