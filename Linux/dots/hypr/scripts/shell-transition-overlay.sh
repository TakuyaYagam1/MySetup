#!/usr/bin/env bash
# shellcheck shell=bash
# shellcheck disable=SC2034

wahrwelt_shell_transition_config=wahrwelt-shell-transition
wahrwelt_shell_transition_target=shellTransition
wahrwelt_shell_transition_uptime_file=/proc/uptime
wahrwelt_shell_transition_instance_id=
wahrwelt_shell_transition_launcher_pid=
wahrwelt_shell_transition_started=0
wahrwelt_shell_transition_active=0
wahrwelt_shell_transition_visible_deadline_us=
wahrwelt_shell_transition_cleanup_deadline_us=

wahrwelt_shell_transition_now_us() {
  local uptime_value uptime_seconds uptime_fraction

  [ -r "$wahrwelt_shell_transition_uptime_file" ] || return 1
  IFS=' ' read -r uptime_value _ <"$wahrwelt_shell_transition_uptime_file" || return 1
  [[ "$uptime_value" =~ ^([0-9]+)\.([0-9]{1,6})$ ]] || return 1
  uptime_seconds="${BASH_REMATCH[1]}"
  uptime_fraction="${BASH_REMATCH[2]}000000"
  uptime_fraction="${uptime_fraction:0:6}"
  printf '%s\n' "$((10#$uptime_seconds * 1000000 + 10#$uptime_fraction))"
}

wahrwelt_shell_transition_remaining_us() {
  local deadline_us="$1"
  local now_us

  now_us="$(wahrwelt_shell_transition_now_us)" || return 1
  [[ "$now_us" =~ ^[0-9]+$ ]] || return 1
  [ "$now_us" -lt "$deadline_us" ] || return 1
  printf '%s\n' "$((deadline_us - now_us))"
}

wahrwelt_shell_transition_duration() {
  local duration_us="$1"

  [ "$duration_us" -gt 0 ] || return 1
  printf '%d.%06ds\n' "$((duration_us / 1000000))" "$((duration_us % 1000000))"
}

wahrwelt_shell_transition_timeout_before() {
  local deadline_us="$1"
  local maximum_us="$2"
  local remaining_us

  remaining_us="$(wahrwelt_shell_transition_remaining_us "$deadline_us")" || return 1
  if [ "$remaining_us" -gt "$maximum_us" ]; then
    remaining_us="$maximum_us"
  fi
  wahrwelt_shell_transition_duration "$remaining_us"
}

wahrwelt_shell_transition_sleep_before() {
  local deadline_us="$1"
  local maximum_us="$2"
  local duration

  duration="$(wahrwelt_shell_transition_timeout_before "$deadline_us" "$maximum_us")" || return 1
  sleep "$duration"
}

wahrwelt_shell_transition_valid_instance_id() {
  [[ "${1:-}" =~ ^[a-z0-9]{1,64}$ ]]
}

wahrwelt_shell_transition_list_ids() {
  local timeout_duration="$1"
  local output ids no_instances_pattern

  command -v qs >/dev/null 2>&1 || return 1
  command -v jq >/dev/null 2>&1 || return 1
  command -v timeout >/dev/null 2>&1 || return 1
  output="$(
    timeout "$timeout_duration" qs -c "$wahrwelt_shell_transition_config" \
      list -j 2>/dev/null
  )" || return 1
  no_instances_pattern=$'^No running instances for "[^"]+/shell\\.qml"\nUse --all to list all instances\\.$'
  if [[ "$output" =~ $no_instances_pattern ]]; then
    return 0
  fi
  ids="$(
    jq -er '
      if type != "array" then
        error("instance list is not an array")
      elif all(.[];
        type == "object" and
        (.config_path | type == "string" and length > 0) and
        (.id | type == "string" and test("^[a-z0-9]{1,64}$")) and
        (.launch_time | type == "string" and length > 0) and
        (.pid | type == "number" and . > 0 and floor == .) and
        (.shell_id | type == "string" and test("^[a-f0-9]{32}$"))
      ) and ([.[].id] | length == (unique | length)) then
        map(.id) | join("\n")
      else
        error("invalid instance entry")
      end
    ' <<<"$output" 2>/dev/null
  )" || return 2
  [ -n "$ids" ] && printf '%s\n' "$ids"
}

wahrwelt_shell_transition_kill_id() {
  local instance_id="$1"

  wahrwelt_shell_transition_valid_instance_id "$instance_id" || return 1
  timeout 0.075s qs kill -i "$instance_id" >/dev/null 2>&1
}

wahrwelt_shell_transition_stop_launcher() {
  local launcher_pid="$wahrwelt_shell_transition_launcher_pid"

  wahrwelt_shell_transition_launcher_pid=
  [[ "$launcher_pid" =~ ^[1-9][0-9]*$ ]] || return 0
  if jobs -pr | grep -Fqx -- "$launcher_pid"; then
    kill -KILL -- "$launcher_pid" 2>/dev/null || true
  fi
  wait "$launcher_pid" 2>/dev/null || true
}

wahrwelt_shell_transition_complete() {
  local owned_id="$wahrwelt_shell_transition_instance_id"

  if command -v qs >/dev/null 2>&1 && command -v timeout >/dev/null 2>&1 &&
    wahrwelt_shell_transition_valid_instance_id "$owned_id"; then
    wahrwelt_shell_transition_kill_id "$owned_id" || true
  fi
  wahrwelt_shell_transition_stop_launcher
  wahrwelt_shell_transition_active=0
  wahrwelt_shell_transition_started=0
  wahrwelt_shell_transition_instance_id=
  wahrwelt_shell_transition_visible_deadline_us=
  wahrwelt_shell_transition_cleanup_deadline_us=
}

wahrwelt_shell_transition_cleanup_all() {
  local attempt ids instance_id list_result
  local untrusted_list_seen=0

  command -v qs >/dev/null 2>&1 || return 1
  command -v timeout >/dev/null 2>&1 || return 1
  for attempt in 1 2 3 4 5 6 7 8; do
    if ids="$(wahrwelt_shell_transition_list_ids 0.075s)"; then
      list_result=0
    else
      list_result=$?
    fi
    if [ "$list_result" -eq 0 ]; then
      if [ -z "$ids" ]; then
        [ "$untrusted_list_seen" -eq 0 ] && return 0
        return 1
      fi
      while IFS= read -r instance_id; do
        [ -n "$instance_id" ] || continue
        wahrwelt_shell_transition_kill_id "$instance_id" || true
      done <<<"$ids"
    else
      untrusted_list_seen=1
      # A config-scoped kill is the only safe fallback when the selected
      # config cannot provide trustworthy instance IDs. QuickShell 0.3.1
      # kills only one matching instance, so repeat it within a fixed bound.
      timeout 0.075s qs -c "$wahrwelt_shell_transition_config" \
        kill >/dev/null 2>&1 || true
    fi
  done
  return 1
}

wahrwelt_shell_transition_status_with_timeout() {
  local timeout_duration="$1"
  local status

  wahrwelt_shell_transition_valid_instance_id \
    "$wahrwelt_shell_transition_instance_id" || return 1
  command -v qs >/dev/null 2>&1 || return 1
  command -v timeout >/dev/null 2>&1 || return 1
  status="$(
    timeout "$timeout_duration" qs ipc -i "$wahrwelt_shell_transition_instance_id" \
      call "$wahrwelt_shell_transition_target" status 2>/dev/null
  )" || return 1
  case "$status" in
    capturing | captured | outgoing | covered | incoming | settling | done | aborted)
      printf '%s\n' "$status"
      ;;
    *)
      return 2
      ;;
  esac
}

wahrwelt_shell_transition_status() {
  wahrwelt_shell_transition_status_with_timeout 0.075s
}

wahrwelt_shell_transition_abort() {
  local owned_id="$wahrwelt_shell_transition_instance_id"

  if command -v qs >/dev/null 2>&1 && command -v timeout >/dev/null 2>&1; then
    if wahrwelt_shell_transition_valid_instance_id "$owned_id"; then
      timeout 0.075s qs ipc -i "$owned_id" \
        call "$wahrwelt_shell_transition_target" abort >/dev/null 2>&1 || true
      wahrwelt_shell_transition_kill_id "$owned_id" || true
    fi
    wahrwelt_shell_transition_cleanup_all || true
  fi
  wahrwelt_shell_transition_stop_launcher
  wahrwelt_shell_transition_active=0
  wahrwelt_shell_transition_started=0
  wahrwelt_shell_transition_instance_id=
  wahrwelt_shell_transition_visible_deadline_us=
  wahrwelt_shell_transition_cleanup_deadline_us=
}

# The runtime-lock supervisor escalates a forwarded signal after one second.
# Kill the detached overlay before rollback work so it cannot outlive the
# supervised process group. This path is intentionally bounded to four 75ms
# QuickShell calls instead of the normal exhaustive cleanup loop.
wahrwelt_shell_transition_abort_signal_safe() {
  local owned_id="$wahrwelt_shell_transition_instance_id"
  local attempt

  if command -v qs >/dev/null 2>&1 && command -v timeout >/dev/null 2>&1; then
    if wahrwelt_shell_transition_valid_instance_id "$owned_id"; then
      timeout 0.075s qs ipc -i "$owned_id" \
        call "$wahrwelt_shell_transition_target" abort >/dev/null 2>&1 || true
      wahrwelt_shell_transition_kill_id "$owned_id" || true
    fi
    # The daemon may receive TERM between launch and instance discovery.
    # QuickShell 0.3.1 kills one config instance per call.
    for attempt in 1 2; do
      timeout 0.075s qs -c "$wahrwelt_shell_transition_config" \
        kill >/dev/null 2>&1 || true
    done
  fi
  wahrwelt_shell_transition_stop_launcher
  wahrwelt_shell_transition_active=0
  wahrwelt_shell_transition_started=0
  wahrwelt_shell_transition_instance_id=
  wahrwelt_shell_transition_visible_deadline_us=
  wahrwelt_shell_transition_cleanup_deadline_us=
}

wahrwelt_shell_transition_begin() {
  local status status_result now_us ids instance_count
  local deadline_us command_timeout list_result

  wahrwelt_shell_transition_active=0
  wahrwelt_shell_transition_started=0
  wahrwelt_shell_transition_instance_id=
  wahrwelt_shell_transition_visible_deadline_us=
  wahrwelt_shell_transition_cleanup_deadline_us=
  wahrwelt_shell_transition_stop_launcher
  command -v qs >/dev/null 2>&1 || return 1
  command -v jq >/dev/null 2>&1 || return 1
  command -v timeout >/dev/null 2>&1 || return 1

  wahrwelt_shell_transition_cleanup_all || return 1
  now_us="$(wahrwelt_shell_transition_now_us)" || {
    wahrwelt_shell_transition_cleanup_all || true
    return 1
  }
  [[ "$now_us" =~ ^[0-9]+$ ]] || {
    wahrwelt_shell_transition_cleanup_all || true
    return 1
  }
  deadline_us=$((now_us + 1000000))
  wahrwelt_shell_transition_started=1
  qs -c "$wahrwelt_shell_transition_config" >/dev/null 2>&1 &
  wahrwelt_shell_transition_launcher_pid=$!
  [[ "$wahrwelt_shell_transition_launcher_pid" =~ ^[1-9][0-9]*$ ]] || {
    wahrwelt_shell_transition_abort
    return 1
  }

  while :; do
    command_timeout="$(wahrwelt_shell_transition_timeout_before "$deadline_us" 75000)" || break
    if [ -z "$wahrwelt_shell_transition_instance_id" ]; then
      if ids="$(wahrwelt_shell_transition_list_ids "$command_timeout")"; then
        list_result=0
      else
        list_result=$?
      fi
      if [ "$list_result" -eq 2 ]; then
        wahrwelt_shell_transition_abort
        return 1
      fi
      if [ "$list_result" -eq 0 ]; then
        if [ -n "$ids" ]; then
          instance_count="$(printf '%s\n' "$ids" | awk 'NF { count++ } END { print count + 0 }')"
        else
          instance_count=0
        fi
        if [ "$instance_count" -gt 1 ]; then
          wahrwelt_shell_transition_abort
          return 1
        fi
        if [ "$instance_count" -eq 1 ]; then
          wahrwelt_shell_transition_instance_id="$ids"
        fi
      fi
      if [ -z "$wahrwelt_shell_transition_instance_id" ]; then
        wahrwelt_shell_transition_sleep_before "$deadline_us" 50000 || break
        continue
      fi
      command_timeout="$(wahrwelt_shell_transition_timeout_before "$deadline_us" 75000)" || break
    fi
    if status="$(
      wahrwelt_shell_transition_status_with_timeout "$command_timeout" 2>/dev/null
    )"; then
      status_result=0
    else
      status_result=$?
    fi
    if [ "$status_result" -eq 2 ]; then
      wahrwelt_shell_transition_abort
      return 1
    fi
    if [ "$status_result" -eq 0 ]; then
      case "$status" in
        captured)
          now_us="$(wahrwelt_shell_transition_now_us)" || {
            wahrwelt_shell_transition_abort
            return 1
          }
          [[ "$now_us" =~ ^[0-9]+$ ]] || {
            wahrwelt_shell_transition_abort
            return 1
          }
          wahrwelt_shell_transition_visible_deadline_us=$((now_us + 10000000))
          wahrwelt_shell_transition_cleanup_deadline_us=$((now_us + 10750000))
          command_timeout="$(
            wahrwelt_shell_transition_timeout_before \
              "$wahrwelt_shell_transition_cleanup_deadline_us" 75000
          )" || {
            wahrwelt_shell_transition_abort
            return 1
          }
          timeout "$command_timeout" qs ipc \
            -i "$wahrwelt_shell_transition_instance_id" \
            call "$wahrwelt_shell_transition_target" start >/dev/null 2>&1 || {
            wahrwelt_shell_transition_abort
            return 1
          }
          command_timeout="$(
            wahrwelt_shell_transition_timeout_before \
              "$wahrwelt_shell_transition_cleanup_deadline_us" 75000
          )" || {
            wahrwelt_shell_transition_abort
            return 1
          }
          status="$(
            wahrwelt_shell_transition_status_with_timeout \
              "$command_timeout" 2>/dev/null
          )" || {
            wahrwelt_shell_transition_abort
            return 1
          }
          [ "$status" = outgoing ] || {
            wahrwelt_shell_transition_abort
            return 1
          }
          wahrwelt_shell_transition_active=1
          return 0
          ;;
        capturing)
          ;;
        *)
          wahrwelt_shell_transition_abort
          return 1
          ;;
      esac
    fi
    wahrwelt_shell_transition_sleep_before "$deadline_us" 50000 || break
  done

  wahrwelt_shell_transition_abort
  return 1
}

wahrwelt_shell_transition_profile_handle() {
  case "$1" in
    caelestia) printf '%s\n' "${caelestia_handle:-__caelestia__}" ;;
    noctalia) printf '%s\n' "${noctalia_handle:-__noctalia__}" ;;
    end4) printf '%s\n' "${end4_official_handle:-__end4_official__}" ;;
    end4-pc) printf '%s\n' "${end4_pc_handle:-__end4_pc__}" ;;
    *) return 1 ;;
  esac
}

wahrwelt_shell_transition_profile_running() {
  local profile="$1"
  local handle pid

  handle="$(wahrwelt_shell_transition_profile_handle "$profile")" || return 1
  while IFS= read -r pid; do
    [[ "$pid" =~ ^[1-9][0-9]*$ ]] && return 0
  done < <(matching_pids "$handle")
  return 1
}

wahrwelt_shell_transition_target_layers_ready() {
  local profile="$1"
  local timeout_duration="${2:-0.075s}"
  local handle layers pids_json
  local pids=()

  command -v hyprctl >/dev/null 2>&1 || return 0
  command -v jq >/dev/null 2>&1 || return 0
  handle="$(wahrwelt_shell_transition_profile_handle "$profile")" || return 1
  mapfile -t pids < <(matching_pids "$handle" | sed -n '/^[1-9][0-9]*$/p' | sort -u)
  [ "${#pids[@]}" -gt 0 ] || return 1
  pids_json="$(printf '%s\n' "${pids[@]}" | jq -Rsc '
    split("\n") | map(select(length > 0) | tonumber)
  ')" || return 1
  command -v timeout >/dev/null 2>&1 || return 0
  layers="$(timeout "$timeout_duration" hyprctl -j layers 2>/dev/null)" || return 1
  jq -e --argjson targetPids "$pids_json" '
    type == "object" and length > 0 and
    all(to_entries[];
      [.value.levels[]?[]? | .pid?] as $outputPids |
      any($outputPids[]; . as $pid | $targetPids | index($pid) != null)
    )
  ' <<<"$layers" >/dev/null
}

wahrwelt_shell_transition_wait_target_ready() {
  local profile="$1"
  local command_timeout incoming_deadline_us

  [ "$wahrwelt_shell_transition_active" -eq 1 ] || return 0
  [[ "$wahrwelt_shell_transition_visible_deadline_us" =~ ^[0-9]+$ ]] || return 1
  incoming_deadline_us=$((wahrwelt_shell_transition_visible_deadline_us - 3000000))
  [ "$incoming_deadline_us" -gt 0 ] || return 1
  while command_timeout="$(
    wahrwelt_shell_transition_timeout_before \
      "$incoming_deadline_us" 75000
  )"; do
    wahrwelt_shell_transition_target_layers_ready \
      "$profile" "$command_timeout" && return 0
    wahrwelt_shell_transition_sleep_before \
      "$incoming_deadline_us" 100000 || break
  done
  return 1
}

wahrwelt_shell_transition_bridge_budget_available() {
  local minimum_us="${1:-0}"
  local now_us incoming_deadline_us remaining_us

  [ "$wahrwelt_shell_transition_active" -eq 1 ] || return 0
  [[ "$minimum_us" =~ ^[0-9]{1,18}$ ]] || return 1
  [[ "$wahrwelt_shell_transition_visible_deadline_us" =~ ^[0-9]+$ ]] || return 1
  now_us="$(wahrwelt_shell_transition_now_us)" || return 1
  [[ "$now_us" =~ ^[0-9]+$ ]] || return 1
  incoming_deadline_us=$((wahrwelt_shell_transition_visible_deadline_us - 3000000))
  [ "$now_us" -lt "$incoming_deadline_us" ] || return 1
  remaining_us=$((incoming_deadline_us - now_us))
  minimum_us=$((10#$minimum_us))
  [ "$minimum_us" -lt "$remaining_us" ]
}

wahrwelt_shell_transition_wait_covered() {
  local status status_result command_timeout cover_deadline_us

  [ "$wahrwelt_shell_transition_active" -eq 1 ] || return 1
  [[ "$wahrwelt_shell_transition_visible_deadline_us" =~ ^[0-9]+$ ]] || {
    wahrwelt_shell_transition_abort
    return 1
  }
  cover_deadline_us=$((wahrwelt_shell_transition_visible_deadline_us - 6000000))
  [ "$cover_deadline_us" -gt 0 ] || {
    wahrwelt_shell_transition_abort
    return 1
  }
  while command_timeout="$(
    wahrwelt_shell_transition_timeout_before \
      "$cover_deadline_us" 500000
  )"; do
    if status="$(
      wahrwelt_shell_transition_status_with_timeout "$command_timeout" 2>/dev/null
    )"; then
      status_result=0
    else
      status_result=$?
    fi
    if [ "$status_result" -eq 2 ]; then
      wahrwelt_shell_transition_abort
      return 1
    fi
    if [ "$status_result" -eq 0 ]; then
      case "$status" in
        covered)
          return 0
          ;;
        captured | outgoing)
          ;;
        *)
          wahrwelt_shell_transition_abort
          return 1
          ;;
      esac
    fi
    wahrwelt_shell_transition_sleep_before \
      "$cover_deadline_us" 50000 || break
  done

  wahrwelt_shell_transition_abort
  return 1
}

wahrwelt_shell_transition_wait_done() {
  local status command_timeout

  [ "$wahrwelt_shell_transition_active" -eq 1 ] || return 0
  [[ "$wahrwelt_shell_transition_visible_deadline_us" =~ ^[0-9]+$ ]] || {
    wahrwelt_shell_transition_abort
    return 1
  }
  [[ "$wahrwelt_shell_transition_cleanup_deadline_us" =~ ^[0-9]+$ ]] || {
    wahrwelt_shell_transition_abort
    return 1
  }
  while command_timeout="$(
    wahrwelt_shell_transition_timeout_before \
      "$wahrwelt_shell_transition_cleanup_deadline_us" 75000
  )"; do
    if ! status="$(
      wahrwelt_shell_transition_status_with_timeout "$command_timeout" 2>/dev/null
    )"; then
      wahrwelt_shell_transition_abort
      return 1
    fi
    case "$status" in
      done)
        wahrwelt_shell_transition_complete
        return 0
        ;;
      outgoing | covered | incoming | settling)
        ;;
      *)
        wahrwelt_shell_transition_abort
        return 1
        ;;
    esac
    wahrwelt_shell_transition_sleep_before \
      "$wahrwelt_shell_transition_cleanup_deadline_us" 50000 || break
  done

  wahrwelt_shell_transition_abort
  return 1
}
