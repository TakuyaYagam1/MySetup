#!/usr/bin/env bash
# shellcheck shell=bash
# shellcheck disable=SC2034,SC2154

is_running() {
  matching_pids "$1" | grep -q .
}

running_count() {
  matching_pids "$1" | sort -u | wc -l | tr -d '[:space:]'
}

matching_pids() {
  local handle="$1"
  local pid

  case "$handle" in
    "$selector_handle")
      pgrep -u "$user_name" -f "$selector_pattern" 2>/dev/null || true
      ;;
    "$caelestia_handle")
      pgrep -u "$user_name" -f "$caelestia_pattern" 2>/dev/null || true
      ;;
    "$caelestia_resizer_handle")
      pgrep -u "$user_name" -f '(^|[ /])(\.caelestia-wrapped|caelestia)[[:space:]]+resizer([[:space:]]|$)' 2>/dev/null || true
      ;;
    "$noctalia_handle")
      wahrwelt_noctalia_pids
      ;;
    "$end4_handle")
      for pid in $(wahrwelt_quickshell_pids); do
        if wahrwelt_pid_has_env_regex "$pid" "$end4_env_pattern"; then
          printf '%s\n' "$pid"
        fi
      done | sort -u
      ;;
    "$end4_official_handle")
      wahrwelt_end4_profile_pids end4 | sort -u
      ;;
    "$end4_pc_handle")
      wahrwelt_end4_profile_pids end4-pc | sort -u
      ;;
    "$end4_idle_handle")
      for pid in $(pgrep -u "$user_name" -x hypridle 2>/dev/null || true); do
        if wahrwelt_pid_has_adjacent_args "$pid" -c "$end4_idle_config"; then
          printf '%s\n' "$pid"
        fi
      done
      ;;
    *)
      pgrep -u "$user_name" -f "$handle" 2>/dev/null || true
      ;;
  esac
}

kill_matching_pids() {
  local handle="$1"
  local signal="${2:-TERM}"
  local pid

  while read -r pid; do
    [ -n "$pid" ] || continue
    kill -"$signal" "$pid" >/dev/null 2>&1 || true
  done < <(matching_pids "$handle" | sort -u)
}

wait_until_stopped() {
  local handle="$1"
  local attempt

  for attempt in $(seq 1 20); do
    if ! is_running "$handle"; then
      return 0
    fi
    sleep 0.1
  done

  kill_matching_pids "$handle" KILL
  sleep 0.1
  ! is_running "$handle"
}

cleanup_legacy_end4_processes() {
  local tokens
  local attempt pid
  local -a pids=()

  if ! tokens="$(wahrwelt_read_end4_upgrade_tokens)"; then
    log "failed to read durable pre-marker end4 process provenance"
    return 1
  fi
  legacy_end4_upgrade_tokens="$tokens"
  [ -n "$tokens" ] || return 0
  mapfile -t pids < <(wahrwelt_legacy_end4_upgrade_pids "$tokens" | sort -u)
  if [ "${#pids[@]}" -eq 0 ]; then
    if ! legacy_end4_upgrade_tokens="$(wahrwelt_remove_end4_upgrade_tokens "$tokens")"; then
      log "failed to consume resolved pre-marker end4 process provenance"
      return 1
    fi
    return 0
  fi

  log "stopping pre-marker end4 process during runtime upgrade"
  for pid in "${pids[@]}"; do
    if wahrwelt_legacy_end4_upgrade_pids "$tokens" | grep -Fqx -- "$pid"; then
      kill -TERM "$pid" >/dev/null 2>&1 || true
    fi
  done
  for attempt in $(seq 1 20); do
    if ! wahrwelt_legacy_end4_upgrade_pids "$tokens" | grep -q .; then
      if ! legacy_end4_upgrade_tokens="$(wahrwelt_remove_end4_upgrade_tokens "$tokens")"; then
        log "failed to consume stopped pre-marker end4 process provenance"
        return 1
      fi
      return 0
    fi
    sleep 0.1
  done
  while read -r pid; do
    [ -n "$pid" ] || continue
    if wahrwelt_legacy_end4_upgrade_pids "$tokens" | grep -Fqx -- "$pid"; then
      kill -KILL "$pid" >/dev/null 2>&1 || true
    fi
  done < <(wahrwelt_legacy_end4_upgrade_pids "$tokens" | sort -u)
  sleep 0.1
  if wahrwelt_legacy_end4_upgrade_pids "$tokens" | grep -q .; then
    log "failed to stop pre-marker end4 process during runtime upgrade"
    return 1
  fi
  if ! legacy_end4_upgrade_tokens="$(wahrwelt_remove_end4_upgrade_tokens "$tokens")"; then
    log "failed to consume stopped pre-marker end4 process provenance"
    return 1
  fi
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
