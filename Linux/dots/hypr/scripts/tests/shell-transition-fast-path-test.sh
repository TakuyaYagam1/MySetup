#!/usr/bin/env bash
set -euo pipefail

source_scripts="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
test_root="$(mktemp -d)"
trap 'rm -rf -- "$test_root"' EXIT

instrumented_scripts="$test_root/scripts"
hooks="$test_root/hooks.sh"
operations="$test_root/operations"
active_profile_file="$test_root/active-profile"
fake_bin="$test_root/bin"
home_dir="$test_root/home"
runtime_dir="$test_root/runtime"
state_dir="$home_dir/.local/state/wahrwelt"
hypr_dir="$home_dir/.config/hypr"
hypr_runtime_dir="$state_dir/hypr-runtime"

mkdir -p \
  "$instrumented_scripts" \
  "$fake_bin" \
  "$runtime_dir" \
  "$hypr_dir/user" \
  "$hypr_dir/scripts" \
  "$hypr_dir/caelestia" \
  "$hypr_dir/noctalia" \
  "$state_dir" \
  "$hypr_runtime_dir"
chmod 0700 "$runtime_dir"

for helper in shell-runtime.sh shell-runtime-env.sh shell-profile-sync.sh shell-process.sh; do
  ln -s -- "$source_scripts/$helper" "$instrumented_scripts/$helper"
done
ln -s -- "$source_scripts/start-shell.sh" "$hypr_dir/scripts/start-shell.sh"

printf '%s\n' '-- canonical user entrypoint fixture' >"$hypr_dir/user/hyprland.lua"
for profile in caelestia noctalia; do
  printf '%s\n' "-- $profile launcher fixture" >"$hypr_dir/$profile/launcher.lua"
  printf '%s\n' "-- $profile keybind fixture" >"$hypr_dir/$profile/keybinds.lua"
  printf '%s\n' '#!/usr/bin/env bash' 'exit 0' >"$fake_bin/$profile"
  chmod 0755 "$fake_bin/$profile"
done

cat >"$hooks" <<'EOF'
WAHRWELT_TRANSITION_FAST_PATH_TEST_HOOKS=1

test_record_operation() {
  [ "${WAHRWELT_TEST_CAPTURE:-0}" -eq 1 ] || return 0
  printf '%s\n' "$1" >>"$WAHRWELT_TEST_OPERATIONS"
}

eval "$(declare -f wahrwelt_begin_exact_snapshot | sed '1s/^wahrwelt_begin_exact_snapshot /test_real_begin_exact_snapshot /')"
wahrwelt_begin_exact_snapshot() {
  test_record_operation "snapshot-begin:${2:-missing}"
  test_real_begin_exact_snapshot "$@"
}

eval "$(declare -f snapshot_exact_paths | sed '1s/^snapshot_exact_paths /test_real_snapshot_exact_paths /')"
snapshot_exact_paths() {
  local snapshot_name="${1##*/}"
  case "$snapshot_name" in
    .runtime-rollback-*) snapshot_name=.runtime-rollback- ;;
    .state-switch-rollback-*) snapshot_name=.state-switch-rollback- ;;
    .state-rollback-*) snapshot_name=.state-rollback- ;;
  esac
  test_record_operation "snapshot-paths:$snapshot_name:$(($# - 1))"
  test_real_snapshot_exact_paths "$@"
}

eval "$(declare -f snapshot_read_field | sed '1s/^snapshot_read_field /test_real_snapshot_read_field /')"
snapshot_read_field() {
  if [ "${WAHRWELT_TEST_CAPTURE:-0}" -eq 1 ] &&
    [ "${wahrwelt_test_before_shell_stop:-1}" -eq 1 ]; then
    case "${2:-}" in
      *.path) printf 'metadata-path-read:%s:%s\n' "${1##*/}" "$2" >>"$WAHRWELT_TEST_OPERATIONS" ;;
    esac
  fi
  test_real_snapshot_read_field "$@"
}

eval "$(declare -f write_regular_file | sed '1s/^write_regular_file /test_real_write_regular_file /')"
write_regular_file() {
  test_record_operation "write:$1"
  test_real_write_regular_file "$@"
}

wait_for_session() {
  :
}

sleep() {
  :
}

stop_quickshells() {
  wahrwelt_test_before_shell_stop=0
  if [ "${WAHRWELT_TEST_REPORT_TIMING:-0}" -eq 1 ]; then
    printf 'shell-stop-time:%s\n' "$EPOCHREALTIME" >>"$WAHRWELT_TEST_OPERATIONS"
  fi
  test_record_operation shell-stop
}

stop_end4_idle() {
  :
}

start_profile_shell() {
  test_record_operation "shell-start:$profile"
  if [ "${WAHRWELT_TEST_FAIL_START_PROFILE:-}" = "$profile" ]; then
    test_record_operation "shell-start-failed:$profile"
    return 1
  fi
  if [ -n "${WAHRWELT_TEST_ACTIVE_PROFILE_FILE:-}" ]; then
    printf '%s\n' "$profile" >"$WAHRWELT_TEST_ACTIVE_PROFILE_FILE"
  fi
  if [ "${WAHRWELT_TEST_REPLACE_STATE_AFTER_START_PROFILE:-}" = "$profile" ]; then
    replacement="${persistent_state_file}.concurrent-$BASHPID"
    printf '%s\n' "${WAHRWELT_TEST_CONCURRENT_STATE_VALUE:-concurrent-winner}" >"$replacement"
    mv -T -- "$replacement" "$persistent_state_file"
    test_record_operation "state-replaced-after-start:$profile"
  fi
}

reload_hypr() {
  test_record_operation hypr-reload
}

propagate_runtime_environment() {
  :
}
EOF

awk -v hooks="$hooks" '
  $0 == "log \"requested profile=$profile input=${requested_profile:-auto}\"" {
    while ((getline hook_line < hooks) > 0) {
      print hook_line
    }
    close(hooks)
  }
  { print }
' "$source_scripts/start-shell.sh" >"$instrumented_scripts/start-shell.sh"

if [ "$(grep -Fc 'WAHRWELT_TRANSITION_FAST_PATH_TEST_HOOKS=1' "$instrumented_scripts/start-shell.sh")" -ne 1 ]; then
  printf 'FAIL: could not inject fast-path hooks into start-shell.sh\n' >&2
  exit 1
fi
chmod 0755 "$instrumented_scripts/start-shell.sh"

failures=0

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  failures=$((failures + 1))
}

run_switch_status() {
  local capture="$1"
  local profile="$2"

  HOME="$home_dir" \
    XDG_CONFIG_HOME="$home_dir/.config" \
    XDG_STATE_HOME="$home_dir/.local/state" \
    XDG_RUNTIME_DIR="$runtime_dir" \
    PATH="$fake_bin:$PATH" \
    WAHRWELT_TEST_CAPTURE="$capture" \
    WAHRWELT_TEST_OPERATIONS="$operations" \
    WAHRWELT_TEST_ACTIVE_PROFILE_FILE="$active_profile_file" \
    WAHRWELT_TEST_FAIL_START_PROFILE="${WAHRWELT_TEST_FAIL_START_PROFILE:-}" \
    WAHRWELT_TEST_REPLACE_STATE_AFTER_START_PROFILE="${WAHRWELT_TEST_REPLACE_STATE_AFTER_START_PROFILE:-}" \
    WAHRWELT_TEST_CONCURRENT_STATE_VALUE="${WAHRWELT_TEST_CONCURRENT_STATE_VALUE:-}" \
    "$instrumented_scripts/start-shell.sh" "$profile"
}

run_switch() {
  local capture="$1"
  local profile="$2"

  if run_switch_status "$capture" "$profile"; then
    return 0
  fi
  printf 'FAIL: instrumented switch to %s failed\n' "$profile" >&2
  if [ -f "$runtime_dir/wahrwelt-shell.log" ]; then
    sed 's/^/  log: /' "$runtime_dir/wahrwelt-shell.log" >&2
  fi
  return 1
}

operation_line() {
  local pattern="$1"

  grep -n -m1 -E -- "$pattern" "$operations" | cut -d: -f1 || true
}

# Bootstrap one canonical runtime. Its recovery residue is a baseline only;
# the measured ordinary transition must not add more residue.
: >"$operations"
run_switch 0 caelestia

# Home Manager owns the public stable entrypoint as a current-generation
# symlink on real installations. Exercise that topology on the measured leg.
hm_generation="$test_root/home-manager-generation"
hm_entrypoint="$hm_generation/home-files/.config/hypr/hyprland.lua"
hm_gcroot="$home_dir/.local/state/home-manager/gcroots/current-home"
mkdir -p "$(dirname -- "$hm_entrypoint")" "$(dirname -- "$hm_gcroot")"
mv -- "$hypr_dir/hyprland.lua" "$hm_entrypoint"
ln -s -- "$hm_generation" "$hm_gcroot"
ln -s -- "$hm_entrypoint" "$hypr_dir/hyprland.lua"

: >"$operations"
if [ "${WAHRWELT_TEST_REPORT_TIMING:-0}" -eq 1 ]; then
  printf 'switch-start-time:%s\n' "$EPOCHREALTIME" >>"$operations"
fi
run_switch 1 noctalia

if [ "$(tr -d '[:space:]' <"$state_dir/active-shell")" != noctalia ]; then
  fail 'ordinary transition did not persist the requested Noctalia profile'
fi

expected_writes="$(printf '%s\n' \
  "$hypr_runtime_dir/shell-keybinds.lua" \
  "$hypr_runtime_dir/shell-launcher.lua" \
  "$state_dir/active-shell" | sort)"
actual_writes="$(sed -n 's/^write://p' "$operations" | sort)"
if [ "$actual_writes" != "$expected_writes" ]; then
  fail "ordinary transition publications differ from changed payloads plus active state
got:
$actual_writes
want:
$expected_writes"
fi

expected_snapshots=".runtime-rollback-
.state-switch-rollback-"
actual_snapshots="$(sed -n 's/^snapshot-begin://p' "$operations")"
if [ "$actual_snapshots" != "$expected_snapshots" ]; then
  fail "ordinary transition used nested or unexpected snapshots
got:
$actual_snapshots
want:
$expected_snapshots"
fi

expected_snapshot_paths="snapshot-paths:.runtime-rollback-:2
snapshot-paths:.state-switch-rollback-:1"
actual_snapshot_paths="$(grep '^snapshot-paths:' "$operations" || true)"
if [ "$actual_snapshot_paths" != "$expected_snapshot_paths" ]; then
  fail "ordinary transition snapshotted unchanged or absent runtime paths
got:
$actual_snapshot_paths
want:
$expected_snapshot_paths"
fi

metadata_path_reads="$(grep -c '^metadata-path-read:' "$operations" || true)"
# The fast path should use its anchored in-memory index rather than reopening
# journal path fields before the current shell is stopped.
if [ "$metadata_path_reads" -ne 0 ]; then
  fail "ordinary transition performed $metadata_path_reads pre-stop snapshot path reads, want 0"
fi

runtime_snapshot_line="$(operation_line '^snapshot-begin:\.runtime-rollback-$')"
shell_stop_line="$(operation_line '^shell-stop$')"
target_start_line="$(operation_line '^shell-start:noctalia$')"
state_snapshot_line="$(operation_line '^snapshot-begin:\.state-switch-rollback-$')"
if [ -z "$runtime_snapshot_line" ] || [ -z "$shell_stop_line" ] ||
  [ -z "$target_start_line" ] || [ -z "$state_snapshot_line" ] ||
  [ "$runtime_snapshot_line" -ge "$shell_stop_line" ] ||
  [ "$shell_stop_line" -ge "$target_start_line" ] ||
  [ "$target_start_line" -ge "$state_snapshot_line" ]; then
  fail "ordinary transition order is not runtime snapshot < stop < target start < deferred state snapshot
$(grep -E '^(snapshot-begin|shell-stop|shell-start):?' "$operations" || true)"
fi

if [ "${WAHRWELT_TEST_REPORT_TIMING:-0}" -eq 1 ]; then
  awk -F: '
    /^switch-start-time:/ { started = $2 }
    /^shell-stop-time:/ { stopped = $2 }
    END {
      if (started != "" && stopped != "") {
        printf "metric pre-stop=%.3fs\n", stopped - started
      }
    }
  ' "$operations"
fi

: >"$operations"
run_switch 1 noctalia
unexpected_same_profile_work="$(grep -E '^(snapshot-begin|snapshot-paths|write):' "$operations" || true)"
if [ -n "$unexpected_same_profile_work" ]; then
  fail "same-profile restart performed runtime or state publication work
$unexpected_same_profile_work"
fi

# The persisted previous profile can differ from an already prepared runtime.
# A failed target start must still prepare and restart that previous profile.
fallback_original_launcher="$test_root/fallback-original-shell-launcher.lua"
fallback_original_keybinds="$test_root/fallback-original-shell-keybinds.lua"
cp -p -- "$hypr_runtime_dir/shell-launcher.lua" "$fallback_original_launcher"
cp -p -- "$hypr_runtime_dir/shell-keybinds.lua" "$fallback_original_keybinds"
printf '%s\n' caelestia >"$state_dir/active-shell"
printf '%s\n' caelestia >"$active_profile_file"
: >"$operations"
if WAHRWELT_TEST_FAIL_START_PROFILE=noctalia run_switch_status 1 noctalia; then
  fail 'failed Noctalia start unexpectedly returned success instead of falling back'
fi
if [ "$(tr -d '[:space:]' <"$state_dir/active-shell")" != caelestia ]; then
  fail 'failed target start did not retain the persisted Caelestia fallback state'
fi
if [ "$(tr -d '[:space:]' <"$active_profile_file")" != caelestia ]; then
  fail 'failed target start did not leave the Caelestia fallback active'
fi
if ! grep -Fqx -- "write:$hypr_runtime_dir/shell-launcher.lua" "$operations" ||
  ! grep -Fqx -- "write:$hypr_runtime_dir/shell-keybinds.lua" "$operations"; then
  fail "failed target start did not temporarily publish the Caelestia fallback runtime
$(grep '^write:' "$operations" || true)"
fi
if ! cmp -s -- "$fallback_original_launcher" "$hypr_runtime_dir/shell-launcher.lua" ||
  [ "$(stat -c %a -- "$fallback_original_launcher")" != "$(stat -c %a -- "$hypr_runtime_dir/shell-launcher.lua")" ]; then
  fail 'successful fallback did not restore the exact transaction-begin Noctalia launcher runtime'
fi
if ! cmp -s -- "$fallback_original_keybinds" "$hypr_runtime_dir/shell-keybinds.lua" ||
  [ "$(stat -c %a -- "$fallback_original_keybinds")" != "$(stat -c %a -- "$hypr_runtime_dir/shell-keybinds.lua")" ]; then
  fail 'successful fallback did not restore the exact transaction-begin Noctalia keybind runtime'
fi
failed_target_line="$(operation_line '^shell-start:noctalia$')"
fallback_start_line="$(operation_line '^shell-start:caelestia$')"
fallback_start_count="$(grep -c '^shell-start:caelestia$' "$operations" || true)"
fallback_start_events="$(grep -E '^shell-start:(noctalia|caelestia)$' "$operations" || true)"
if [ -z "$failed_target_line" ] || [ -z "$fallback_start_line" ] ||
  [ "$failed_target_line" -ge "$fallback_start_line" ]; then
  fail "failed target did not start the previous profile as fallback
$(grep -E '^shell-start' "$operations" || true)"
fi
if [ "$fallback_start_count" -ne 1 ] ||
  [ "$fallback_start_events" != $'shell-start:noctalia\nshell-start:caelestia' ]; then
  fail "successful fallback triggered an additional EXIT rollback or shell start
$fallback_start_events"
fi

# Restore the ordinary Noctalia baseline before exercising the deferred-state
# race. The replacement happens after the target start but before persistence.
: >"$operations"
run_switch 0 noctalia
: >"$operations"
if WAHRWELT_TEST_REPLACE_STATE_AFTER_START_PROFILE=caelestia \
  WAHRWELT_TEST_CONCURRENT_STATE_VALUE=concurrent-winner \
  run_switch_status 1 caelestia; then
  fail 'state replacement after target start was committed instead of failing closed'
fi
if [ "$(tr -d '[:space:]' <"$state_dir/active-shell")" != concurrent-winner ]; then
  fail 'state persistence race did not preserve the concurrent winner'
fi
if [ "$(tr -d '[:space:]' <"$active_profile_file")" != noctalia ]; then
  fail 'state persistence race did not restore the previous Noctalia shell'
fi
race_target_start_line="$(operation_line '^shell-start:caelestia$')"
race_replace_line="$(operation_line '^state-replaced-after-start:caelestia$')"
race_state_snapshot_line="$(operation_line '^snapshot-begin:\.state-switch-rollback-$')"
if [ -z "$race_target_start_line" ] || [ -z "$race_replace_line" ] ||
  [ "$race_target_start_line" -ge "$race_replace_line" ] ||
  [ -n "$race_state_snapshot_line" ]; then
  fail "state replacement race was not rejected after target start and before deferred state snapshot
$(grep -E '^(shell-start|state-replaced|snapshot-begin)' "$operations" || true)"
fi

# Function-level planner matrix. Seed an exact canonical runtime for the source
# profile, then verify that only payloads which differ for the target are
# planned. The union planner must add exactly the fallback mutations, without
# duplicates or unrelated runtime paths.
if ! (
  planner_home="$test_root/planner-home"
  export HOME="$planner_home"
  export XDG_CONFIG_HOME="$planner_home/.config"
  export XDG_STATE_HOME="$planner_home/.local/state"
  export XDG_RUNTIME_DIR="$test_root/planner-runtime"
  mkdir -p "$XDG_CONFIG_HOME/hypr" "$XDG_STATE_HOME" "$XDG_RUNTIME_DIR"
  chmod 0700 "$XDG_RUNTIME_DIR"

  # shellcheck source=Linux/dots/hypr/scripts/shell-runtime.sh
  . "$source_scripts/shell-runtime.sh"
  hypr_runtime_dir="$wahrwelt_hypr_runtime_dir"
  hypr_dir() {
    wahrwelt_hypr_dir_path
  }
  mkdir -p "$hypr_runtime_dir" "$(hypr_dir)/scripts"
  # shellcheck source=Linux/dots/hypr/scripts/shell-profile-sync.sh
  . "$source_scripts/shell-profile-sync.sh"

  planner_failures=0
  planner_fail() {
    printf 'FAIL: planner %s\n' "$*" >&2
    planner_failures=$((planner_failures + 1))
  }

  planner_seed_file() {
    local path="$1"
    local content="$2"

    mkdir -p "$(dirname -- "$path")"
    printf '%s\n' "$content" >"$path"
    chmod 0644 "$path"
  }

  planner_seed_runtime() {
    local source_profile="$1"
    local content

    profile="$source_profile"
    content="$(wahrwelt_print_stable_runtime_entrypoint "$hypr_runtime_dir/hyprland.lua")"
    planner_seed_file "$(hypr_dir)/hyprland.lua" "$content"
    content="$(runtime_shell_profile_content)"
    planner_seed_file "$(runtime_file shell-profile.lua)" "$content"
    content="$(wahrwelt_print_canonical_runtime_entrypoint)"
    planner_seed_file "$(runtime_file hyprland.lua)" "$content"
    content="$(runtime_shell_launcher_content)"
    planner_seed_file "$(runtime_file shell-launcher.lua)" "$content"
    content="$(runtime_shell_keybinds_content)"
    planner_seed_file "$(runtime_file shell-keybinds.lua)" "$content"
    content="$(runtime_hyprlock_content)"
    planner_seed_file "$(runtime_file hyprlock.conf)" "$content"
    content="$(runtime_hypridle_content)"
    planner_seed_file "$(runtime_file hypridle.conf)" "$content"
  }

  planner_assert_count() {
    local source_profile="$1"
    local target_profile="$2"
    local expected_count="$3"
    local output actual

    planner_seed_runtime "$source_profile"
    profile="$target_profile"
    output="$(runtime_profile_mutation_paths)"
    actual="$(printf '%s\n' "$output" | sed '/^$/d' | wc -l | tr -d '[:space:]')"
    if [ "$actual" != "$expected_count" ]; then
      planner_fail "$source_profile -> $target_profile planned $actual paths, want $expected_count
$output"
    fi
  }

  planner_assert_count caelestia caelestia 0
  planner_assert_count noctalia noctalia 0
  planner_assert_count end4 end4 0
  planner_assert_count end4-pc end4-pc 0
  planner_assert_count caelestia noctalia 2
  planner_assert_count noctalia caelestia 2
  planner_assert_count end4 end4-pc 1
  planner_assert_count end4-pc end4 1
  for planner_source in caelestia noctalia; do
    for planner_target in end4 end4-pc; do
      planner_assert_count "$planner_source" "$planner_target" 4
      planner_assert_count "$planner_target" "$planner_source" 4
    done
  done

  planner_profiles=(caelestia noctalia end4 end4-pc)
  for planner_target in "${planner_profiles[@]}"; do
    for planner_fallback in "${planner_profiles[@]}"; do
      [ "$planner_target" != "$planner_fallback" ] || continue
      planner_seed_runtime "$planner_target"
      profile="$planner_fallback"
      direct_fallback_paths="$(runtime_profile_mutation_paths | sort)"
      profile="$planner_target"
      previous="$planner_fallback"
      union_paths="$(runtime_switch_bundle_paths | sort)"
      if [ "$union_paths" != "$direct_fallback_paths" ]; then
        planner_fail "union for target=$planner_target fallback=$planner_fallback added or omitted paths
union:
$union_paths
direct fallback:
$direct_fallback_paths"
      fi
    done
  done

  [ "$planner_failures" -eq 0 ]
); then
  fail 'runtime mutation planner matrix failed'
fi

if [ "$failures" -ne 0 ]; then
  exit 1
fi

printf 'OK shell transition fast path\n'
