#!/usr/bin/env bash
set -euo pipefail

scripts_dir="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
helper="$scripts_dir/shell-transition-overlay.sh"
test_root="$(mktemp -d)"
trap 'rm -rf -- "$test_root"' EXIT

operations="$test_root/operations"
instances="$test_root/instances"
unrelated_state="$test_root/unrelated-state"
status_count_file="$test_root/status-count"
clock_file="$test_root/monotonic-us"
uptime_file="$test_root/uptime"
wall_clock_file="$test_root/wall-clock"
wall_jumps="$test_root/wall-jumps"
start_marker="$test_root/start-marker"
owned_id=ownednew01
race_id=raceinst01
status_mode=start-outgoing
forced_status=
start_mode=success
launch_mode=success
list_mode=normal
launch_advance_us=0
layers_mode=ready

: >"$operations"
: >"$instances"
: >"$wall_jumps"
: >"$start_marker"
printf '%s\n' running >"$unrelated_state"
printf '%s\n' 0 >"$status_count_file"
printf '%s\n' 1000000 >"$clock_file"
printf '%s\n' 1000000000 >"$wall_clock_file"
failures=0

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  failures=$((failures + 1))
}

assert_eq() {
  local want="$1"
  local got="$2"
  local message="$3"

  [ "$got" = "$want" ] || fail "$message: got $got, want $want"
}

write_uptime() {
  local now_us seconds fraction

  now_us="$(cat "$clock_file")"
  seconds=$((now_us / 1000000))
  fraction=$((now_us % 1000000))
  printf '%d.%06d 999.00\n' "$seconds" "$fraction" >"$uptime_file"
}

advance_clock_us() {
  local delta_us="$1"
  local now_us wall_us

  now_us="$(cat "$clock_file")"
  printf '%s\n' "$((now_us + delta_us))" >"$clock_file"
  wall_us="$(cat "$wall_clock_file")"
  if [ $((now_us / 50000 % 2)) -eq 0 ]; then
    wall_us=$((wall_us + 9000000000))
    printf '%s\n' forward >>"$wall_jumps"
  else
    wall_us=$((wall_us - 18000000000))
    printf '%s\n' backward >>"$wall_jumps"
  fi
  printf '%s\n' "$wall_us" >"$wall_clock_file"
  write_uptime
}

reset_fixture() {
  : >"$operations"
  : >"$instances"
  : >"$start_marker"
  printf '%s\n' running >"$unrelated_state"
  printf '%s\n' 0 >"$status_count_file"
  printf '%s\n' 1000000 >"$clock_file"
  printf '%s\n' 1000000000 >"$wall_clock_file"
  : >"$wall_jumps"
  status_mode=start-outgoing
  forced_status=
  start_mode=success
  launch_mode=success
  list_mode=normal
  launch_advance_us=0
  layers_mode=ready
  write_uptime
}

instance_exists() {
  grep -Fqx -- "$1" "$instances"
}

remove_instance() {
  local id="$1"
  local replacement="$test_root/instances.next"

  grep -Fvx -- "$id" "$instances" >"$replacement" || true
  mv -- "$replacement" "$instances"
}

remove_oldest_instance() {
  local replacement="$test_root/instances.next"

  [ -s "$instances" ] || return 0
  awk 'NR > 1' "$instances" >"$replacement"
  mv -- "$replacement" "$instances"
}

list_instances_json() {
  local id separator=

  if [ ! -s "$instances" ]; then
    # QuickShell 0.3.1 emits human-readable output even with -j when the
    # selected config exists but has no running instance.
    printf '%s\n' \
      'No running instances for "/nix/store/test/wahrwelt-shell-transition/shell.qml"' \
      'Use --all to list all instances.'
    return 0
  fi
  printf '['
  while IFS= read -r id; do
    printf '%s{"config_path":"/nix/store/test/wahrwelt-shell-transition/shell.qml","id":"%s","launch_time":"2026-08-27 12:00:00","pid":1234,"shell_id":"0123456789abcdef0123456789abcdef"}' \
      "$separator" "$id"
    separator=,
  done <"$instances"
  printf ']\n'
}

timeout() {
  local duration="$1"
  shift

  {
    printf 'timeout\t%s' "$duration"
    printf '\t%s' "$@"
    printf '\n'
  } >>"$operations"
  if [ "${1:-}" = qs ] && [[ " $* " == *' -d '* ]] && [ "$launch_advance_us" -gt 0 ]; then
    advance_clock_us "$launch_advance_us"
  fi
  "$@"
}

sleep() {
  local numeric delta_us

  numeric="${1%s}"
  delta_us="$(awk -v seconds="$numeric" 'BEGIN { printf "%.0f", seconds * 1000000 }')"
  advance_clock_us "$delta_us"
}

qs_status() {
  local id="$1" status_calls

  instance_exists "$id" || return 1
  status_calls="$(cat "$status_count_file")"
  status_calls=$((status_calls + 1))
  printf '%s\n' "$status_calls" >"$status_count_file"
  if [ "$id" != "$owned_id" ]; then
    printf '%s\n' captured
    return 0
  fi
  case "$status_mode" in
    captured) printf '%s\n' captured ;;
    unavailable-then-captured)
      if [ -s "$start_marker" ]; then
        printf '%s\n' outgoing
      else
        case "$status_calls" in
          1 | 2 | 3) return 1 ;;
          4) printf '%s\n' capturing ;;
          *) printf '%s\n' captured ;;
        esac
      fi
      ;;
    unavailable) return 1 ;;
    malformed) printf '%s\n' 'captured extra' ;;
    capturing) printf '%s\n' capturing ;;
    fixed) printf '%s\n' "$forced_status" ;;
    start-outgoing)
      [ -s "$start_marker" ] && printf '%s\n' outgoing || printf '%s\n' captured
      ;;
    capture-delay-then-outgoing)
      if [ -s "$start_marker" ]; then
        printf '%s\n' outgoing
      else
        advance_clock_us 250000
        printf '%s\n' captured
      fi
      ;;
    start-to-covered)
      if [ ! -s "$start_marker" ]; then
        printf '%s\n' captured
      elif [ "$(cat "$clock_file")" -lt "$(($(cat "$start_marker") + 150000))" ]; then
        printf '%s\n' outgoing
      else
        printf '%s\n' covered
      fi
      ;;
    start-transient-then-covered)
      if [ ! -s "$start_marker" ]; then
        printf '%s\n' captured
      elif [ "$status_calls" -eq 2 ]; then
        printf '%s\n' outgoing
      elif [ "$status_calls" -eq 3 ]; then
        return 1
      else
        printf '%s\n' covered
      fi
      ;;
    start-covered-after-three-seconds)
      if [ ! -s "$start_marker" ]; then
        printf '%s\n' captured
      elif [ "$(cat "$clock_file")" -lt "$(($(cat "$start_marker") + 3100000))" ]; then
        printf '%s\n' outgoing
      else
        printf '%s\n' covered
      fi
      ;;
    start-outgoing-then-incoming)
      if [ ! -s "$start_marker" ]; then
        printf '%s\n' captured
      elif [ "$status_calls" -le 2 ]; then
        printf '%s\n' outgoing
      else
        printf '%s\n' incoming
      fi
      ;;
    start-outgoing-then-unavailable)
      if [ ! -s "$start_marker" ]; then
        printf '%s\n' captured
      elif [ "$status_calls" -le 2 ]; then
        printf '%s\n' outgoing
      else
        return 1
      fi
      ;;
    timeline)
      if [ ! -s "$start_marker" ]; then
        printf '%s\n' captured
      else
        local started_us elapsed_us
        started_us="$(cat "$start_marker")"
        elapsed_us=$(($(cat "$clock_file") - started_us))
        if [ "$elapsed_us" -lt 3000000 ]; then
          printf '%s\n' outgoing
        elif [ "$elapsed_us" -lt 7000000 ]; then
          printf '%s\n' covered
        elif [ "$elapsed_us" -lt 9900000 ]; then
          printf '%s\n' incoming
        elif [ "$elapsed_us" -lt 10000000 ]; then
          printf '%s\n' settling
        else
          printf '%s\n' 'done'
        fi
      fi
      ;;
    *) return 1 ;;
  esac
}

qs() {
  local id command_name

  {
    printf 'qs'
    printf '\t%s' "$@"
    printf '\n'
  } >>"$operations"

  case "$*" in
    '-c wahrwelt-shell-transition list -j')
      case "$list_mode" in
        normal) list_instances_json ;;
        malformed) printf '%s\n' '{not-json' ;;
        failure) return 1 ;;
        fail-after-launch)
          [ -s "$instances" ] && return 1
          list_instances_json
          ;;
      esac
      return
      ;;
    '-c wahrwelt-shell-transition kill')
      remove_oldest_instance
      return 0
      ;;
    '-d -c wahrwelt-shell-transition' | '-n -d -c wahrwelt-shell-transition')
      if [ "$*" = '-n -d -c wahrwelt-shell-transition' ] && [ -s "$instances" ]; then
        return 0
      fi
      printf '%s\n' "$owned_id" >>"$instances"
      case "$launch_mode" in
        success) return 0 ;;
        duplicate)
          printf '%s\n' "$race_id" >>"$instances"
          return 0
          ;;
        late-register)
          (
            command sleep 0.2
            printf '%s\n' "$owned_id" >>"$instances"
          ) &
          return 124
          ;;
        fail-live) return 124 ;;
      esac
      ;;
    '-c wahrwelt-shell-transition')
      if [ "$launch_advance_us" -gt 0 ]; then
        advance_clock_us "$launch_advance_us"
      fi
      case "$launch_mode" in
        success)
          printf '%s\n' "$owned_id" >>"$instances"
          return 0
          ;;
        duplicate)
          printf '%s\n' "$owned_id" "$race_id" >>"$instances"
          return 0
          ;;
        late-register | fail-live) return 124 ;;
      esac
      ;;
  esac

  if [ "$#" -eq 3 ] && [ "$1" = kill ] && [ "$2" = -i ]; then
    id="$3"
    instance_exists "$id" || return 1
    remove_instance "$id"
    return 0
  fi
  if [ "$#" -eq 6 ] && [ "$1" = ipc ] && [ "$2" = -i ] &&
    [ "$4" = call ] && [ "$5" = shellTransition ]; then
    id="$3"
    command_name="$6"
    case "$command_name" in
      status) qs_status "$id" ;;
      start)
        instance_exists "$id" || return 1
        [ "$start_mode" = success ] || return 1
        cat "$clock_file" >"$start_marker"
        ;;
      abort) instance_exists "$id" ;;
      *) return 97 ;;
    esac
    return
  fi

  # Legacy config-only IPC is modeled so the pre-fix helper reaches assertions.
  if [ "$#" -eq 6 ] && [ "$1" = -c ] && [ "$2" = wahrwelt-shell-transition ] &&
    [ "$3" = ipc ] && [ "$4" = call ] && [ "$5" = shellTransition ]; then
    id="$(sed -n '1p' "$instances")"
    [ -n "$id" ] || return 1
    case "$6" in
      status) qs_status "$id" ;;
      start) cat "$clock_file" >"$start_marker" ;;
      abort) return 0 ;;
      *) return 97 ;;
    esac
    return
  fi
  return 97
}

matching_pids() {
  case "$1" in
    __caelestia__) printf '%s\n' 101 202 ;;
    *) return 1 ;;
  esac
}

hyprctl() {
  [ "$1" = -j ] && [ "$2" = layers ] || return 1
  case "$layers_mode" in
    ready)
      printf '%s\n' '{"eDP-1":{"levels":{"2":[{"pid":101}]}},"HDMI-A-1":{"levels":{"2":[{"pid":202}]}}}'
      ;;
    partial)
      printf '%s\n' '{"eDP-1":{"levels":{"2":[{"pid":101}]}},"HDMI-A-1":{"levels":{"2":[{"pid":999}]}}}'
      ;;
    *) return 1 ;;
  esac
}

export -f timeout sleep qs matching_pids hyprctl

# shellcheck source=Linux/dots/hypr/scripts/shell-transition-overlay.sh
. "$helper"
wahrwelt_shell_transition_uptime_file="$uptime_file"

for valid_id in a1b2c3d a1b2c3d4e5f6g7h8; do
  reset_fixture
  owned_id="$valid_id"
  if wahrwelt_shell_transition_begin; then
    assert_eq "$valid_id" "$wahrwelt_shell_transition_instance_id" \
      "variable-length full instance ID $valid_id discovery"
    if ! assert_status="$(wahrwelt_shell_transition_status)"; then
      fail "variable-length full instance ID $valid_id status IPC failed"
    else
      assert_eq outgoing "$assert_status" \
        "variable-length full instance ID $valid_id exact status"
    fi
  else
    fail "valid variable-length full instance ID $valid_id was rejected"
  fi
  wahrwelt_shell_transition_abort
  grep -Fqx $'qs\tkill\t-i\t'"$valid_id" "$operations" ||
    fail "variable-length full instance ID $valid_id was not exact-killed"
  [ ! -s "$instances" ] ||
    fail "variable-length full instance ID $valid_id remained after abort"
done
owned_id=ownednew01

for invalid_id in '' 'unsafe/id' 'abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklm'; do
  reset_fixture
  printf '%s\n' "$invalid_id" >"$instances"
  if wahrwelt_shell_transition_begin; then
    fail "invalid full instance ID ${invalid_id:-empty} activated capture"
  fi
  [ ! -s "$instances" ] ||
    fail "invalid full instance ID ${invalid_id:-empty} survived exact-config cleanup"
done

reset_fixture
printf '%s\n' dupid1234 dupid1234 >"$instances"
if wahrwelt_shell_transition_begin; then
  fail 'duplicate full instance IDs activated capture'
fi
[ ! -s "$instances" ] || fail 'duplicate full instance IDs survived exact-config cleanup'

reset_fixture
printf '%s\n' staleold01 staleold02 >"$instances"
wahrwelt_shell_transition_begin || fail 'stale-instance cleanup did not permit a fresh exact launch'
assert_eq "$owned_id" "${wahrwelt_shell_transition_instance_id:-}" \
  'begin did not retain the full owned QuickShell instance ID'
if grep -Eq $'qs\tipc\t-i\t(staleold01|staleold02)\t' "$operations"; then
  fail 'a stale captured instance was accepted for runtime IPC'
fi
for stale_id in staleold01 staleold02; do
  grep -Fqx $'qs\tkill\t-i\t'"$stale_id" "$operations" ||
    fail "stale transition instance $stale_id was not exact-killed"
done
assert_eq outgoing "$(wahrwelt_shell_transition_status)" 'owned instance exact IPC status'
[ -s "$unrelated_state" ] || fail 'stale cleanup removed unrelated QuickShell state'
wahrwelt_shell_transition_abort
[ ! -s "$instances" ] || fail 'exact abort retained a transition instance'

reset_fixture
status_mode=unavailable-then-captured
if ! wahrwelt_shell_transition_begin; then
  fail 'cold exact IPC registration was not retried to captured'
fi
[ "$(cat "$status_count_file")" -ge 5 ] || fail 'cold registration did not retry status'
wahrwelt_shell_transition_abort

reset_fixture
launch_mode=duplicate
if wahrwelt_shell_transition_begin; then
  fail 'ambiguous duplicate post-launch instances activated capture'
fi
[ ! -s "$instances" ] || fail 'duplicate race cleanup retained a transition instance'

reset_fixture
printf '%s\n' staleold01 staleold02 >"$instances"
list_mode=malformed
if wahrwelt_shell_transition_begin; then
  fail 'malformed instance JSON activated capture'
fi
[ ! -s "$instances" ] || fail 'malformed-list fallback did not clean every exact-config instance'
[ -s "$unrelated_state" ] || fail 'malformed-list cleanup removed unrelated QuickShell state'

reset_fixture
list_mode=fail-after-launch
if wahrwelt_shell_transition_begin; then
  fail 'post-launch instance-list failure activated capture'
fi
[ ! -s "$instances" ] || fail 'post-launch list failure retained a transition instance'
[ -s "$unrelated_state" ] || fail 'post-launch list failure removed unrelated QuickShell state'

reset_fixture
launch_mode=fail-live
if wahrwelt_shell_transition_begin; then
  fail 'nonzero launch that left a daemon activated capture'
fi
[ ! -s "$instances" ] || fail 'nonzero launch retained an exact transition daemon'

reset_fixture
launch_mode=late-register
if wahrwelt_shell_transition_begin; then
  fail 'timed-out launch with delayed registration activated capture'
fi
command sleep 0.35
if [ -s "$instances" ]; then
  fail 'timed-out detached launch registered after cleanup completed'
fi
: >"$instances"

if grep -q 'EPOCHREALTIME' "$helper"; then
  fail 'transition deadlines still depend on mutable wall clock EPOCHREALTIME'
else
  reset_fixture
  status_mode=unavailable
  launch_advance_us=0
  if wahrwelt_shell_transition_begin; then
    fail 'permanently unavailable capture status activated overlay'
  fi
  assert_eq 2000000 "$(cat "$clock_file")" \
    'monotonic capture deadline was not exactly 1000ms including launch'
  grep -Fqx forward "$wall_jumps" || fail 'capture timing did not simulate a forward wall jump'
  grep -Fqx backward "$wall_jumps" || fail 'capture timing did not simulate a backward wall jump'
  if grep -Eq $'^qs\t(-n\t)?-d\t-c\twahrwelt-shell-transition$' "$operations"; then
    fail 'transition launch detached from the runtime lock process group'
  fi

  reset_fixture
  printf '%s\n' malformed >"$uptime_file"
  if wahrwelt_shell_transition_begin; then
    fail 'malformed monotonic source activated capture'
  fi
  [ ! -s "$instances" ] || fail 'malformed monotonic source retained exact transition instances'
fi

reset_fixture
status_mode=start-outgoing
wahrwelt_shell_transition_begin || fail 'readiness fixture did not capture'
layers_mode=ready
wahrwelt_shell_transition_wait_target_ready caelestia ||
  fail 'exact target PID layer readiness rejected every output'
wahrwelt_shell_transition_abort

# A missing exact start call leaves the old desktop frozen instead of starting
# the visible ten-second transition immediately after capture.
reset_fixture
status_mode=capture-delay-then-outgoing
wahrwelt_shell_transition_begin || fail 'outgoing fixture did not launch and capture'
transition_start_us="$(cat "$start_marker")"
assert_eq 1250000 "$transition_start_us" \
  'deadline fixture did not separate capture time from transition start'
grep -Fqx $'qs\tipc\t-i\townednew01\tcall\tshellTransition\tstart' "$operations" ||
  fail 'begin did not send exact owned-instance start IPC'
assert_eq outgoing "$(wahrwelt_shell_transition_status 2>/dev/null || true)" \
  'begin did not leave the overlay in outgoing state'
assert_eq "$((transition_start_us + 10000000))" \
  "${wahrwelt_shell_transition_visible_deadline_us:-unset}" \
  'visible deadline was not anchored once at transition start plus 10 seconds'
assert_eq "$((transition_start_us + 10750000))" \
  "${wahrwelt_shell_transition_cleanup_deadline_us:-unset}" \
  'cleanup deadline was not anchored once at transition start plus 10.75 seconds'
wahrwelt_shell_transition_abort
assert_eq '' "${wahrwelt_shell_transition_visible_deadline_us:-}" \
  'abort retained the visible deadline'
assert_eq '' "${wahrwelt_shell_transition_cleanup_deadline_us:-}" \
  'abort retained the cleanup deadline'

reset_fixture
status_mode=start-outgoing
start_mode=failure
if wahrwelt_shell_transition_begin; then
  fail 'failed exact start IPC activated the transition'
fi
[ ! -s "$instances" ] || fail 'failed exact start IPC retained the owned instance'
assert_eq '' "${wahrwelt_shell_transition_visible_deadline_us:-}" \
  'failed exact start IPC retained the visible deadline'
assert_eq '' "${wahrwelt_shell_transition_cleanup_deadline_us:-}" \
  'failed exact start IPC retained the cleanup deadline'

reset_fixture
status_mode=captured
if wahrwelt_shell_transition_begin; then
  fail 'begin accepted captured after successful start IPC instead of outgoing'
fi
[ ! -s "$instances" ] || fail 'non-outgoing start retained the owned instance'
assert_eq '' "${wahrwelt_shell_transition_visible_deadline_us:-}" \
  'non-outgoing start retained the visible deadline'

if ! declare -F wahrwelt_shell_transition_wait_covered >/dev/null; then
  fail 'wait_covered protocol function is missing'
else
  reset_fixture
  status_mode=start-to-covered
  wahrwelt_shell_transition_begin || fail 'covered fixture did not start outgoing'
  wahrwelt_shell_transition_wait_covered ||
    fail 'wait_covered rejected the exact covered state'
  assert_eq covered "$(wahrwelt_shell_transition_status)" \
    'wait_covered returned before the overlay reached covered'
  wahrwelt_shell_transition_abort

  reset_fixture
  status_mode=start-transient-then-covered
  wahrwelt_shell_transition_begin || fail 'transient IPC fixture did not start outgoing'
  wahrwelt_shell_transition_wait_covered ||
    fail 'wait_covered aborted after one transient outgoing IPC failure'
  assert_eq covered "$(wahrwelt_shell_transition_status)" \
    'wait_covered did not recover to the exact covered state'
  [ -s "$instances" ] || fail 'transient outgoing IPC failure killed the owned instance'
  wahrwelt_shell_transition_abort

  reset_fixture
  status_mode=start-covered-after-three-seconds
  wahrwelt_shell_transition_begin || fail 'post-animation cover fixture did not start outgoing'
  wahrwelt_shell_transition_wait_covered ||
    fail 'wait_covered gave the presentation fence no grace after the three-second animation'
  assert_eq covered "$(wahrwelt_shell_transition_status)" \
    'post-animation presentation grace returned before covered'
  wahrwelt_shell_transition_abort

  reset_fixture
  status_mode=start-outgoing-then-incoming
  wahrwelt_shell_transition_begin || fail 'premature incoming fixture did not start'
  if wahrwelt_shell_transition_wait_covered; then
    fail 'wait_covered accepted incoming without observing covered'
  fi
  [ ! -s "$instances" ] || fail 'invalid wait_covered state retained the owned instance'
fi

# Layer readiness shares the original visible deadline. Re-arming a private
# readiness budget here would push the complete swap beyond ten seconds.
reset_fixture
status_mode=start-outgoing
wahrwelt_shell_transition_begin || fail 'anchored readiness fixture did not start'
anchored_visible_deadline="${wahrwelt_shell_transition_visible_deadline_us:-unset}"
anchored_incoming_deadline=$((anchored_visible_deadline - 3000000))
layers_mode=partial
advance_clock_us 6850000
if wahrwelt_shell_transition_wait_target_ready caelestia; then
  fail 'permanently partial layers unexpectedly became ready'
fi
assert_eq "$anchored_visible_deadline" \
  "${wahrwelt_shell_transition_visible_deadline_us:-unset}" \
  'readiness re-armed the visible deadline'
assert_eq "$anchored_incoming_deadline" "$(cat "$clock_file")" \
  'readiness did not stop at the incoming phase boundary'
wahrwelt_shell_transition_abort

if ! declare -F wahrwelt_shell_transition_bridge_budget_available >/dev/null; then
  fail 'bridge_budget_available protocol function is missing'
else
  reset_fixture
  status_mode=start-outgoing
  wahrwelt_shell_transition_begin || fail 'bridge budget fixture did not start'
  bridge_start_us="$(cat "$start_marker")"
  bridge_incoming_boundary=$((bridge_start_us + 7000000))
  advance_clock_us 6499999
  if ! wahrwelt_shell_transition_bridge_budget_available 500000; then
    fail 'bridge budget rejected a minimum ending one microsecond before incoming'
  fi
  if wahrwelt_shell_transition_bridge_budget_available 500001; then
    fail 'bridge budget accepted a minimum ending exactly at incoming'
  fi
  advance_clock_us 500000
  if ! wahrwelt_shell_transition_bridge_budget_available; then
    fail 'zero bridge budget was rejected one microsecond before incoming'
  fi
  advance_clock_us 1
  assert_eq "$bridge_incoming_boundary" "$(cat "$clock_file")" \
    'bridge budget fixture missed the exact incoming boundary'
  if wahrwelt_shell_transition_bridge_budget_available; then
    fail 'zero bridge budget was accepted at the exact incoming boundary'
  fi
  if wahrwelt_shell_transition_bridge_budget_available invalid; then
    fail 'bridge budget accepted a malformed minimum while active'
  fi
  assert_eq 1 "$wahrwelt_shell_transition_active" \
    'bridge budget failure deactivated the transition'
  assert_eq "$((bridge_start_us + 10000000))" \
    "$wahrwelt_shell_transition_visible_deadline_us" \
    'bridge budget check changed the visible deadline'
  [ -s "$instances" ] || fail 'bridge budget failure cleaned the owned instance'
  wahrwelt_shell_transition_abort
  wahrwelt_shell_transition_bridge_budget_available ||
    fail 'bridge budget rejected an inactive transition'
fi

if ! declare -F wahrwelt_shell_transition_wait_done >/dev/null; then
  fail 'wait_done protocol function is missing'
else
  reset_fixture
  status_mode=timeline
  wahrwelt_shell_transition_begin || fail 'complete timeline fixture did not start'
  timeline_start_us="$(cat "$start_marker")"
  advance_clock_us 6900000
  wahrwelt_shell_transition_wait_done || fail 'ten-second transition did not reach done'
  assert_eq "$((timeline_start_us + 10000000))" "$(cat "$clock_file")" \
    'wait_done did not use the start-anchored ten-second timeline'
  assert_eq 0 "$wahrwelt_shell_transition_active" \
    'completed transition remained active'
  assert_eq '' "${wahrwelt_shell_transition_visible_deadline_us:-}" \
    'completed transition retained the visible deadline'
  assert_eq '' "${wahrwelt_shell_transition_cleanup_deadline_us:-}" \
    'completed transition retained the cleanup deadline'
  [ ! -s "$instances" ] || fail 'completed transition retained the owned instance'
  if grep -Fqx $'qs\tipc\t-i\townednew01\tcall\tshellTransition\tabort' "$operations"; then
    fail 'normal completion sent abort IPC'
  fi
  grep -Fqx $'qs\tkill\t-i\townednew01' "$operations" ||
    fail 'normal completion did not exact-kill the lingering owned instance'

  reset_fixture
  status_mode=start-outgoing
  wahrwelt_shell_transition_begin || fail 'stuck timeline fixture did not start'
  anchored_cleanup_deadline="${wahrwelt_shell_transition_cleanup_deadline_us:-unset}"
  advance_clock_us 10600000
  if wahrwelt_shell_transition_wait_done; then
    fail 'permanently outgoing transition unexpectedly completed'
  fi
  assert_eq "$anchored_cleanup_deadline" "$(cat "$clock_file")" \
    'wait_done re-armed instead of consuming the start-anchored cleanup deadline'
  assert_eq '' "${wahrwelt_shell_transition_cleanup_deadline_us:-}" \
    'timed-out transition retained the cleanup deadline'

  reset_fixture
  status_mode=start-outgoing-then-unavailable
  wahrwelt_shell_transition_begin || fail 'early status failure fixture did not start'
  if wahrwelt_shell_transition_wait_done; then
    fail 'early overlay disappearance was accepted as completion'
  fi
  [ ! -s "$instances" ] || fail 'early overlay disappearance retained the owned instance'
  grep -Fqx $'qs\tipc\t-i\townednew01\tcall\tshellTransition\tabort' "$operations" ||
    fail 'early overlay disappearance did not exact-abort the owned instance'
  grep -Fqx $'qs\tkill\t-i\townednew01' "$operations" ||
    fail 'early overlay disappearance did not exact-kill the owned instance'

  reset_fixture
  status_mode=start-outgoing-then-unavailable
  wahrwelt_shell_transition_begin || fail 'late status failure fixture did not start'
  advance_clock_us 10000000
  if wahrwelt_shell_transition_wait_done; then
    fail 'late overlay disappearance was accepted without explicit done'
  fi
  [ ! -s "$instances" ] || fail 'late overlay disappearance retained the owned instance'
  grep -Fqx $'qs\tipc\t-i\townednew01\tcall\tshellTransition\tabort' "$operations" ||
    fail 'late overlay disappearance did not exact-abort the owned instance'
  grep -Fqx $'qs\tkill\t-i\townednew01' "$operations" ||
    fail 'late overlay disappearance did not exact-kill the owned instance'
fi

reset_fixture
wahrwelt_shell_transition_begin || fail 'status vocabulary fixture did not capture'
for allowed_status in capturing captured outgoing covered incoming settling 'done' aborted; do
  status_mode=fixed
  forced_status="$allowed_status"
  assert_eq "$allowed_status" \
    "$(wahrwelt_shell_transition_status 2>/dev/null || true)" \
    "status rejected protocol state $allowed_status"
done
wahrwelt_shell_transition_abort

reset_fixture
wahrwelt_shell_transition_begin || fail 'signal-safe exact cleanup fixture did not capture'
wahrwelt_shell_transition_abort_signal_safe
[ ! -s "$instances" ] || fail 'signal-safe exact cleanup retained the owned instance'
assert_eq '' "${wahrwelt_shell_transition_visible_deadline_us:-}" \
  'signal-safe cleanup retained the visible deadline'
assert_eq '' "${wahrwelt_shell_transition_cleanup_deadline_us:-}" \
  'signal-safe cleanup retained the cleanup deadline'
grep -Fqx $'qs\tipc\t-i\townednew01\tcall\tshellTransition\tabort' "$operations" ||
  fail 'signal-safe cleanup did not exact-abort the discovered instance'
grep -Fqx $'qs\tkill\t-i\townednew01' "$operations" ||
  fail 'signal-safe cleanup did not exact-kill the discovered instance'
assert_eq 2 "$(grep -Fxc $'qs\t-c\twahrwelt-shell-transition\tkill' "$operations")" \
  'signal-safe cleanup config fallback count'
[ -s "$unrelated_state" ] || fail 'signal-safe cleanup removed unrelated QuickShell state'

reset_fixture
printf '%s\n' raceunknown01 raceunknown02 >"$instances"
wahrwelt_shell_transition_instance_id=
wahrwelt_shell_transition_abort_signal_safe
[ ! -s "$instances" ] || fail 'signal-safe pre-discovery cleanup retained transition instances'
assert_eq 2 "$(grep -Fxc $'qs\t-c\twahrwelt-shell-transition\tkill' "$operations")" \
  'signal-safe pre-discovery cleanup bounded fallback count'
[ -s "$unrelated_state" ] || fail 'signal-safe pre-discovery cleanup removed unrelated state'

command sleep 30 &
owned_launcher_pid=$!
wahrwelt_shell_transition_launcher_pid="$owned_launcher_pid"
wahrwelt_shell_transition_stop_launcher
if kill -0 "$owned_launcher_pid" 2>/dev/null; then
  fail 'owned foreground transition launcher survived exact cleanup'
fi

if grep $'^qs\t' "$operations" |
  grep -Ev $'^qs\t(-c\twahrwelt-shell-transition(\t(list\t-j|kill))?|kill\t-i\t[a-z0-9]{1,64}|ipc\t-i\t[a-z0-9]{1,64}\tcall\tshellTransition\t(status|start|abort))$' |
  grep -q .; then
  fail "transition helper issued a non-exact QuickShell command
$(cat "$operations")"
fi

[ "$failures" -eq 0 ] || exit 1
printf 'OK shell transition overlay helper\n'
