#!/usr/bin/env bash
# shellcheck disable=SC2016
set -euo pipefail

scripts_dir="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
installer_dir="$(CDPATH='' cd -- "$scripts_dir/../../../installer" && pwd)"
selector_script="$scripts_dir/shell-selector.sh"
runtime_script="$scripts_dir/shell-runtime.sh"
test_root="$(mktemp -d)"
lock_pid=""
trap '[ -z "$lock_pid" ] || kill "$lock_pid" 2>/dev/null || true; rm -rf -- "$test_root"' EXIT

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

wait_for_file() {
  local path="$1"
  local _

  for _ in $(seq 1 200); do
    [ -e "$path" ] && return 0
    sleep 0.01
  done
  fail "timed out waiting for $path"
}

wait_for_event() {
  local pattern="$1"
  local _

  for _ in $(seq 1 200); do
    grep -Eq -- "$pattern" "$events" 2>/dev/null && return 0
    sleep 0.01
  done
  fail "timed out waiting for event $pattern"
}

home="$test_root/home"
runtime="$test_root/runtime"
fake_bin="$test_root/bin"
events="$test_root/events"
ready="$test_root/lock-ready"
release="$test_root/lock-release"
fs_helper="$test_root/wahrwelt-fs-helper"
error_helper="$test_root/wahrwelt-fs-helper-error"
handoff_helper="$test_root/wahrwelt-fs-helper-handoff"
handoff_count="$test_root/handoff-count"
handoff_ready="$test_root/handoff-ready"
handoff_release="$test_root/handoff-release"

mkdir -m 0700 -- "$home" "$runtime"
mkdir -p -- "$home/.config/hypr/scripts" "$home/.local/state" "$fake_bin"
: >"$events"

(cd "$installer_dir" && go build -o "$fs_helper" ./cmd/wahrwelt-fs-helper)

printf '%s\n' \
  '#!/usr/bin/env bash' \
  'set -euo pipefail' \
  '. "${WAHRWELT_TEST_RUNTIME_SCRIPT:?}"' \
  'wahrwelt_enter_runtime_lock_v2 wahrwelt-shell-v2.lock 1500 1 "$0" "$@"' \
  'printf "start:%s\n" "${1:-}" >>"${WAHRWELT_TEST_SELECTOR_EVENTS:?}"' \
  >"$home/.config/hypr/scripts/start-shell.sh"
printf '%s\n' \
  '#!/usr/bin/env bash' \
  'set -euo pipefail' \
  'printf "qs:%s\n" "$*" >>"${WAHRWELT_TEST_SELECTOR_EVENTS:?}"' \
  >"$fake_bin/qs"
printf '%s\n' \
  '#!/usr/bin/env bash' \
  'set -euo pipefail' \
  'if grep -Eq "^qs:" "${WAHRWELT_TEST_SELECTOR_EVENTS:?}" 2>/dev/null; then exit 0; fi' \
  'exit 1' \
  >"$fake_bin/pgrep"
printf '%s\n' \
  '#!/usr/bin/env bash' \
  'set -euo pipefail' \
  'printf "pkill:%s\n" "$*" >>"${WAHRWELT_TEST_SELECTOR_EVENTS:?}"' \
  >"$fake_bin/pkill"
printf '%s\n' \
  '#!/usr/bin/env bash' \
  'exit 42' \
  >"$error_helper"
printf '%s\n' \
  '#!/usr/bin/env bash' \
  'set -euo pipefail' \
  'name=""' \
  'previous=""' \
  'for argument in "$@"; do' \
  '  if [ "$previous" = --name ]; then name="$argument"; break; fi' \
  '  previous="$argument"' \
  'done' \
  'if [ "$name" = wahrwelt-shell-v2.lock ]; then' \
  '  count=0' \
  '  [ ! -f "$WAHRWELT_TEST_HANDOFF_COUNT" ] || read -r count <"$WAHRWELT_TEST_HANDOFF_COUNT"' \
  '  count=$((count + 1))' \
  '  printf "%s\n" "$count" >"$WAHRWELT_TEST_HANDOFF_COUNT"' \
  '  if [ "$count" -eq 2 ]; then' \
  '    : >"$WAHRWELT_TEST_HANDOFF_READY"' \
  '    while [ ! -e "$WAHRWELT_TEST_HANDOFF_RELEASE" ]; do sleep 0.01; done' \
  '  fi' \
  'fi' \
  'exec "${WAHRWELT_REAL_FS_HELPER:?}" "$@"' \
  >"$handoff_helper"
chmod 0755 -- \
  "$home/.config/hypr/scripts/start-shell.sh" \
  "$fake_bin/qs" "$fake_bin/pgrep" "$fake_bin/pkill" \
  "$error_helper" "$handoff_helper"

run_selector() {
  local helper="${WAHRWELT_TEST_SELECTOR_HELPER:-$fs_helper}"

  HOME="$home" \
    XDG_CONFIG_HOME="$home/.config" \
    XDG_STATE_HOME="$home/.local/state" \
    XDG_RUNTIME_DIR="$runtime" \
    WAHRWELT_FS_HELPER="$helper" \
    WAHRWELT_REAL_FS_HELPER="$fs_helper" \
    WAHRWELT_TEST_RUNTIME_SCRIPT="$runtime_script" \
    WAHRWELT_TEST_HANDOFF_COUNT="$handoff_count" \
    WAHRWELT_TEST_HANDOFF_READY="$handoff_ready" \
    WAHRWELT_TEST_HANDOFF_RELEASE="$handoff_release" \
    WAHRWELT_TEST_SELECTOR_EVENTS="$events" \
    PATH="$fake_bin:$PATH" \
    bash "$selector_script" "$@"
}

hold_shell_lock() {
  rm -f -- "$ready" "$release"
  HOME="$home" \
    XDG_RUNTIME_DIR="$runtime" \
    WAHRWELT_TEST_LOCK_READY="$ready" \
    WAHRWELT_TEST_LOCK_RELEASE="$release" \
    "$fs_helper" runtime-lock-run \
    --root "$runtime" \
    --name wahrwelt-shell-v2.lock \
    --wait-ms 0 \
    -- bash -c '
        set -euo pipefail
        : >"$WAHRWELT_TEST_LOCK_READY"
        while [ ! -e "$WAHRWELT_TEST_LOCK_RELEASE" ]; do sleep 0.01; done
      ' &
  lock_pid=$!
  wait_for_file "$ready"
}

release_shell_lock() {
  : >"$release"
  wait "$lock_pid"
  lock_pid=""
}

case "${WAHRWELT_SELECTOR_LOCK_CASE:-all}" in
  busy | all)
    : >"$events"
    hold_shell_lock
    for _ in 1 2 3; do
      run_selector switch noctalia
      run_selector toggle eDP-1
    done
    run_selector close
    sleep 0.1
    if grep -Eq '^(start:|qs:)' "$events"; then
      fail "active shell lock allowed a competing selector action: $(tr '\n' ' ' <"$events")"
    fi
    grep -Eq '^pkill:' "$events" || fail 'active shell lock prevented selector close cleanup'
    release_shell_lock
    ;;
esac

case "${WAHRWELT_SELECTOR_LOCK_CASE:-all}" in
  handoff | all)
    : >"$events"
    rm -f -- "$handoff_count" "$handoff_ready" "$handoff_release"
    WAHRWELT_TEST_SELECTOR_HELPER="$handoff_helper" \
      run_selector switch noctalia >"$test_root/handoff-switch.out" 2>&1 &
    handoff_switch_pid=$!
    wait_for_file "$handoff_ready"
    WAHRWELT_TEST_SELECTOR_HELPER="$handoff_helper" \
      run_selector toggle eDP-1
    handoff_raced=0
    grep -Eq '^qs:' "$events" && handoff_raced=1
    : >"$handoff_release"
    wait "$handoff_switch_pid" || fail 'delayed main-lock switch failed after handoff release'
    wait_for_event '^start:noctalia$'
    [ "$handoff_raced" -eq 0 ] || fail 'selector lock was released before delayed main-lock acquisition'
    ;;
esac

case "${WAHRWELT_SELECTOR_LOCK_CASE:-all}" in
  error | all)
    : >"$events"
    for action in 'switch noctalia' 'toggle eDP-1'; do
      read -r verb argument <<<"$action"
      if HOME="$home" \
        XDG_CONFIG_HOME="$home/.config" \
        XDG_STATE_HOME="$home/.local/state" \
        XDG_RUNTIME_DIR="$runtime" \
        WAHRWELT_FS_HELPER="$error_helper" \
        WAHRWELT_RUNTIME_LOCK_V2=wahrwelt-shell-selector-v2.lock \
        WAHRWELT_RUNTIME_LOCK_V2_ROOT="$runtime" \
        WAHRWELT_TEST_SELECTOR_EVENTS="$events" \
        PATH="$fake_bin:$PATH" \
        bash "$selector_script" "$verb" "$argument"; then
        fail "lock helper failure allowed selector action $action"
      fi
    done
    if grep -Eq '^(start:|qs:)' "$events"; then
      fail "lock helper failure launched a selector action: $(tr '\n' ' ' <"$events")"
    fi
    ;;
esac

case "${WAHRWELT_SELECTOR_LOCK_CASE:-all}" in
  free | all)
    : >"$events"
    run_selector switch noctalia
    wait_for_event '^start:noctalia$'
    : >"$events"
    run_selector toggle eDP-1
    wait_for_event '^qs:-c wahrwelt-shell-selector$'
    ;;
esac

printf 'OK shell selector cross-lock guard\n'
