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
mysetup_noctalia_v4_pattern='(^|[ /])noctalia-shell([[:space:]]|$)|share/noctalia-shell'
mysetup_caelestia_pattern='share/caelestia-shell|caelestia-shell|(^|[ /])caelestia[[:space:]]+shell([[:space:]]|$)'
mysetup_noctalia_v4_env_pattern='^QS_CONFIG_PATH=.*/share/noctalia-shell$'
mysetup_end4_env_pattern='^qsConfig=.*/quickshell/ii$|^ILLOGICAL_IMPULSE_DOTFILES_SOURCE='

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
    caelestia | noctalia | end4) return 0 ;;
    *) return 1 ;;
  esac
}

mysetup_noctalia_command() {
  if command -v noctalia >/dev/null 2>&1; then
    printf '%s' noctalia
    return 0
  fi

  if command -v noctalia-shell >/dev/null 2>&1; then
    printf '%s' noctalia-shell
    return 0
  fi

  return 1
}

mysetup_noctalia_daemon_flag() {
  local command_name

  command_name="$(mysetup_noctalia_command 2>/dev/null || true)"
  case "${command_name##*/}" in
    noctalia-shell)
      printf '%s' --daemonize
      ;;
    *)
      printf '%s' --daemon
      ;;
  esac
}

mysetup_noctalia_msg() {
  local command_name

  command_name="$(mysetup_noctalia_command 2>/dev/null || true)"
  [ -n "$command_name" ] || return 1
  "$command_name" msg "$@"
}

mysetup_noctalia_is_v4() {
  local command_name

  command_name="$(mysetup_noctalia_command 2>/dev/null || true)"
  [ "${command_name##*/}" = "noctalia-shell" ]
}

# v4 (noctalia-shell) uses `msg <target> <function>`; v5 (noctalia) uses a
# flat hyphenated `msg <command>` vocabulary. Translate stable semantic
# action names to whichever syntax the running binary understands.
mysetup_noctalia_action() {
  local action="$1"

  case "$action" in
    launcher-toggle)
      if mysetup_noctalia_is_v4; then
        mysetup_noctalia_msg launcher toggle
      else
        mysetup_noctalia_msg panel-toggle launcher
      fi
      ;;
    session-menu-toggle)
      if mysetup_noctalia_is_v4; then
        mysetup_noctalia_msg sessionMenu toggle
      else
        mysetup_noctalia_msg panel-toggle session
      fi
      ;;
    control-center-toggle)
      if mysetup_noctalia_is_v4; then
        mysetup_noctalia_msg controlCenter toggle
      else
        mysetup_noctalia_msg panel-toggle control-center
      fi
      ;;
    notifications-clear)
      if mysetup_noctalia_is_v4; then
        mysetup_noctalia_msg notifications dismissAll
      else
        mysetup_noctalia_msg notification-clear-active
      fi
      ;;
    settings-toggle)
      if mysetup_noctalia_is_v4; then
        mysetup_noctalia_msg settings toggle
      else
        mysetup_noctalia_msg settings-toggle
      fi
      ;;
    lock)
      if mysetup_noctalia_is_v4; then
        mysetup_noctalia_msg lockScreen lock
      else
        mysetup_noctalia_msg session lock
      fi
      ;;
    brightness-up)
      if mysetup_noctalia_is_v4; then
        mysetup_noctalia_msg brightness increase
      else
        mysetup_noctalia_msg brightness-up
      fi
      ;;
    brightness-down)
      if mysetup_noctalia_is_v4; then
        mysetup_noctalia_msg brightness decrease
      else
        mysetup_noctalia_msg brightness-down
      fi
      ;;
    clipboard-toggle)
      if mysetup_noctalia_is_v4; then
        mysetup_noctalia_msg launcher clipboard
      else
        mysetup_noctalia_msg panel-toggle clipboard
      fi
      ;;
    emoji-toggle)
      if mysetup_noctalia_is_v4; then
        mysetup_noctalia_msg launcher emoji
      else
        mysetup_noctalia_msg panel-toggle launcher /emo
      fi
      ;;
    *)
      shift
      mysetup_noctalia_msg "$action" "$@"
      ;;
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
  local env_file

  [ -n "$pid" ] || return 1
  env_file="/proc/$pid/environ"
  [ -r "$env_file" ] || return 1
  { tr '\0' '\n' <"$env_file"; } 2>/dev/null | grep -qE "$regex"
}

mysetup_quickshell_pids() {
  pgrep -u "$mysetup_user_name" -f '(^|[ /])\.?quickshell(-wrapped)?([[:space:]]|$)' 2>/dev/null || true
}

mysetup_noctalia_pids() {
  local pid

  {
    pgrep -u "$mysetup_user_name" -x noctalia 2>/dev/null || true
    pgrep -u "$mysetup_user_name" -f '(^|/)noctalia([[:space:]]|$)' 2>/dev/null || true
    pgrep -u "$mysetup_user_name" -f "$mysetup_noctalia_v4_pattern" 2>/dev/null || true
    for pid in $(mysetup_quickshell_pids); do
      if mysetup_pid_has_env_regex "$pid" "$mysetup_noctalia_v4_env_pattern"; then
        printf '%s\n' "$pid"
      fi
    done
  } | sort -u
}

mysetup_noctalia_running() {
  mysetup_noctalia_pids | grep -q .
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
