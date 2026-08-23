#!/usr/bin/env bash
# shellcheck disable=SC2034

wahrwelt_config_home="${XDG_CONFIG_HOME:-$HOME/.config}"
wahrwelt_runtime_session_dir="${XDG_RUNTIME_DIR:-}"
if [ -z "$wahrwelt_runtime_session_dir" ]; then
  wahrwelt_user_id="${UID:-$(id -u 2>/dev/null || printf '%s' 0)}"
  wahrwelt_runtime_session_dir="${TMPDIR:-/tmp}/wahrwelt-runtime-$wahrwelt_user_id"
  mkdir -p "$wahrwelt_runtime_session_dir"
  chmod 700 "$wahrwelt_runtime_session_dir" 2>/dev/null || true
fi
wahrwelt_state_home="${XDG_STATE_HOME:-$HOME/.local/state}"
wahrwelt_hypr_dir="$wahrwelt_config_home/hypr"
wahrwelt_state_dir="$wahrwelt_state_home/wahrwelt"
wahrwelt_hypr_runtime_dir="$wahrwelt_state_dir/hypr-runtime"
wahrwelt_active_shell_state="$wahrwelt_state_dir/active-shell"
wahrwelt_end4_variant_state="$wahrwelt_state_dir/end4-variant"
wahrwelt_log_file="$wahrwelt_runtime_session_dir/wahrwelt-shell.log"
wahrwelt_default_shell_profile="caelestia"

wahrwelt_selector_pattern='((^|[ /])(qs|quickshell)([[:space:]].*)?-c[[:space:]]wahrwelt-shell-selector([[:space:]]|$))|quickshell/wahrwelt-shell-selector([/[:space:]]|$)'
wahrwelt_end4_official_pattern='((^|[ /])(qs-end4|qs|quickshell)([[:space:]].*)?-c[[:space:]]ii([[:space:]]|$))|quickshell/ii([/[:space:]]|$)'
wahrwelt_end4_pc_pattern='((^|[ /])(qs-end4|qs|quickshell)([[:space:]].*)?-c[[:space:]]end4-pC([[:space:]]|$))|quickshell/end4-pC([/[:space:]]|$)'
wahrwelt_end4_pattern="($wahrwelt_end4_official_pattern)|($wahrwelt_end4_pc_pattern)"
wahrwelt_noctalia_v4_pattern='(^|[ /])noctalia-shell([[:space:]]|$)|share/noctalia-shell'
wahrwelt_caelestia_pattern='share/caelestia-shell|caelestia-shell|(^|[ /])caelestia[[:space:]]+shell([[:space:]]|$)'
wahrwelt_noctalia_v4_env_pattern='^QS_CONFIG_PATH=.*/share/noctalia-shell$'
wahrwelt_end4_official_env_pattern='^qsConfig=.*/quickshell/ii$'
wahrwelt_end4_pc_env_pattern='^qsConfig=.*/quickshell/end4-pC$'
wahrwelt_end4_env_pattern="$wahrwelt_end4_official_env_pattern|$wahrwelt_end4_pc_env_pattern|^ILLOGICAL_IMPULSE_DOTFILES_SOURCE="

wahrwelt_user_name="${USER:-}"
if [ -z "$wahrwelt_user_name" ]; then
  wahrwelt_user_name="$(id -un 2>/dev/null || printf '%s' user)"
fi

wahrwelt_hypr_dir_path() {
  printf '%s' "$wahrwelt_hypr_dir"
}

wahrwelt_runtime_file() {
  printf '%s/%s' "$wahrwelt_hypr_runtime_dir" "$1"
}

wahrwelt_valid_shell_profile() {
  case "$1" in
    caelestia | noctalia | end4 | end4-pc) return 0 ;;
    *) return 1 ;;
  esac
}

wahrwelt_valid_end4_variant() {
  case "$1" in
    end4 | end4-pc) return 0 ;;
    *) return 1 ;;
  esac
}

wahrwelt_shell_family() {
  case "$1" in
    end4 | end4-pc) printf '%s' end4 ;;
    *) printf '%s' "$1" ;;
  esac
}

wahrwelt_end4_quickshell_config() {
  case "$1" in
    end4) printf '%s' ii ;;
    end4-pc) printf '%s' end4-pC ;;
    *) return 1 ;;
  esac
}

wahrwelt_end4_quickshell_path() {
  local qs_config

  qs_config="$(wahrwelt_end4_quickshell_config "$1")" || return 1
  printf '%s/quickshell/%s' "$wahrwelt_config_home" "$qs_config"
}

wahrwelt_export_end4_quickshell_config() {
  local qs_path

  qs_path="$(wahrwelt_end4_quickshell_path "$1")" || return 1
  export WAHRWELT_QS_CONFIG="$qs_path"
  export qsConfig="$qs_path"
}

wahrwelt_read_end4_variant() {
  local path="${1:-$wahrwelt_end4_variant_state}"
  local stored=""

  if [ -f "$path" ]; then
    IFS= read -r stored <"$path" || stored=""
    if ! printf '%s\n' "$stored" | cmp -s - "$path"; then
      stored=""
    fi
  fi

  if wahrwelt_valid_end4_variant "$stored"; then
    printf '%s' "$stored"
  else
    printf '%s' end4
  fi
}

wahrwelt_noctalia_command() {
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

wahrwelt_noctalia_daemon_flag() {
  local command_name

  command_name="$(wahrwelt_noctalia_command 2>/dev/null || true)"
  case "${command_name##*/}" in
    noctalia-shell)
      printf '%s' --daemonize
      ;;
    *)
      printf '%s' --daemon
      ;;
  esac
}

wahrwelt_noctalia_msg() {
  local command_name

  command_name="$(wahrwelt_noctalia_command 2>/dev/null || true)"
  [ -n "$command_name" ] || return 1
  "$command_name" msg "$@"
}

wahrwelt_noctalia_is_v4() {
  local command_name

  command_name="$(wahrwelt_noctalia_command 2>/dev/null || true)"
  [ "${command_name##*/}" = "noctalia-shell" ]
}

# v4 (noctalia-shell) uses `msg <target> <function>`; v5 (noctalia) uses a
# flat hyphenated `msg <command>` vocabulary. Translate stable semantic
# action names to whichever syntax the running binary understands.
wahrwelt_noctalia_action() {
  local action="$1"

  case "$action" in
    launcher-toggle)
      if wahrwelt_noctalia_is_v4; then
        wahrwelt_noctalia_msg launcher toggle
      else
        wahrwelt_noctalia_msg panel-toggle launcher
      fi
      ;;
    session-menu-toggle)
      if wahrwelt_noctalia_is_v4; then
        wahrwelt_noctalia_msg sessionMenu toggle
      else
        wahrwelt_noctalia_msg panel-toggle session
      fi
      ;;
    control-center-toggle)
      if wahrwelt_noctalia_is_v4; then
        wahrwelt_noctalia_msg controlCenter toggle
      else
        wahrwelt_noctalia_msg panel-toggle control-center
      fi
      ;;
    notifications-clear)
      if wahrwelt_noctalia_is_v4; then
        wahrwelt_noctalia_msg notifications dismissAll
      else
        wahrwelt_noctalia_msg notification-clear-active
      fi
      ;;
    settings-toggle)
      if wahrwelt_noctalia_is_v4; then
        wahrwelt_noctalia_msg settings toggle
      else
        wahrwelt_noctalia_msg settings-toggle
      fi
      ;;
    lock)
      if wahrwelt_noctalia_is_v4; then
        wahrwelt_noctalia_msg lockScreen lock
      else
        wahrwelt_noctalia_msg session lock
      fi
      ;;
    brightness-up)
      if wahrwelt_noctalia_is_v4; then
        wahrwelt_noctalia_msg brightness increase
      else
        wahrwelt_noctalia_msg brightness-up
      fi
      ;;
    brightness-down)
      if wahrwelt_noctalia_is_v4; then
        wahrwelt_noctalia_msg brightness decrease
      else
        wahrwelt_noctalia_msg brightness-down
      fi
      ;;
    clipboard-toggle)
      if wahrwelt_noctalia_is_v4; then
        wahrwelt_noctalia_msg launcher clipboard
      else
        wahrwelt_noctalia_msg panel-toggle clipboard
      fi
      ;;
    emoji-toggle)
      if wahrwelt_noctalia_is_v4; then
        wahrwelt_noctalia_msg launcher emoji
      else
        wahrwelt_noctalia_msg panel-toggle launcher /emo
      fi
      ;;
    *)
      shift
      wahrwelt_noctalia_msg "$action" "$@"
      ;;
  esac
}

wahrwelt_pid_matches() {
  local pid="$1"
  local pattern="$2"

  [ -n "$pid" ] || return 1
  ps -p "$pid" -o args= 2>/dev/null | grep -qE "$pattern"
}

wahrwelt_pid_has_env_regex() {
  local pid="$1"
  local regex="$2"
  local env_file

  [ -n "$pid" ] || return 1
  env_file="/proc/$pid/environ"
  [ -r "$env_file" ] || return 1
  { tr '\0' '\n' <"$env_file"; } 2>/dev/null | grep -qE "$regex"
}

wahrwelt_quickshell_pids() {
  pgrep -u "$wahrwelt_user_name" -f '(^|[ /])\.?quickshell(-wrapped)?([[:space:]]|$)' 2>/dev/null || true
}

wahrwelt_noctalia_pids() {
  local pid

  {
    pgrep -u "$wahrwelt_user_name" -x noctalia 2>/dev/null || true
    pgrep -u "$wahrwelt_user_name" -f '(^|/)noctalia([[:space:]]|$)' 2>/dev/null || true
    pgrep -u "$wahrwelt_user_name" -f "$wahrwelt_noctalia_v4_pattern" 2>/dev/null || true
    for pid in $(wahrwelt_quickshell_pids); do
      if wahrwelt_pid_has_env_regex "$pid" "$wahrwelt_noctalia_v4_env_pattern"; then
        printf '%s\n' "$pid"
      fi
    done
  } | sort -u
}

wahrwelt_noctalia_running() {
  wahrwelt_noctalia_pids | grep -q .
}

wahrwelt_read_active_shell() {
  local path="${1:-$wahrwelt_active_shell_state}"
  local stored=""

  if [ -f "$path" ]; then
    stored="$(tr -d '[:space:]' <"$path" 2>/dev/null || true)"
  fi

  if wahrwelt_valid_shell_profile "$stored"; then
    printf '%s' "$stored"
    return 0
  fi

  return 1
}

wahrwelt_lock_owner_running() {
  local owner_pid="$1"
  local owner_file="$2"
  local owner_name="$3"
  local owner_pattern="$4"
  local active_owner

  active_owner="$(cat "$owner_file" 2>/dev/null || true)"
  [ "$active_owner" = "$owner_name" ] || return 1
  wahrwelt_pid_matches "$owner_pid" "$owner_pattern"
}

wahrwelt_acquire_lock() {
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
    if wahrwelt_lock_owner_running "$owner_pid" "$owner_file" "$owner_name" "$owner_pattern"; then
      sleep "$delay"
      continue
    fi

    rm -rf -- "$lock_dir"
  done

  return 1
}
