#!/usr/bin/env bash
# shellcheck shell=bash
# shellcheck disable=SC2154

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
      {
        pgrep -u "$user_name" -f "$noctalia_pattern" 2>/dev/null || true
        for pid in $(mysetup_quickshell_pids); do
          if mysetup_pid_has_env_regex "$pid" "$noctalia_env_pattern"; then
            printf '%s\n' "$pid"
          fi
        done
      } | sort -u
      ;;
    "$end4_handle")
      pgrep -u "$user_name" -f "$end4_pattern" 2>/dev/null || true
      ;;
    "$end4_idle_handle")
      for pid in $(pgrep -u "$user_name" -x hypridle 2>/dev/null || true); do
        if ps -p "$pid" -o args= 2>/dev/null | grep -Fq -- "-c $end4_idle_config"; then
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
