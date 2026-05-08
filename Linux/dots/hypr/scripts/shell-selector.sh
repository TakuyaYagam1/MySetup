#!/usr/bin/env bash
set -euo pipefail

action="${1:-toggle}"
requested_profile="${2:-}"

config_home="${XDG_CONFIG_HOME:-$HOME/.config}"
runtime_dir="${XDG_RUNTIME_DIR:-/tmp}"
state_home="${XDG_STATE_HOME:-$HOME/.local/state}"
state_dir="$runtime_dir/mysetup-shell-selector"
lock_dir="$state_dir/lock"
lock_pid_file="$lock_dir/pid"
lock_owner_file="$lock_dir/owner"
selector_name="mysetup-shell-selector"
selector_pattern='qs([[:space:]].*)?-c[[:space:]]mysetup-shell-selector([[:space:]]|$)|quickshell/mysetup-shell-selector/shell\.qml'
end4_pattern='qs([[:space:]].*)?-c[[:space:]]ii([[:space:]]|$)|quickshell/ii/shell\.qml'
noctalia_pattern='noctalia-shell|share/noctalia-shell'
caelestia_pattern='share/caelestia-shell|caelestia-shell|(^|[ /])caelestia([[:space:]]+shell|[[:space:]]|$)'
active_shell_state="$state_home/mysetup/active-shell"
start_shell_script="$config_home/hypr/scripts/start-shell.sh"

mkdir -p "$state_dir"

pid_matches() {
  local pid="$1"
  local pattern="$2"

  [ -n "$pid" ] || return 1
  ps -p "$pid" -o args= 2>/dev/null | grep -qE "$pattern"
}

lock_owner_running() {
  local owner_pid="$1"
  local owner_name

  owner_name="$(cat "$lock_owner_file" 2>/dev/null || true)"
  [ "$owner_name" = "mysetup-shell-selector" ] || return 1
  pid_matches "$owner_pid" '(^|[ /])shell-selector\.sh([[:space:]]|$)'
}

acquire_lock() {
  local owner_pid

  for _ in $(seq 1 20); do
    if mkdir "$lock_dir" 2>/dev/null; then
      printf '%s\n' "$$" >"$lock_pid_file"
      printf '%s\n' "mysetup-shell-selector" >"$lock_owner_file"
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

read_stored_active_shell() {
  local stored=""

  if [ -f "$active_shell_state" ]; then
    stored="$(tr -d '[:space:]' <"$active_shell_state" 2>/dev/null || true)"
  fi

  case "$stored" in
    caelestia|noctalia|end4)
      printf '%s' "$stored"
      ;;
    *)
      return 1
      ;;
  esac
}

detect_shell_from_processes() {
  if pgrep -u "${USER:-$(id -un)}" -f "$end4_pattern" >/dev/null 2>&1; then
    printf '%s' end4
    return 0
  fi

  if pgrep -u "${USER:-$(id -un)}" -f "$noctalia_pattern" >/dev/null 2>&1; then
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
  local keybinds_path="$config_home/hypr/shell-keybinds.conf"

  if [ ! -r "$keybinds_path" ]; then
    return 1
  fi

  if grep -qE 'noctalia/keybinds\.conf|noctalia-shell ipc call|noctalia-launcher\.sh' "$keybinds_path"; then
    printf '%s' noctalia
    return 0
  fi

  if grep -qE 'caelestia/keybinds\.conf|caelestia:launcher' "$keybinds_path"; then
    printf '%s' caelestia
    return 0
  fi

  return 1
}

detect_shell_from_entrypoint() {
  local entrypoint_path="$config_home/hypr/hyprland.conf"

  if [ ! -r "$entrypoint_path" ]; then
    return 1
  fi

  if grep -q 'end4/hyprland.conf' "$entrypoint_path"; then
    printf '%s' end4
    return 0
  fi

  if grep -q 'mysetup/hyprland.conf' "$entrypoint_path"; then
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
  local attempt

  for attempt in $(seq 1 30); do
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
    exit 1
  fi

  monitor="${requested_profile:-$(detect_focused_monitor)}"
  active_shell="$(detect_active_shell)"

  env \
    MYSETUP_SHELL_SELECTOR_MONITOR="$monitor" \
    MYSETUP_ACTIVE_SHELL="$active_shell" \
    qs -c "$selector_name" >/dev/null 2>&1 &

  wait_for_selector_spawn || true
}

switch_shell() {
  local profile="$1"

  case "$profile" in
    caelestia|noctalia|end4) ;;
    *)
      exit 1
      ;;
  esac

  "$start_shell_script" "$profile" >/dev/null 2>&1 &
  stop_selector
}

acquire_lock
trap 'rm -rf -- "$lock_dir" 2>/dev/null || true' EXIT

case "$action" in
  toggle)
    if selector_running; then
      stop_selector
      exit 0
    fi
    start_selector
    ;;
  switch)
    switch_shell "$requested_profile"
    ;;
  close)
    stop_selector
    ;;
  *)
    exit 1
    ;;
esac
