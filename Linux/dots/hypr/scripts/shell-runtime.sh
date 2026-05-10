#!/usr/bin/env bash
# shellcheck disable=SC2034

mysetup_config_home="${XDG_CONFIG_HOME:-$HOME/.config}"
mysetup_runtime_session_dir="${XDG_RUNTIME_DIR:-}"
if [ -z "$mysetup_runtime_session_dir" ]; then
  mysetup_user_id="${UID:-$(id -u 2>/dev/null || printf '%s' 0)}"
  mysetup_runtime_session_dir="${TMPDIR:-/tmp}/mysetup-runtime-$mysetup_user_id"
  mkdir -p "$mysetup_runtime_session_dir"
  chmod 700 "$mysetup_runtime_session_dir" 2>/dev/null || true
fi
mysetup_state_home="${XDG_STATE_HOME:-$HOME/.local/state}"
mysetup_hypr_dir="$mysetup_config_home/hypr"
mysetup_state_dir="$mysetup_state_home/mysetup"
mysetup_hypr_runtime_dir="$mysetup_state_dir/hypr-runtime"
mysetup_active_shell_state="$mysetup_state_dir/active-shell"
mysetup_log_file="$mysetup_runtime_session_dir/mysetup-shell.log"
mysetup_default_shell_profile="caelestia"

mysetup_selector_pattern='((^|[ /])(qs|quickshell)([[:space:]].*)?-c[[:space:]]mysetup-shell-selector([[:space:]]|$))|quickshell/mysetup-shell-selector([/[:space:]]|$)'
mysetup_end4_pattern='((^|[ /])(qs-end4|qs|quickshell)([[:space:]].*)?-c[[:space:]]ii([[:space:]]|$))|quickshell/ii([/[:space:]]|$)'
mysetup_noctalia_pattern='noctalia-shell|share/noctalia-shell'
mysetup_caelestia_pattern='share/caelestia-shell|caelestia-shell|(^|[ /])caelestia[[:space:]]+shell([[:space:]]|$)'
mysetup_noctalia_env_pattern='^QS_CONFIG_PATH=.*/share/noctalia-shell$'

mysetup_user_name="${USER:-}"
if [ -z "$mysetup_user_name" ]; then
  mysetup_user_name="$(id -un 2>/dev/null || printf '%s' user)"
fi

mysetup_hypr_dir_path() {
  printf '%s' "$mysetup_hypr_dir"
}

mysetup_runtime_file() {
  printf '%s/%s' "$mysetup_hypr_runtime_dir" "$1"
}

mysetup_valid_shell_profile() {
  case "$1" in
    caelestia|noctalia|end4) return 0 ;;
    *) return 1 ;;
  esac
}

mysetup_pid_matches() {
  local pid="$1"
  local pattern="$2"

  [ -n "$pid" ] || return 1
  ps -p "$pid" -o args= 2>/dev/null | grep -qE "$pattern"
}

mysetup_pid_has_env_regex() {
  local pid="$1"
  local regex="$2"

  [ -n "$pid" ] || return 1
  tr '\0' '\n' <"/proc/$pid/environ" 2>/dev/null | grep -qE "$regex"
}

mysetup_quickshell_pids() {
  pgrep -u "$mysetup_user_name" -f '(^|[ /])quickshell([[:space:]]|$)' 2>/dev/null || true
}

mysetup_read_active_shell() {
  local path="${1:-$mysetup_active_shell_state}"
  local stored=""

  if [ -f "$path" ]; then
    stored="$(tr -d '[:space:]' <"$path" 2>/dev/null || true)"
  fi

  if mysetup_valid_shell_profile "$stored"; then
      printf '%s' "$stored"
      return 0
  fi

  return 1
}

mysetup_lock_owner_running() {
  local owner_pid="$1"
  local owner_file="$2"
  local owner_name="$3"
  local owner_pattern="$4"
  local active_owner

  active_owner="$(cat "$owner_file" 2>/dev/null || true)"
  [ "$active_owner" = "$owner_name" ] || return 1
  mysetup_pid_matches "$owner_pid" "$owner_pattern"
}

mysetup_acquire_lock() {
  local lock_dir="$1"
  local pid_file="$2"
  local owner_file="$3"
  local owner_name="$4"
  local owner_pattern="$5"
  local attempts="${6:-20}"
  local delay="${7:-0.02}"
  local owner_pid

  for _ in $(seq 1 "$attempts"); do
    if mkdir "$lock_dir" 2>/dev/null; then
      printf '%s\n' "$$" >"$pid_file"
      printf '%s\n' "$owner_name" >"$owner_file"
      return 0
    fi

    owner_pid="$(cat "$pid_file" 2>/dev/null || true)"
    if mysetup_lock_owner_running "$owner_pid" "$owner_file" "$owner_name" "$owner_pattern"; then
      sleep "$delay"
      continue
    fi

    rm -rf -- "$lock_dir"
  done

  return 1
}
