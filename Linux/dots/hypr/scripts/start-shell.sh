#!/usr/bin/env bash
set -uo pipefail

script_dir="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=Linux/dots/hypr/scripts/shell-runtime.sh
. "$script_dir/shell-runtime.sh"

requested_profile="${1:-}"
config_home="$mysetup_config_home"
runtime_dir="$mysetup_runtime_session_dir"
persistent_state_file="$mysetup_active_shell_state"
log_file="$mysetup_log_file"
lock_dir="$runtime_dir/mysetup-shell.lock"
lock_owner_file="$lock_dir/owner"
hypr_runtime_dir="$mysetup_hypr_runtime_dir"
user_name="$mysetup_user_name"
selector_pattern="$mysetup_selector_pattern"
end4_pattern="$mysetup_end4_pattern"
noctalia_pattern="$mysetup_noctalia_pattern"
caelestia_pattern="$mysetup_caelestia_pattern"
selector_handle='__selector__'
caelestia_handle='__caelestia__'
caelestia_resizer_handle='__caelestia_resizer__'
noctalia_handle='__noctalia__'
end4_handle='__end4__'
end4_idle_handle='__end4_idle__'
end4_idle_config="$hypr_runtime_dir/hypridle.conf"
noctalia_env_pattern="$mysetup_noctalia_env_pattern"

# shellcheck source=Linux/dots/hypr/scripts/shell-runtime-env.sh
. "$script_dir/shell-runtime-env.sh"
# shellcheck source=Linux/dots/hypr/scripts/shell-profile-sync.sh
. "$script_dir/shell-profile-sync.sh"
# shellcheck source=Linux/dots/hypr/scripts/shell-end4-overrides.sh
. "$script_dir/shell-end4-overrides.sh"
# shellcheck source=Linux/dots/hypr/scripts/shell-process.sh
. "$script_dir/shell-process.sh"

prepare_runtime_environment

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

  if mysetup_valid_shell_profile "$stored"; then
      printf '%s' "$stored"
      return 0
  fi

  printf '%s' "$mysetup_default_shell_profile"
}

profile="$(resolve_profile "$requested_profile")"

if ! mysetup_valid_shell_profile "$profile"; then
  log "unknown shell before lock: $profile"
  exit 1
fi

hypr_dir() {
  mysetup_hypr_dir_path
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
    if [ "$lock_owner" = "mysetup-start-shell" ] && mysetup_pid_matches "$lock_pid" '(^|[ /])start-shell\.sh([[:space:]]|$)'; then
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

  kill_matching_pids "$caelestia_handle" TERM
  wait_until_stopped "$caelestia_handle" >/dev/null 2>&1 || true
  stop_caelestia_resizer
}

stop_caelestia_resizer() {
  log "stopping caelestia resizer"
  kill_matching_pids "$caelestia_resizer_handle" TERM
  wait_until_stopped "$caelestia_resizer_handle" >/dev/null 2>&1 || true
}

stop_noctalia() {
  log "stopping noctalia shell"
  kill_matching_pids "$noctalia_handle" TERM
  wait_until_stopped "$noctalia_handle" >/dev/null 2>&1 || true
}

stop_shell_selector() {
  log "stopping shell selector"
  kill_matching_pids "$selector_handle" TERM
  wait_until_stopped "$selector_handle" >/dev/null 2>&1 || true
}

stop_end4() {
  log "stopping end4 shell"
  kill_matching_pids "$end4_handle" TERM
  wait_until_stopped "$end4_handle" >/dev/null 2>&1 || true
}

stop_end4_idle() {
  log "stopping end4 hypridle"
  kill_matching_pids "$end4_idle_handle" TERM
  wait_until_stopped "$end4_idle_handle" >/dev/null 2>&1 || true
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

ensure_end4_idle() {
  local idle_config

  idle_config="$end4_idle_config"

  if [ ! -f "$idle_config" ]; then
    log "end4 hypridle config missing: $idle_config"
    return 1
  fi

  if command -v hypridle >/dev/null 2>&1; then
    start_with_retry "end4 hypridle" "$end4_idle_handle" hypridle -c "$idle_config" || return 1
    return 0
  fi

  log "hypridle command not found"
  return 1
}

start_profile_shell() {
  case "$profile" in
    caelestia)
      dedupe_shell "caelestia" "$caelestia_handle" stop_caelestia || true
      dedupe_shell "caelestia resizer" "$caelestia_resizer_handle" stop_caelestia_resizer || true

      if command -v caelestia >/dev/null 2>&1; then
        start_with_retry "caelestia" "$caelestia_handle" caelestia shell -d || return 1
      elif command -v caelestia-shell >/dev/null 2>&1; then
        start_with_retry "caelestia-shell" "$caelestia_handle" caelestia-shell -d || return 1
      else
        log "caelestia command not found"
        return 1
      fi

      if command -v caelestia >/dev/null 2>&1 && ! is_running "$caelestia_resizer_handle"; then
        (caelestia resizer -d >>"$log_file" 2>&1 &)
      fi
      ;;

    noctalia)
      dedupe_shell "noctalia" "$noctalia_handle" stop_noctalia || true

      if command -v noctalia-shell >/dev/null 2>&1; then
        start_with_retry "noctalia" "$noctalia_handle" noctalia-shell -d || return 1
      else
        log "noctalia-shell command not found"
        return 1
      fi
      ;;

    end4)
      ensure_end4_idle || true
      dedupe_shell "end4" "$end4_handle" stop_end4 || true

      if command -v qs-end4 >/dev/null 2>&1; then
        start_with_retry "end4" "$end4_handle" qs-end4 -n -d -c ii || return 1
        sleep 0.5
        apply_end4_hypr_runtime_overrides
      else
        log "qs-end4 command not found"
        return 1
      fi
      ;;
  esac
}

valid_profile() {
  mysetup_valid_shell_profile "$1"
}

log "requested profile=$profile input=${requested_profile:-auto}"
wait_for_session

previous=""
if [ -f "$persistent_state_file" ]; then
  previous="$(tr -d '[:space:]' <"$persistent_state_file" 2>/dev/null || true)"
fi

if ! prepare_profile_or_fallback; then
  log "aborting shell switch before stopping current shell; profile=$profile"
  exit 1
fi

if [ "$previous" != "$profile" ] || [ -n "$requested_profile" ]; then
  stop_quickshells
  sleep 0.2
else
  stop_inactive_shells "$profile"
fi

if [ "$profile" != "end4" ]; then
  stop_end4_idle
fi

if ! start_profile_shell; then
  failed_profile="$profile"
  log "shell start failed for profile=$failed_profile; active state was not changed"

  if valid_profile "$previous" && [ "$previous" != "$failed_profile" ]; then
    profile="$previous"
    log "attempting fallback to previous profile=$profile"
    if prepare_profile_or_fallback && start_profile_shell; then
      persist_profile || log "failed to persist runtime shell state for fallback profile=$profile"
      reload_hypr
      apply_end4_hypr_runtime_overrides
      exit 1
    fi
  fi

  exit 1
fi

persist_profile || log "failed to persist runtime shell state for profile=$profile"
reload_hypr
apply_end4_hypr_runtime_overrides
