#!/usr/bin/env bash
# shellcheck disable=SC2016
set -euo pipefail

scripts_dir="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
installer_dir="$(CDPATH='' cd -- "$scripts_dir/../../../installer" && pwd)"
runtime_script="$scripts_dir/shell-runtime.sh"
selector_script="$scripts_dir/shell-selector.sh"
noctalia_launcher="$scripts_dir/noctalia-launcher.sh"
record_toggle="$scripts_dir/record-toggle.sh"
test_root="$(mktemp -d)"
legacy_lock_pid=""
background_pids=""
pid=""
trap 'for pid in $background_pids; do kill "$pid" 2>/dev/null || true; done; [ -z "$legacy_lock_pid" ] || kill "$legacy_lock_pid" 2>/dev/null || true; rm -rf -- "$test_root"' EXIT

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
  fail "timed out waiting for barrier $path"
}

assert_file() {
  local path="$1"
  local expected="$2"
  [ -f "$path" ] || fail "missing file $path"
  [ "$(cat "$path")" = "$expected" ] || fail "unexpected bytes in $path"
}

seed_exact_leaf_marker() {
  local state_dir="$1"
  local leaf="$2"
  local kind="$3"
  local marker="$state_dir/.wahrwelt-owner.$leaf"

  printf 'Wahrwelt managed regular v1\nkind=%s\ninode=%s\n' \
    "$kind" "$(stat -c '%d:%i' -- "$state_dir/$leaf")" >"$marker"
  chmod 0600 -- "$marker"
}

create_state_namespace() {
  local runtime="$1"
  local name="$2"
  local kind="$3"

  HOME="$home" XDG_RUNTIME_DIR="$runtime" bash -c '
    set -euo pipefail
    . "$1"
    wahrwelt_open_private_state_directory "$2" "$3"
  ' bash "$runtime_script" "$name" "$kind"
}

consumer_lock_name() {
  case "${1##*/}" in
    shell-selector.sh) printf '%s' wahrwelt-shell-selector-v2.lock ;;
    noctalia-launcher.sh) printf '%s' wahrwelt-noctalia-launcher-v2.lock ;;
    record-toggle.sh) printf '%s' wahrwelt-record-toggle-v2.lock ;;
    *) return 1 ;;
  esac
}

adopt_legacy_state_namespace() {
  local runtime="$1"
  local name="$2"
  local kind="$3"

  HOME="$home" XDG_RUNTIME_DIR="$runtime" bash -c '
    set -euo pipefail
    . "$1"
    wahrwelt_adopt_legacy_private_state_directory "$2" "$3"
  ' bash "$runtime_script" "$name" "$kind"
}

home="$test_root/home"
mkdir -m 0700 -- "$home"
export WAHRWELT_FS_HELPER="$test_root/wahrwelt-fs-helper"
(cd "$installer_dir" && go build -o "$WAHRWELT_FS_HELPER" ./cmd/wahrwelt-fs-helper)

tmp_target="$test_root/tmp-target"
mkdir -m 0755 -- "$tmp_target"
printf '%s\n' 'tmp target bytes' >"$tmp_target/sentinel"
ln -s -- "$tmp_target" "$test_root/tmp-link"
if env -u XDG_RUNTIME_DIR HOME="$home" TMPDIR="$test_root/tmp-link" \
  bash -c '. "$1"; printf adopted' bash "$runtime_script" >"$test_root/absent-xdg.out" 2>&1; then
  fail 'missing XDG_RUNTIME_DIR was accepted through a TMPDIR symlink'
fi
assert_file "$tmp_target/sentinel" 'tmp target bytes'
[ "$(stat -c %a -- "$tmp_target")" = 755 ] || fail 'hostile TMPDIR target mode changed'

tmp_unknown="$test_root/tmp-unknown"
unknown_fallback="$tmp_unknown/wahrwelt-runtime-$UID"
mkdir -p -- "$unknown_fallback"
chmod 0755 -- "$unknown_fallback"
printf '%s\n' 'unknown fallback bytes' >"$unknown_fallback/sentinel"
if env -u XDG_RUNTIME_DIR HOME="$home" TMPDIR="$tmp_unknown" \
  bash -c '. "$1"' bash "$runtime_script" >"$test_root/unknown-fallback.out" 2>&1; then
  fail 'missing XDG_RUNTIME_DIR adopted an unknown TMPDIR fallback'
fi
assert_file "$unknown_fallback/sentinel" 'unknown fallback bytes'
[ "$(stat -c %a -- "$unknown_fallback")" = 755 ] || fail 'unknown fallback mode changed'

unsafe_target="$test_root/unsafe-target"
mkdir -m 0700 -- "$unsafe_target"
printf '%s\n' 'unsafe target bytes' >"$unsafe_target/sentinel"
ln -s -- "$unsafe_target" "$test_root/unsafe-runtime"
if HOME="$home" XDG_RUNTIME_DIR="$test_root/unsafe-runtime" \
  bash -c '. "$1"' bash "$runtime_script" >"$test_root/unsafe-link.out" 2>&1; then
  fail 'symlink XDG_RUNTIME_DIR was accepted'
fi
assert_file "$unsafe_target/sentinel" 'unsafe target bytes'

for unsafe_spelling in "$test_root/unsafe-runtime/" "$test_root/unsafe-runtime/."; do
  if HOME="$home" XDG_RUNTIME_DIR="$unsafe_spelling" \
    bash -c '. "$1"' bash "$runtime_script" >"$test_root/unsafe-spelling.out" 2>&1; then
    fail "symlink XDG_RUNTIME_DIR bypass was accepted: $unsafe_spelling"
  fi
  assert_file "$unsafe_target/sentinel" 'unsafe target bytes'
done

unsafe_mode="$test_root/unsafe-mode"
mkdir -m 0755 -- "$unsafe_mode"
printf '%s\n' 'unsafe mode bytes' >"$unsafe_mode/sentinel"
if HOME="$home" XDG_RUNTIME_DIR="$unsafe_mode" \
  bash -c '. "$1"' bash "$runtime_script" >"$test_root/unsafe-mode.out" 2>&1; then
  fail 'non-private XDG_RUNTIME_DIR was accepted'
fi
assert_file "$unsafe_mode/sentinel" 'unsafe mode bytes'
[ "$(stat -c %a -- "$unsafe_mode")" = 755 ] || fail 'unsafe XDG runtime mode changed'

runtime="$test_root/runtime"
runtime_saved="$test_root/runtime-before-swap"
mkdir -m 0700 -- "$runtime"
HOME="$home" XDG_RUNTIME_DIR="$runtime" RUNTIME_SAVED="$runtime_saved" \
  bash -c '
    set -euo pipefail
    . "$1"
    case "$wahrwelt_runtime_session_dir" in /proc/*/fd/*) ;; *) exit 91 ;; esac
    mv -- "$XDG_RUNTIME_DIR" "$RUNTIME_SAVED"
    mkdir -m 0700 -- "$XDG_RUNTIME_DIR"
    printf "%s\n" "unknown runtime bytes" >"$XDG_RUNTIME_DIR/sentinel"
    printf "%s\n" "pinned log bytes" >"$wahrwelt_log_file"
  ' bash "$runtime_script"
assert_file "$runtime_saved/wahrwelt-shell.log" 'pinned log bytes'
assert_file "$runtime/sentinel" 'unknown runtime bytes'
[ ! -e "$runtime/wahrwelt-shell.log" ] || fail 'post-validation runtime replacement received a write'

for consumer in "$selector_script" "$noctalia_launcher" "$record_toggle"; do
  consumer_name="${consumer##*/}"
  consumer_lock="$(consumer_lock_name "$consumer")"
  runtime="$test_root/runtime-consumer-${consumer_name%.sh}"
  runtime_saved="$test_root/runtime-consumer-${consumer_name%.sh}-original"
  videos="$test_root/videos-${consumer_name%.sh}"
  mkdir -m 0700 -- "$runtime"
  if HOME="$home" XDG_RUNTIME_DIR="$runtime" RUNTIME_SAVED="$runtime_saved" \
    XDG_VIDEOS_DIR="$videos" WAHRWELT_RUNTIME_LOCK_V2="$consumer_lock" \
    WAHRWELT_RUNTIME_LOCK_V2_ROOT="$runtime" \
    bash -c '
      set -euo pipefail
      wahrwelt_after_runtime_directory_pin_hook() {
        mv -- "$1" "$RUNTIME_SAVED"
        mkdir -m 0700 -- "$1"
        printf "%s\n" "consumer runtime winner bytes" >"$1/sentinel"
      }
      command() {
        if [ "${1:-}" = -v ] && [ "${2:-}" = gpu-screen-recorder ]; then
          return 1
        fi
        builtin command "$@"
      }
      notify-send() { :; }
      . "$1" invalid-action
    ' bash "$consumer" >"$test_root/$consumer_name.out" 2>&1; then
    fail "$consumer_name unexpectedly completed during root-swap probe"
  fi
  assert_file "$runtime/sentinel" 'consumer runtime winner bytes'
  [ ! -e "$runtime/wahrwelt-noctalia-launcher" ] || fail "$consumer_name wrote through replaced public runtime root"
  [ ! -e "$runtime/wahrwelt-recording" ] || fail "$consumer_name wrote recording state through replaced public runtime root"
  [ ! -e "$runtime/wahrwelt-shell-selector" ] || fail "$consumer_name wrote selector state through replaced public runtime root"
  case "$consumer_name" in
    shell-selector.sh)
      [ -d "$runtime_saved/wahrwelt-shell-selector" ] || fail 'shell selector did not use pinned runtime root'
      ;;
    noctalia-launcher.sh)
      [ -d "$runtime_saved/wahrwelt-noctalia-launcher" ] || fail 'noctalia launcher did not use pinned runtime root'
      ;;
    record-toggle.sh)
      [ -d "$runtime_saved/wahrwelt-recording" ] || fail 'record toggle did not use pinned runtime root'
      ;;
  esac
done

for consumer_spec in \
  "$selector_script:wahrwelt-shell-selector" \
  "$noctalia_launcher:wahrwelt-noctalia-launcher" \
  "$record_toggle:wahrwelt-recording"; do
  consumer="${consumer_spec%%:*}"
  state_name="${consumer_spec#*:}"
  consumer_name="${consumer##*/}"
  consumer_lock="$(consumer_lock_name "$consumer")"
  runtime="$test_root/runtime-state-link-${consumer_name%.sh}"
  state_target="$test_root/state-link-target-${consumer_name%.sh}"
  videos="$test_root/videos-state-link-${consumer_name%.sh}"
  mkdir -m 0700 -- "$runtime" "$state_target"
  printf '%s\n' 'state directory target bytes' >"$state_target/sentinel"
  ln -s -- "$state_target" "$runtime/$state_name"
  if HOME="$home" XDG_RUNTIME_DIR="$runtime" XDG_VIDEOS_DIR="$videos" \
    WAHRWELT_RUNTIME_LOCK_V2="$consumer_lock" WAHRWELT_RUNTIME_LOCK_V2_ROOT="$runtime" \
    bash -c '
      set -euo pipefail
      command() {
        if [ "${1:-}" = -v ] && [ "${2:-}" = gpu-screen-recorder ]; then return 1; fi
        builtin command "$@"
      }
      notify-send() { :; }
      . "$1" invalid-action
    ' bash "$consumer" >"$test_root/state-link-$consumer_name.out" 2>&1; then
    fail "$consumer_name adopted a symlink state directory"
  fi
  [ -L "$runtime/$state_name" ] || fail "$consumer_name changed the state directory symlink"
  assert_file "$state_target/sentinel" 'state directory target bytes'
done

runtime="$test_root/runtime-unsafe-legacy-log"
unknown_target="$runtime/wahrwelt-shell.log"
mkdir -m 0700 -- "$runtime"
printf '%s\n' 'unsafe legacy log bytes' >"$unknown_target"
chmod 0660 -- "$unknown_target"
if HOME="$home" XDG_RUNTIME_DIR="$runtime" \
  bash -c '. "$1"' bash "$runtime_script" >"$test_root/unsafe-legacy-log.out" 2>&1; then
  fail 'group-writable legacy managed log was adopted'
fi
assert_file "$unknown_target" 'unsafe legacy log bytes'
[ ! -e "$runtime/.wahrwelt-owner.wahrwelt-shell.log" ] || fail 'ownership marker was added beside an unsafe log'

for consumer_spec in \
  "$selector_script:wahrwelt-shell-selector" \
  "$noctalia_launcher:wahrwelt-noctalia-launcher" \
  "$record_toggle:wahrwelt-recording"; do
  consumer="${consumer_spec%%:*}"
  state_name="${consumer_spec#*:}"
  consumer_name="${consumer##*/}"
  consumer_lock="$(consumer_lock_name "$consumer")"
  runtime="$test_root/runtime-state-ordinary-${consumer_name%.sh}"
  state_dir="$runtime/$state_name"
  videos="$test_root/videos-state-ordinary-${consumer_name%.sh}"
  mkdir -m 0700 -- "$runtime" "$state_dir"
  printf '%s\n' 'ordinary unknown state bytes' >"$state_dir/sentinel"
  if HOME="$home" XDG_RUNTIME_DIR="$runtime" XDG_VIDEOS_DIR="$videos" \
    WAHRWELT_RUNTIME_LOCK_V2="$consumer_lock" WAHRWELT_RUNTIME_LOCK_V2_ROOT="$runtime" \
    bash -c '
      command() {
        if [ "${1:-}" = -v ] && [ "${2:-}" = gpu-screen-recorder ]; then return 1; fi
        builtin command "$@"
      }
      notify-send() { :; }
      . "$1" invalid-action
    ' bash "$consumer" >"$test_root/state-ordinary-$consumer_name.out" 2>&1; then
    fail "$consumer_name adopted an unmarked ordinary state directory"
  fi
  assert_file "$state_dir/sentinel" 'ordinary unknown state bytes'
  [ ! -e "$state_dir/.wahrwelt-state-owner" ] || fail "$consumer_name marked an unknown state directory"
done

runtime="$test_root/runtime-legacy-log"
legacy_log="$runtime/wahrwelt-shell.log"
mkdir -m 0700 -- "$runtime"
printf '%s\n' 'legacy shell log payload' >"$legacy_log"
chmod 0644 -- "$legacy_log"
HOME="$home" XDG_RUNTIME_DIR="$runtime" \
  bash -c '. "$1"' bash "$runtime_script"
assert_file "$legacy_log" 'legacy shell log payload'
[ "$(stat -c %a -- "$legacy_log")" = 644 ] || fail 'legacy shell log mode changed during adoption'
[ "$(stat -c %a -- "$runtime/.wahrwelt-owner.wahrwelt-shell.log")" = 600 ] ||
  fail 'legacy shell log ownership marker was not created privately'
legacy_log_inode="$(stat -c '%d:%i' -- "$legacy_log")"
legacy_log_marker_inode="$(stat -c '%d:%i' -- "$runtime/.wahrwelt-owner.wahrwelt-shell.log")"
HOME="$home" XDG_RUNTIME_DIR="$runtime" bash -c '. "$1"' bash "$runtime_script"
[ "$(stat -c '%d:%i' -- "$legacy_log")" = "$legacy_log_inode" ] ||
  fail 'legacy shell log retry changed the payload inode'
[ "$(stat -c '%d:%i' -- "$runtime/.wahrwelt-owner.wahrwelt-shell.log")" = "$legacy_log_marker_inode" ] ||
  fail 'legacy shell log retry changed the exact committed marker'

runtime="$test_root/runtime-legacy-log-marker-replacement"
legacy_log="$runtime/wahrwelt-shell.log"
mkdir -m 0700 -- "$runtime"
printf '%s\n' 'legacy marker replacement payload' >"$legacy_log"
chmod 0644 -- "$legacy_log"
printf '%s\n' 'unknown replacement marker' >"$runtime/.wahrwelt-owner.wahrwelt-shell.log"
chmod 0600 -- "$runtime/.wahrwelt-owner.wahrwelt-shell.log"
replacement_marker_inode="$(stat -c '%d:%i' -- "$runtime/.wahrwelt-owner.wahrwelt-shell.log")"
if HOME="$home" XDG_RUNTIME_DIR="$runtime" bash -c '. "$1"' bash "$runtime_script" \
  >"$test_root/legacy-log-marker-replacement.out" 2>&1; then
  fail 'legacy shell log accepted an unknown replacement marker'
fi
assert_file "$legacy_log" 'legacy marker replacement payload'
assert_file "$runtime/.wahrwelt-owner.wahrwelt-shell.log" 'unknown replacement marker'
[ "$(stat -c '%d:%i' -- "$runtime/.wahrwelt-owner.wahrwelt-shell.log")" = "$replacement_marker_inode" ] ||
  fail 'legacy shell log cleanup replaced or removed an unknown marker inode'

runtime="$test_root/runtime-legacy-log-hardlink"
legacy_log="$runtime/wahrwelt-shell.log"
legacy_log_outside="$test_root/legacy-log-hardlink-outside"
mkdir -m 0700 -- "$runtime"
printf '%s\n' 'legacy hardlink payload' >"$legacy_log_outside"
chmod 0644 -- "$legacy_log_outside"
ln -- "$legacy_log_outside" "$legacy_log"
if HOME="$home" XDG_RUNTIME_DIR="$runtime" bash -c '. "$1"' bash "$runtime_script" \
  >"$test_root/legacy-log-hardlink.out" 2>&1; then
  fail 'hardlinked legacy shell log was adopted'
fi
assert_file "$legacy_log" 'legacy hardlink payload'
assert_file "$legacy_log_outside" 'legacy hardlink payload'
[ ! -e "$runtime/.wahrwelt-owner.wahrwelt-shell.log" ] || fail 'hardlinked legacy log received an ownership marker'

runtime="$test_root/runtime-legacy-log-race"
legacy_log="$runtime/wahrwelt-shell.log"
legacy_log_saved="$test_root/legacy-log-race-original"
mkdir -m 0700 -- "$runtime"
printf '%s\n' 'legacy race original' >"$legacy_log"
chmod 0644 -- "$legacy_log"
if HOME="$home" XDG_RUNTIME_DIR="$runtime" LEGACY_LOG_SAVED="$legacy_log_saved" bash -c '
  set -euo pipefail
  wahrwelt_after_legacy_managed_regular_preflight_hook() {
    mv -- "$2" "$LEGACY_LOG_SAVED"
    printf "%s\n" "legacy race winner" >"$2"
    chmod 0644 -- "$2"
  }
  . "$1"
' bash "$runtime_script" >"$test_root/legacy-log-race.out" 2>&1; then
  fail 'legacy shell log replacement after preflight was adopted'
fi
assert_file "$legacy_log_saved" 'legacy race original'
assert_file "$legacy_log" 'legacy race winner'
[ ! -e "$runtime/.wahrwelt-owner.wahrwelt-shell.log" ] || fail 'legacy log race left an ownership marker'

runtime="$test_root/runtime-legacy-noctalia"
state_dir="$runtime/wahrwelt-noctalia-launcher"
mkdir -m 0700 -- "$runtime"
mkdir -m 0755 -- "$state_dir"
: >"$state_dir/active"
: >"$state_dir/interrupted"
chmod 0644 -- "$state_dir/active" "$state_dir/interrupted"
HOME="$home" XDG_RUNTIME_DIR="$runtime" \
  WAHRWELT_RUNTIME_LOCK_V2=wahrwelt-noctalia-launcher-v2.lock WAHRWELT_RUNTIME_LOCK_V2_ROOT="$runtime" \
  bash -c 'notify-send() { :; }; . "$1" press' bash "$noctalia_launcher" \
  >"$test_root/legacy-noctalia-stale-lock.out" 2>&1
[ "$(stat -c %a -- "$state_dir")" = 700 ] || fail 'legacy noctalia state directory was not made private'
assert_file "$state_dir/active" 1
assert_file "$state_dir/interrupted" 0
for marker in \
  .wahrwelt-state-owner \
  .wahrwelt-owner.active \
  .wahrwelt-owner.interrupted; do
  [ "$(stat -c %a -- "$state_dir/$marker")" = 600 ] ||
    fail "legacy noctalia ownership marker is missing or unsafe: $marker"
done
HOME="$home" XDG_RUNTIME_DIR="$runtime" \
  WAHRWELT_RUNTIME_LOCK_V2=wahrwelt-noctalia-launcher-v2.lock WAHRWELT_RUNTIME_LOCK_V2_ROOT="$runtime" \
  bash -c 'notify-send() { :; }; . "$1" release' bash "$noctalia_launcher" \
  >"$test_root/legacy-noctalia-second-invocation.out" 2>&1 ||
  fail 'second noctalia invocation rejected recoveries from the first managed lock'
assert_file "$state_dir/active" 0
assert_file "$state_dir/interrupted" 0

runtime="$test_root/runtime-legacy-noctalia-malformed"
state_dir="$runtime/wahrwelt-noctalia-launcher"
mkdir -m 0700 -- "$runtime"
mkdir -m 0755 -- "$state_dir"
printf '%s\n' 'not a historical presence marker' >"$state_dir/active"
chmod 0644 -- "$state_dir/active"
if adopt_legacy_state_namespace "$runtime" wahrwelt-noctalia-launcher noctalia-launcher-state \
  >"$test_root/legacy-noctalia-malformed.out" 2>&1; then
  fail 'malformed legacy noctalia presence marker was adopted'
fi
assert_file "$state_dir/active" 'not a historical presence marker'
[ "$(stat -c %a -- "$state_dir")" = 755 ] || fail 'malformed legacy noctalia directory mode changed'
[ ! -e "$state_dir/.wahrwelt-state-owner" ] || fail 'malformed legacy noctalia state received a marker'

for collision in symlink hardlink writable; do
  runtime="$test_root/runtime-legacy-recording-$collision"
  state_dir="$runtime/wahrwelt-recording"
  collision_target="$test_root/legacy-recording-$collision-target"
  mkdir -m 0700 -- "$runtime"
  mkdir -m 0755 -- "$state_dir"
  printf '%s\n' "legacy recording $collision payload" >"$collision_target"
  chmod 0644 -- "$collision_target"
  case "$collision" in
    symlink) ln -s -- "$collision_target" "$state_dir/gpu-screen-recorder.log" ;;
    hardlink) ln -- "$collision_target" "$state_dir/gpu-screen-recorder.log" ;;
    writable)
      mv -- "$collision_target" "$state_dir/gpu-screen-recorder.log"
      chmod 0660 -- "$state_dir/gpu-screen-recorder.log"
      ;;
  esac
  if adopt_legacy_state_namespace "$runtime" wahrwelt-recording record-toggle-state \
    >"$test_root/legacy-recording-$collision.out" 2>&1; then
    fail "unsafe legacy recording $collision leaf was adopted"
  fi
  [ "$(stat -c %a -- "$state_dir")" = 755 ] || fail "unsafe legacy recording $collision directory mode changed"
  [ ! -e "$state_dir/.wahrwelt-state-owner" ] || fail "unsafe legacy recording $collision state received a marker"
  case "$collision" in
    symlink | hardlink) assert_file "$collision_target" "legacy recording $collision payload" ;;
    writable) assert_file "$state_dir/gpu-screen-recorder.log" 'legacy recording writable payload' ;;
  esac
done

runtime="$test_root/runtime-legacy-recording-writable-directory"
state_dir="$runtime/wahrwelt-recording"
mkdir -m 0700 -- "$runtime"
mkdir -m 0770 -- "$state_dir"
if adopt_legacy_state_namespace "$runtime" wahrwelt-recording record-toggle-state \
  >"$test_root/legacy-recording-writable-directory.out" 2>&1; then
  fail 'group-writable legacy recording directory was adopted'
fi
[ "$(stat -c %a -- "$state_dir")" = 770 ] || fail 'unsafe legacy recording directory mode changed'
[ ! -e "$state_dir/.wahrwelt-state-owner" ] || fail 'unsafe legacy recording directory received a marker'

runtime="$test_root/runtime-legacy-noctalia-live-lock"
state_dir="$runtime/wahrwelt-noctalia-launcher"
mkdir -m 0700 -- "$runtime"
mkdir -m 0755 -- "$state_dir" "$state_dir/lock"
bash -c 'while :; do sleep 1; done' /tmp/noctalia-launcher.sh &
legacy_lock_pid=$!
printf '%s\n' "$legacy_lock_pid" >"$state_dir/lock/pid"
printf '%s\n' wahrwelt-noctalia-launcher >"$state_dir/lock/owner"
chmod 0644 -- "$state_dir/lock/pid" "$state_dir/lock/owner"
if adopt_legacy_state_namespace "$runtime" wahrwelt-noctalia-launcher noctalia-launcher-state \
  >"$test_root/legacy-noctalia-live-lock.out" 2>&1; then
  fail 'legacy noctalia state with a live lock owner was adopted'
fi
kill "$legacy_lock_pid" 2>/dev/null || true
wait "$legacy_lock_pid" 2>/dev/null || true
legacy_lock_pid=""
[ "$(stat -c %a -- "$state_dir")" = 755 ] || fail 'live-locked legacy noctalia directory mode changed'
[ ! -e "$state_dir/.wahrwelt-state-owner" ] || fail 'live-locked legacy noctalia state received a marker'

runtime="$test_root/runtime-legacy-noctalia-unknown-lock"
state_dir="$runtime/wahrwelt-noctalia-launcher"
mkdir -m 0700 -- "$runtime"
mkdir -m 0755 -- "$state_dir" "$state_dir/lock"
printf '%s\n' 999999 >"$state_dir/lock/pid"
printf '%s\n' unknown-owner >"$state_dir/lock/owner"
chmod 0644 -- "$state_dir/lock/pid" "$state_dir/lock/owner"
if adopt_legacy_state_namespace "$runtime" wahrwelt-noctalia-launcher noctalia-launcher-state \
  >"$test_root/legacy-noctalia-unknown-lock.out" 2>&1; then
  fail 'legacy noctalia state with an unknown lock was adopted'
fi
[ "$(stat -c %a -- "$state_dir")" = 755 ] || fail 'unknown-lock legacy noctalia directory mode changed'
[ ! -e "$state_dir/.wahrwelt-state-owner" ] || fail 'unknown-lock legacy noctalia state received a marker'

runtime="$test_root/runtime-legacy-noctalia-stale-lock"
state_dir="$runtime/wahrwelt-noctalia-launcher"
mkdir -m 0700 -- "$runtime"
mkdir -m 0755 -- "$state_dir" "$state_dir/lock"
printf '%s\n' 999999 >"$state_dir/lock/pid"
printf '%s\n' wahrwelt-noctalia-launcher >"$state_dir/lock/owner"
chmod 0644 -- "$state_dir/lock/pid" "$state_dir/lock/owner"
HOME="$home" XDG_RUNTIME_DIR="$runtime" \
  WAHRWELT_RUNTIME_LOCK_V2=wahrwelt-noctalia-launcher-v2.lock WAHRWELT_RUNTIME_LOCK_V2_ROOT="$runtime" \
  bash -c 'notify-send() { :; }; . "$1" press' bash "$noctalia_launcher"
[ -f "$state_dir/.wahrwelt-state-owner" ] || fail 'exact stale-lock legacy noctalia state was not adopted'

runtime="$test_root/runtime-legacy-selector-empty"
state_dir="$runtime/wahrwelt-shell-selector"
mkdir -m 0700 -- "$runtime"
mkdir -m 0755 -- "$state_dir"
if HOME="$home" XDG_RUNTIME_DIR="$runtime" \
  WAHRWELT_RUNTIME_LOCK_V2=wahrwelt-shell-selector-v2.lock WAHRWELT_RUNTIME_LOCK_V2_ROOT="$runtime" \
  bash -c '. "$1" invalid-action' bash "$selector_script" \
  >"$test_root/legacy-selector-empty.out" 2>&1; then
  fail 'selector invalid-action unexpectedly succeeded'
fi
if grep -Fq 'ownership collision' "$test_root/legacy-selector-empty.out"; then
  fail 'selector rejected the exact empty legacy state shape'
fi
[ "$(stat -c %a -- "$state_dir")" = 700 ] || fail 'legacy selector state directory was not made private'
[ -f "$state_dir/.wahrwelt-state-owner" ] || fail 'selector consumer did not adopt legacy state ownership'

runtime="$test_root/runtime-private-state-staged-crash"
pending="$runtime/.wahrwelt-state-pending.wahrwelt-shell-selector.000000000000000000000000"
mkdir -m 0700 -- "$runtime" "$pending"
printf 'Wahrwelt private state v1\nkind=shell-selector-state\ninode=%s\n' \
  "$(stat -c '%d:%i' -- "$pending")" >"$pending/.wahrwelt-state-owner"
chmod 0600 -- "$pending/.wahrwelt-state-owner"
pending_inode="$(stat -c '%d:%i' -- "$pending")"
HOME="$home" XDG_RUNTIME_DIR="$runtime" bash -c '
  set -euo pipefail
  . "$1"
  wahrwelt_open_private_state_directory wahrwelt-shell-selector shell-selector-state
' bash "$runtime_script"
[ -d "$runtime/wahrwelt-shell-selector" ] ||
  fail 'retry after staged private-state crash did not publish the canonical directory'
[ -f "$runtime/wahrwelt-shell-selector/.wahrwelt-state-owner" ] ||
  fail 'retry after staged private-state crash published an incomplete directory'
[ "$(stat -c '%d:%i' -- "$pending")" = "$pending_inode" ] ||
  fail 'retry mutated the unpublished crash-recovery directory'

runtime="$test_root/runtime-managed-leaf-crash-resume"
state_dir="$runtime/wahrwelt-noctalia-launcher"
mkdir -m 0700 -- "$runtime"
create_state_namespace "$runtime" wahrwelt-noctalia-launcher noctalia-launcher-state
: >"$state_dir/active"
chmod 0600 -- "$state_dir/active"
active_inode="$(stat -c '%d:%i' -- "$state_dir/active")"
adopt_legacy_state_namespace "$runtime" wahrwelt-noctalia-launcher noctalia-launcher-state
[ -f "$state_dir/.wahrwelt-owner.active" ] ||
  fail 'retry after managed-leaf crash did not resume the exact empty leaf marker'
[ "$(stat -c '%d:%i' -- "$state_dir/active")" = "$active_inode" ] ||
  fail 'managed-leaf crash recovery replaced the exact payload inode'
adopt_legacy_state_namespace "$runtime" wahrwelt-noctalia-launcher noctalia-launcher-state

runtime="$test_root/runtime-managed-marker-anonymous-failure"
state_dir="$runtime/wahrwelt-noctalia-launcher"
mkdir -m 0700 -- "$runtime"
create_state_namespace "$runtime" wahrwelt-noctalia-launcher noctalia-launcher-state
if HOME="$home" XDG_RUNTIME_DIR="$runtime" \
  WAHRWELT_TEST_MANAGED_MARKER_FAIL_BEFORE_LINK_KIND=noctalia-active bash -c '
    set -euo pipefail
    . "$1"
    wahrwelt_open_private_state_directory wahrwelt-noctalia-launcher noctalia-launcher-state
    wahrwelt_open_managed_regular_file \
      "$wahrwelt_private_state_directory_fd" \
      "$wahrwelt_private_state_directory_path" \
      active noctalia-active
  ' bash "$runtime_script" >"$test_root/managed-marker-anonymous-failure.out" 2>&1; then
  fail 'injected managed marker failure unexpectedly succeeded'
fi
[ -f "$state_dir/active" ] || fail 'managed marker failure lost its exact empty payload leaf'
[ ! -e "$state_dir/.wahrwelt-owner.active" ] ||
  fail 'managed marker failure exposed partial marker bytes before atomic link'
adopt_legacy_state_namespace "$runtime" wahrwelt-noctalia-launcher noctalia-launcher-state
[ -f "$state_dir/.wahrwelt-owner.active" ] ||
  fail 'retry after anonymous marker failure did not recover the empty managed leaf'

runtime="$test_root/runtime-legacy-selector-stale-lock"
state_dir="$runtime/wahrwelt-shell-selector"
mkdir -m 0700 -- "$runtime"
mkdir -m 0755 -- "$state_dir" "$state_dir/lock"
printf '%s\n' 999999 >"$state_dir/lock/pid"
printf '%s\n' wahrwelt-shell-selector >"$state_dir/lock/owner"
chmod 0644 -- "$state_dir/lock/pid" "$state_dir/lock/owner"
adopt_legacy_state_namespace "$runtime" wahrwelt-shell-selector shell-selector-state
[ -f "$state_dir/.wahrwelt-state-owner" ] || fail 'exact stale-lock legacy selector state was not adopted'
assert_file "$state_dir/lock/pid" 999999
assert_file "$state_dir/lock/owner" wahrwelt-shell-selector

runtime="$test_root/runtime-legacy-selector-live-lock"
state_dir="$runtime/wahrwelt-shell-selector"
mkdir -m 0700 -- "$runtime"
mkdir -m 0755 -- "$state_dir" "$state_dir/lock"
bash -c 'while :; do sleep 1; done' /tmp/shell-selector.sh &
legacy_lock_pid=$!
printf '%s\n' "$legacy_lock_pid" >"$state_dir/lock/pid"
printf '%s\n' wahrwelt-shell-selector >"$state_dir/lock/owner"
chmod 0644 -- "$state_dir/lock/pid" "$state_dir/lock/owner"
if adopt_legacy_state_namespace "$runtime" wahrwelt-shell-selector shell-selector-state \
  >"$test_root/legacy-selector-live-lock.out" 2>&1; then
  fail 'legacy selector state with a live lock owner was adopted'
fi
kill "$legacy_lock_pid" 2>/dev/null || true
wait "$legacy_lock_pid" 2>/dev/null || true
legacy_lock_pid=""
[ "$(stat -c %a -- "$state_dir")" = 755 ] || fail 'live-locked legacy selector directory mode changed'
[ ! -e "$state_dir/.wahrwelt-state-owner" ] || fail 'live-locked legacy selector state received a marker'

runtime="$test_root/runtime-legacy-selector-unknown"
state_dir="$runtime/wahrwelt-shell-selector"
mkdir -m 0700 -- "$runtime"
mkdir -m 0755 -- "$state_dir"
printf '%s\n' 'unknown selector state' >"$state_dir/unknown"
if adopt_legacy_state_namespace "$runtime" wahrwelt-shell-selector shell-selector-state \
  >"$test_root/legacy-selector-unknown.out" 2>&1; then
  fail 'legacy selector state with an unknown entry was adopted'
fi
assert_file "$state_dir/unknown" 'unknown selector state'
[ "$(stat -c %a -- "$state_dir")" = 755 ] || fail 'unknown legacy selector directory mode changed'
[ ! -e "$state_dir/.wahrwelt-state-owner" ] || fail 'unknown legacy selector state received a marker'

runtime="$test_root/runtime-legacy-selector-hardlink"
state_dir="$runtime/wahrwelt-shell-selector"
hardlink_target="$test_root/legacy-selector-hardlink-pid"
mkdir -m 0700 -- "$runtime"
mkdir -m 0755 -- "$state_dir" "$state_dir/lock"
printf '%s\n' 999999 >"$hardlink_target"
ln -- "$hardlink_target" "$state_dir/lock/pid"
printf '%s\n' wahrwelt-shell-selector >"$state_dir/lock/owner"
chmod 0644 -- "$state_dir/lock/pid" "$state_dir/lock/owner"
if adopt_legacy_state_namespace "$runtime" wahrwelt-shell-selector shell-selector-state \
  >"$test_root/legacy-selector-hardlink.out" 2>&1; then
  fail 'legacy selector state with hardlinked lock metadata was adopted'
fi
assert_file "$hardlink_target" 999999
[ "$(stat -c %a -- "$state_dir")" = 755 ] || fail 'hardlinked legacy selector directory mode changed'
[ ! -e "$state_dir/.wahrwelt-state-owner" ] || fail 'hardlinked legacy selector state received a marker'

runtime="$test_root/runtime-legacy-selector-writable"
state_dir="$runtime/wahrwelt-shell-selector"
mkdir -m 0700 -- "$runtime"
mkdir -m 0770 -- "$state_dir"
if adopt_legacy_state_namespace "$runtime" wahrwelt-shell-selector shell-selector-state \
  >"$test_root/legacy-selector-writable.out" 2>&1; then
  fail 'group-writable legacy selector state was adopted'
fi
[ "$(stat -c %a -- "$state_dir")" = 770 ] || fail 'writable legacy selector directory mode changed'
[ ! -e "$state_dir/.wahrwelt-state-owner" ] || fail 'writable legacy selector state received a marker'

runtime="$test_root/runtime-fresh-log-concurrent"
ready="$test_root/fresh-log-ready"
release="$test_root/fresh-log-release"
mkdir -m 0700 -- "$runtime"
HOME="$home" XDG_RUNTIME_DIR="$runtime" READY="$ready" RELEASE="$release" bash -c '
  set -euo pipefail
  wahrwelt_after_managed_regular_token_hook() {
    [ "$1" = shell-log ] && [ "${4:-}" = 1 ] || return 0
    : >"$READY"
    while [ ! -e "$RELEASE" ]; do sleep 0.01; done
  }
  . "$1"
' bash "$runtime_script" >"$test_root/fresh-log-first.out" 2>&1 &
first_pid=$!
background_pids="$background_pids $first_pid"
wait_for_file "$ready"
HOME="$home" XDG_RUNTIME_DIR="$runtime" bash -c '. "$1"' bash "$runtime_script" \
  >"$test_root/fresh-log-second.out" 2>&1
: >"$release"
if ! wait "$first_pid"; then
  fail 'fresh log loser did not reclassify the exact completed winner'
fi
[ -f "$runtime/wahrwelt-shell.log" ] || fail 'concurrent fresh log creation lost the managed log'
[ -f "$runtime/.wahrwelt-owner.wahrwelt-shell.log" ] || fail 'concurrent fresh log creation lost its ownership marker'

runtime="$test_root/runtime-legacy-log-concurrent"
legacy_log="$runtime/wahrwelt-shell.log"
ready="$test_root/legacy-log-concurrent-ready"
release="$test_root/legacy-log-concurrent-release"
mkdir -m 0700 -- "$runtime"
printf '%s\n' 'concurrent legacy log payload' >"$legacy_log"
chmod 0644 -- "$legacy_log"
HOME="$home" XDG_RUNTIME_DIR="$runtime" READY="$ready" RELEASE="$release" bash -c '
  set -euo pipefail
  wahrwelt_after_legacy_managed_regular_preflight_hook() {
    [ -n "${3:-}" ] || return 0
    : >"$READY"
    while [ ! -e "$RELEASE" ]; do sleep 0.01; done
  }
  . "$1"
' bash "$runtime_script" >"$test_root/legacy-log-concurrent-first.out" 2>&1 &
first_pid=$!
background_pids="$background_pids $first_pid"
wait_for_file "$ready"
HOME="$home" XDG_RUNTIME_DIR="$runtime" bash -c '. "$1"' bash "$runtime_script" \
  >"$test_root/legacy-log-concurrent-second.out" 2>&1
: >"$release"
if ! wait "$first_pid"; then
  fail 'legacy log loser did not validate the exact completed winner'
fi
assert_file "$legacy_log" 'concurrent legacy log payload'
[ -f "$runtime/.wahrwelt-owner.wahrwelt-shell.log" ] || fail 'concurrent legacy log adoption lost its ownership marker'

runtime="$test_root/runtime-legacy-state-concurrent"
state_dir="$runtime/wahrwelt-shell-selector"
ready="$test_root/legacy-state-concurrent-ready"
release="$test_root/legacy-state-concurrent-release"
mkdir -m 0700 -- "$runtime"
mkdir -m 0755 -- "$state_dir"
HOME="$home" XDG_RUNTIME_DIR="$runtime" READY="$ready" RELEASE="$release" bash -c '
  set -euo pipefail
  . "$1"
  wahrwelt_after_legacy_state_preflight_hook() {
    [ "$1" = shell-selector-state ] || return 0
    : >"$READY"
    while [ ! -e "$RELEASE" ]; do sleep 0.01; done
  }
  wahrwelt_adopt_legacy_private_state_directory wahrwelt-shell-selector shell-selector-state
' bash "$runtime_script" >"$test_root/legacy-state-concurrent-first.out" 2>&1 &
first_pid=$!
background_pids="$background_pids $first_pid"
wait_for_file "$ready"
adopt_legacy_state_namespace "$runtime" wahrwelt-shell-selector shell-selector-state \
  >"$test_root/legacy-state-concurrent-second.out" 2>&1
: >"$release"
if ! wait "$first_pid"; then
  fail 'legacy state loser did not validate the exact completed winner'
fi
[ "$(stat -c %a -- "$state_dir")" = 700 ] || fail 'concurrent legacy state adoption restored mode 0755 over winner'
[ -f "$state_dir/.wahrwelt-state-owner" ] || fail 'concurrent legacy state adoption lost its ownership marker'

runtime="$test_root/runtime-legacy-recording"
state_dir="$runtime/wahrwelt-recording"
mkdir -m 0700 -- "$runtime"
mkdir -m 0755 -- "$state_dir"
printf '%s\n' 424242 >"$state_dir/gpu-screen-recorder.pid"
printf '%s\n' "$test_root/recordings/legacy.mp4" >"$state_dir/gpu-screen-recorder.path"
printf '%s\n' 'legacy recorder log payload' >"$state_dir/gpu-screen-recorder.log"
chmod 0644 -- "$state_dir"/gpu-screen-recorder.*
HOME="$home" XDG_RUNTIME_DIR="$runtime" bash -c '
  set -euo pipefail
  . "$1"
  wahrwelt_adopt_legacy_private_state_directory wahrwelt-recording record-toggle-state
  wahrwelt_open_private_state_directory wahrwelt-recording record-toggle-state
  state_dir="$wahrwelt_private_state_directory_path"
  state_fd="$wahrwelt_private_state_directory_fd"
  for spec in \
    gpu-screen-recorder.pid:recorder-pid \
    gpu-screen-recorder.path:recorder-path \
    gpu-screen-recorder.log:recorder-log; do
    name="${spec%%:*}"
    kind="${spec#*:}"
    wahrwelt_open_managed_regular_file "$state_fd" "$state_dir" "$name" "$kind"
    cat "$wahrwelt_managed_regular_path"
    exec {wahrwelt_managed_regular_fd}<&-
    wahrwelt_managed_regular_fd=""
  done
' bash "$runtime_script" >"$test_root/legacy-recording.out"
assert_file "$test_root/legacy-recording.out" $'424242\n'"$test_root/recordings/legacy.mp4"$'\nlegacy recorder log payload'
[ "$(stat -c %a -- "$state_dir")" = 700 ] || fail 'legacy recording state directory was not made private'
for marker in \
  .wahrwelt-state-owner \
  .wahrwelt-owner.gpu-screen-recorder.pid \
  .wahrwelt-owner.gpu-screen-recorder.path \
  .wahrwelt-owner.gpu-screen-recorder.log; do
  [ "$(stat -c %a -- "$state_dir/$marker")" = 600 ] ||
    fail "legacy recorder ownership marker is missing or unsafe: $marker"
done

for marker_count in 0 1 2 3; do
  runtime="$test_root/runtime-legacy-recording-partial-$marker_count"
  state_dir="$runtime/wahrwelt-recording"
  mkdir -m 0700 -- "$runtime"
  mkdir -m 0755 -- "$state_dir"
  printf '%s\n' 424242 >"$state_dir/gpu-screen-recorder.pid"
  printf '%s\n' "$test_root/recordings/partial-$marker_count.mp4" >"$state_dir/gpu-screen-recorder.path"
  printf '%s\n' "partial recorder log $marker_count" >"$state_dir/gpu-screen-recorder.log"
  chmod 0644 -- "$state_dir"/gpu-screen-recorder.*
  partial_specs=(
    'gpu-screen-recorder.pid:recorder-pid'
    'gpu-screen-recorder.path:recorder-path'
    'gpu-screen-recorder.log:recorder-log'
  )
  for ((marker_index = 0; marker_index < marker_count; marker_index++)); do
    partial_spec="${partial_specs[$marker_index]}"
    seed_exact_leaf_marker "$state_dir" "${partial_spec%%:*}" "${partial_spec#*:}"
  done
  adopt_legacy_state_namespace "$runtime" wahrwelt-recording record-toggle-state
  adopt_legacy_state_namespace "$runtime" wahrwelt-recording record-toggle-state
  [ "$(stat -c %a -- "$state_dir")" = 700 ] ||
    fail "partial legacy recording state $marker_count did not finish privately"
  [ -f "$state_dir/.wahrwelt-state-owner" ] ||
    fail "partial legacy recording state $marker_count did not publish its commit marker"
  for partial_spec in "${partial_specs[@]}"; do
    [ -f "$state_dir/.wahrwelt-owner.${partial_spec%%:*}" ] ||
      fail "partial legacy recording state $marker_count lost marker ${partial_spec%%:*}"
  done
done

runtime="$test_root/runtime-legacy-state-marker-replacement"
state_dir="$runtime/wahrwelt-recording"
mkdir -m 0700 -- "$runtime"
mkdir -m 0755 -- "$state_dir"
printf '%s\n' 'replacement race log' >"$state_dir/gpu-screen-recorder.log"
chmod 0644 -- "$state_dir/gpu-screen-recorder.log"
seed_exact_leaf_marker "$state_dir" gpu-screen-recorder.log recorder-log
printf '%s\n' 'unknown replacement marker' >"$state_dir/.wahrwelt-owner.gpu-screen-recorder.log"
chmod 0600 -- "$state_dir/.wahrwelt-owner.gpu-screen-recorder.log"
replacement_marker_inode="$(stat -c '%d:%i' -- "$state_dir/.wahrwelt-owner.gpu-screen-recorder.log")"
if adopt_legacy_state_namespace "$runtime" wahrwelt-recording record-toggle-state \
  >"$test_root/legacy-state-marker-replacement.out" 2>&1; then
  fail 'legacy state adoption accepted an unknown replacement leaf marker'
fi
assert_file "$state_dir/gpu-screen-recorder.log" 'replacement race log'
assert_file "$state_dir/.wahrwelt-owner.gpu-screen-recorder.log" 'unknown replacement marker'
[ "$(stat -c '%d:%i' -- "$state_dir/.wahrwelt-owner.gpu-screen-recorder.log")" = "$replacement_marker_inode" ] ||
  fail 'legacy state cleanup replaced or removed an unknown marker inode'

runtime="$test_root/runtime-legacy-recording-consumer"
state_dir="$runtime/wahrwelt-recording"
videos="$test_root/videos-legacy-recording-consumer"
mkdir -m 0700 -- "$runtime"
mkdir -m 0755 -- "$state_dir"
printf '%s\n' 'legacy consumer log payload' >"$state_dir/gpu-screen-recorder.log"
chmod 0644 -- "$state_dir/gpu-screen-recorder.log"
if HOME="$home" XDG_RUNTIME_DIR="$runtime" XDG_VIDEOS_DIR="$videos" \
  WAHRWELT_RUNTIME_LOCK_V2=wahrwelt-record-toggle-v2.lock WAHRWELT_RUNTIME_LOCK_V2_ROOT="$runtime" \
  bash -c '
    command() {
      if [ "${1:-}" = -v ] && [ "${2:-}" = gpu-screen-recorder ]; then return 1; fi
      builtin command "$@"
    }
    notify-send() { :; }
    . "$1"
  ' bash "$record_toggle" >"$test_root/legacy-recording-consumer.out" 2>&1; then
  fail 'recording consumer unexpectedly found gpu-screen-recorder during legacy adoption test'
fi
if grep -Fq 'ownership collision' "$test_root/legacy-recording-consumer.out"; then
  fail 'recording consumer rejected the exact legacy state shape'
fi
assert_file "$state_dir/gpu-screen-recorder.log" 'legacy consumer log payload'
[ -f "$state_dir/.wahrwelt-state-owner" ] || fail 'recording consumer did not adopt legacy state ownership'

runtime="$test_root/runtime-selector-log-link"
unknown_target="$test_root/selector-log-unknown"
mkdir -m 0700 -- "$runtime"
printf '%s\n' 'selector log target bytes' >"$unknown_target"
ln -s -- "$unknown_target" "$runtime/wahrwelt-shell.log"
if HOME="$home" XDG_RUNTIME_DIR="$runtime" \
  WAHRWELT_RUNTIME_LOCK_V2=wahrwelt-shell-selector-v2.lock WAHRWELT_RUNTIME_LOCK_V2_ROOT="$runtime" \
  bash -c '. "$1" invalid-action' bash "$selector_script" >"$test_root/selector-log-link.out" 2>&1; then
  fail 'shell selector adopted a symlink managed log'
fi
assert_file "$unknown_target" 'selector log target bytes'
[ -L "$runtime/wahrwelt-shell.log" ] || fail 'selector log symlink was changed'

runtime="$test_root/runtime-noctalia-leaf-link"
state_dir="$runtime/wahrwelt-noctalia-launcher"
unknown_target="$test_root/noctalia-active-unknown"
mkdir -m 0700 -- "$runtime"
create_state_namespace "$runtime" wahrwelt-noctalia-launcher noctalia-launcher-state
printf '%s\n' 'noctalia active target bytes' >"$unknown_target"
ln -s -- "$unknown_target" "$state_dir/active"
if HOME="$home" XDG_RUNTIME_DIR="$runtime" \
  WAHRWELT_RUNTIME_LOCK_V2=wahrwelt-noctalia-launcher-v2.lock WAHRWELT_RUNTIME_LOCK_V2_ROOT="$runtime" \
  bash -c '. "$1" press' bash "$noctalia_launcher" >"$test_root/noctalia-leaf-link.out" 2>&1; then
  fail 'noctalia launcher adopted a symlink marker leaf'
fi
assert_file "$unknown_target" 'noctalia active target bytes'
[ -L "$state_dir/active" ] || fail 'noctalia active symlink was changed'

for recorder_leaf in gpu-screen-recorder.pid gpu-screen-recorder.path gpu-screen-recorder.log; do
  runtime="$test_root/runtime-recorder-${recorder_leaf##*.}-link"
  state_dir="$runtime/wahrwelt-recording"
  unknown_target="$test_root/recorder-${recorder_leaf##*.}-unknown"
  videos="$test_root/videos-recorder-${recorder_leaf##*.}"
  mkdir -m 0700 -- "$runtime"
  create_state_namespace "$runtime" wahrwelt-recording record-toggle-state
  printf '%s\n' "recorder $recorder_leaf target bytes" >"$unknown_target"
  ln -s -- "$unknown_target" "$state_dir/$recorder_leaf"
  if HOME="$home" XDG_RUNTIME_DIR="$runtime" XDG_VIDEOS_DIR="$videos" \
    WAHRWELT_RUNTIME_LOCK_V2=wahrwelt-record-toggle-v2.lock WAHRWELT_RUNTIME_LOCK_V2_ROOT="$runtime" \
    bash -c '
      command() {
        if [ "${1:-}" = -v ] && [ "${2:-}" = gpu-screen-recorder ]; then return 1; fi
        builtin command "$@"
      }
      notify-send() { :; }
      . "$1"
    ' bash "$record_toggle" >"$test_root/recorder-$recorder_leaf.out" 2>&1; then
    fail "record toggle adopted symlink leaf $recorder_leaf"
  fi
  assert_file "$unknown_target" "recorder $recorder_leaf target bytes"
  [ -L "$state_dir/$recorder_leaf" ] || fail "record toggle changed symlink leaf $recorder_leaf"
done

runtime="$test_root/runtime-noctalia-leaf-ordinary"
state_dir="$runtime/wahrwelt-noctalia-launcher"
unknown_target="$state_dir/active"
mkdir -m 0700 -- "$runtime"
create_state_namespace "$runtime" wahrwelt-noctalia-launcher noctalia-launcher-state
printf '%s\n' 'ordinary active leaf bytes' >"$unknown_target"
chmod 0600 -- "$unknown_target"
if HOME="$home" XDG_RUNTIME_DIR="$runtime" \
  WAHRWELT_RUNTIME_LOCK_V2=wahrwelt-noctalia-launcher-v2.lock WAHRWELT_RUNTIME_LOCK_V2_ROOT="$runtime" \
  bash -c '. "$1" press' bash "$noctalia_launcher" >"$test_root/noctalia-leaf-ordinary.out" 2>&1; then
  fail 'noctalia launcher adopted an unmarked ordinary marker leaf'
fi
assert_file "$unknown_target" 'ordinary active leaf bytes'
[ ! -e "$state_dir/.wahrwelt-owner.active" ] || fail 'noctalia launcher marked an unknown active leaf'

runtime="$test_root/runtime-recorder-leaf-ordinary"
state_dir="$runtime/wahrwelt-recording"
unknown_target="$state_dir/gpu-screen-recorder.log"
videos="$test_root/videos-recorder-ordinary"
mkdir -m 0700 -- "$runtime"
create_state_namespace "$runtime" wahrwelt-recording record-toggle-state
printf '%s\n' 'ordinary recorder log bytes' >"$unknown_target"
chmod 0600 -- "$unknown_target"
if HOME="$home" XDG_RUNTIME_DIR="$runtime" XDG_VIDEOS_DIR="$videos" \
  WAHRWELT_RUNTIME_LOCK_V2=wahrwelt-record-toggle-v2.lock WAHRWELT_RUNTIME_LOCK_V2_ROOT="$runtime" \
  bash -c '
    command() {
      if [ "${1:-}" = -v ] && [ "${2:-}" = gpu-screen-recorder ]; then return 1; fi
      builtin command "$@"
    }
    notify-send() { :; }
    . "$1"
  ' bash "$record_toggle" >"$test_root/recorder-leaf-ordinary.out" 2>&1; then
  fail 'record toggle adopted an unmarked ordinary log leaf'
fi
assert_file "$unknown_target" 'ordinary recorder log bytes'
[ ! -e "$state_dir/.wahrwelt-owner.gpu-screen-recorder.log" ] || fail 'record toggle marked an unknown log leaf'

runtime="$test_root/runtime-state-token-race"
state_saved="$test_root/state-token-original"
mkdir -m 0700 -- "$runtime"
if HOME="$home" XDG_RUNTIME_DIR="$runtime" STATE_SAVED="$state_saved" \
  WAHRWELT_RUNTIME_LOCK_V2=wahrwelt-shell-selector-v2.lock WAHRWELT_RUNTIME_LOCK_V2_ROOT="$runtime" \
  bash -c '
    set -euo pipefail
    wahrwelt_after_private_state_directory_token_hook() {
      [ "$1" = shell-selector-state ] || return 0
      mv -- "$2" "$STATE_SAVED"
      mkdir -m 0700 -- "$2"
      printf "%s\n" "state winner bytes" >"$2/winner"
    }
    . "$1" invalid-action
  ' bash "$selector_script" >"$test_root/state-token-race.out" 2>&1; then
  fail 'state directory replacement between token and reopen was accepted'
fi
[ -d "$state_saved" ] || fail 'creator-owned state directory recovery was not preserved'
assert_file "$runtime/wahrwelt-shell-selector/winner" 'state winner bytes'

runtime="$test_root/runtime-leaf-token-race"
leaf_saved="$test_root/leaf-token-original"
unknown_target="$test_root/leaf-token-unknown"
mkdir -m 0700 -- "$runtime"
printf '%s\n' 'leaf winner target bytes' >"$unknown_target"
if HOME="$home" XDG_RUNTIME_DIR="$runtime" LEAF_SAVED="$leaf_saved" UNKNOWN_TARGET="$unknown_target" \
  WAHRWELT_RUNTIME_LOCK_V2=wahrwelt-noctalia-launcher-v2.lock WAHRWELT_RUNTIME_LOCK_V2_ROOT="$runtime" \
  bash -c '
    set -euo pipefail
    wahrwelt_after_managed_regular_token_hook() {
      [ "$1" = noctalia-active ] || return 0
      mv -- "$2" "$LEAF_SAVED"
      ln -s -- "$UNKNOWN_TARGET" "$2"
    }
    . "$1" press
  ' bash "$noctalia_launcher" >"$test_root/leaf-token-race.out" 2>&1; then
  fail 'managed leaf replacement between token and reopen was accepted'
fi
[ -f "$leaf_saved" ] || fail 'creator-owned managed leaf recovery was not preserved'
assert_file "$unknown_target" 'leaf winner target bytes'

runtime="$test_root/runtime-token-race"
runtime_saved="$test_root/runtime-token-original"
mkdir -m 0700 -- "$runtime"
if HOME="$home" XDG_RUNTIME_DIR="$runtime" RUNTIME_SAVED="$runtime_saved" \
  bash -c '
    set -euo pipefail
    wahrwelt_after_runtime_directory_token_hook() {
      mv -- "$1" "$RUNTIME_SAVED"
      mkdir -m 0700 -- "$1"
      printf "%s\n" "runtime winner bytes" >"$1/winner"
    }
    . "$1"
  ' bash "$runtime_script" >"$test_root/runtime-token-race.out" 2>&1; then
  fail 'runtime replacement between token and reopen was accepted'
fi
[ -d "$runtime_saved" ] || fail 'token-bound original runtime was not preserved'
assert_file "$runtime/winner" 'runtime winner bytes'

runtime="$test_root/runtime-lock-v2"
lock_ready="$test_root/runtime-lock-v2-ready"
lock_release="$test_root/runtime-lock-v2-release"
lock_finished="$test_root/runtime-lock-v2-finished"
lock_worker="$test_root/runtime-lock-v2-worker"
mkdir -m 0700 -- "$runtime"
printf '%s\n' \
  '#!/usr/bin/env bash' \
  'set -euo pipefail' \
  '. "$RUNTIME_SCRIPT"' \
  'wahrwelt_enter_runtime_lock_v2 wahrwelt-shell-v2.lock 0 97 false' \
  '[ -z "${WAHRWELT_RUNTIME_LOCK_V2:-}" ]' \
  '[ -z "${WAHRWELT_RUNTIME_LOCK_V2_ROOT:-}" ]' \
  'if bash -c '\''set -euo pipefail; . "$1"; wahrwelt_enter_runtime_lock_v2 wahrwelt-shell-v2.lock 20 29 true'\'' bash "$RUNTIME_SCRIPT"; then exit 98; else status=$?; [ "$status" -eq 29 ]; fi' \
  ': >"$LOCK_READY"' \
  'while [ ! -e "$LOCK_RELEASE" ]; do sleep 0.005; done' \
  ': >"$LOCK_FINISHED"' \
  >"$lock_worker"
chmod 0755 -- "$lock_worker"
HOME="$home" XDG_RUNTIME_DIR="$runtime" RUNTIME_SCRIPT="$runtime_script" LOCK_READY="$lock_ready" \
  LOCK_RELEASE="$lock_release" LOCK_FINISHED="$lock_finished" \
  bash -c '
    set -euo pipefail
    . "$1"
    wahrwelt_enter_runtime_lock_v2 wahrwelt-shell-v2.lock 0 1 "$2"
  ' bash "$runtime_script" "$lock_worker" &
lock_owner_pid=$!
background_pids="$background_pids $lock_owner_pid"
wait_for_file "$lock_ready"
if HOME="$home" XDG_RUNTIME_DIR="$runtime" bash -c '
  set -euo pipefail
  . "$1"
  wahrwelt_enter_runtime_lock_v2 wahrwelt-shell-v2.lock 30 23 true
' bash "$runtime_script"; then
  fail 'contending v2 abstract lock unexpectedly ran'
else
  lock_status=$?
  [ "$lock_status" -eq 23 ] || fail "contending v2 lock status was $lock_status"
fi
: >"$lock_release"
wait "$lock_owner_pid"
wait_for_file "$lock_finished"
HOME="$home" XDG_RUNTIME_DIR="$runtime" bash -c '
  set -euo pipefail
  . "$1"
  wahrwelt_enter_runtime_lock_v2 wahrwelt-shell-v2.lock 100 1 true
' bash "$runtime_script" || fail 'v2 abstract lock was not released after child exit'
if find "$runtime" -mindepth 1 -name '*lock*' -print -quit | grep -q .; then
  fail 'v2 abstract lock left a filesystem anchor'
fi

runtime="$test_root/runtime-end4-upgrade-state"
mkdir -m 0700 -- "$runtime"
HOME="$home" XDG_RUNTIME_DIR="$runtime" bash -c '
  set -euo pipefail
  . "$1"
  wahrwelt_open_end4_upgrade_state
  [ "$(wahrwelt_merge_end4_upgrade_tokens "5102:102:end4-pC,5101:101:ii")" = \
    "5101:101:ii,5102:102:end4-pC" ]
  [ "$(wahrwelt_read_end4_upgrade_tokens)" = \
    "5101:101:ii,5102:102:end4-pC" ]
  [ "$(wahrwelt_remove_end4_upgrade_tokens "5101:101:ii")" = \
    "5102:102:end4-pC" ]
' bash "$runtime_script"
[ -f "$runtime/wahrwelt-end4-upgrade/5102:102:end4-pC" ] ||
  fail 'durable End4 upgrade state did not retain the unconsumed exact token'
[ ! -e "$runtime/wahrwelt-end4-upgrade/5101:101:ii" ] ||
  fail 'durable End4 upgrade state did not consume the proven exact token'

runtime="$test_root/runtime-end4-upgrade-concurrent"
ready="$test_root/end4-upgrade-concurrent-ready"
release="$test_root/end4-upgrade-concurrent-release"
mkdir -m 0700 -- "$runtime"
HOME="$home" XDG_RUNTIME_DIR="$runtime" READY="$ready" RELEASE="$release" bash -c '
  set -euo pipefail
  . "$1"
  wahrwelt_open_end4_upgrade_state
  wahrwelt_after_end4_upgrade_state_lock_hook() {
    [ "$1" = merge ] || return 0
    : >"$READY"
    while [ ! -e "$RELEASE" ]; do sleep 0.01; done
  }
  wahrwelt_merge_end4_upgrade_tokens "5201:201:ii" >/dev/null
' bash "$runtime_script" >"$test_root/end4-upgrade-concurrent-first.out" 2>&1 &
first_pid=$!
background_pids="$background_pids $first_pid"
wait_for_file "$ready"
HOME="$home" XDG_RUNTIME_DIR="$runtime" bash -c '
  set -euo pipefail
  . "$1"
  wahrwelt_open_end4_upgrade_state
  wahrwelt_merge_end4_upgrade_tokens "5202:202:end4-pC" >/dev/null
' bash "$runtime_script" >"$test_root/end4-upgrade-concurrent-second.out" 2>&1 &
second_pid=$!
background_pids="$background_pids $second_pid"
sleep 0.05
if ! kill -0 "$second_pid" 2>/dev/null; then
  fail 'concurrent End4 upgrade writer bypassed the pinned state transaction lock'
fi
: >"$release"
wait "$first_pid" || fail 'first concurrent End4 upgrade writer failed'
wait "$second_pid" || fail 'second concurrent End4 upgrade writer failed'
concurrent_tokens="$(HOME="$home" XDG_RUNTIME_DIR="$runtime" bash -c '
  set -euo pipefail
  . "$1"
  wahrwelt_open_end4_upgrade_state
  wahrwelt_read_end4_upgrade_tokens
' bash "$runtime_script")"
[ "$concurrent_tokens" = "5201:201:ii,5202:202:end4-pC" ] ||
  fail "concurrent End4 upgrade writers lost a token: $concurrent_tokens"

HOME="$home" XDG_RUNTIME_DIR="$runtime" bash -c '
  set -euo pipefail
  . "$1"
  wahrwelt_open_end4_upgrade_state
  wahrwelt_merge_end4_upgrade_tokens "5203:203:ii" >/dev/null
  [ "$(wahrwelt_remove_end4_upgrade_tokens "5201:201:ii")" = \
    "5202:202:end4-pC,5203:203:ii" ]
' bash "$runtime_script" || fail 'compare-and-clear removed a concurrently merged End4 upgrade token'

runtime="$test_root/runtime-end4-upgrade-remove-race"
state_dir="$runtime/wahrwelt-end4-upgrade"
saved_token="$test_root/end4-upgrade-remove-race-original"
replacement_identity="$test_root/end4-upgrade-remove-race-replacement-identity"
mkdir -m 0700 -- "$runtime"
HOME="$home" XDG_RUNTIME_DIR="$runtime" bash -c '
  set -euo pipefail
  . "$1"
  wahrwelt_open_end4_upgrade_state
  wahrwelt_merge_end4_upgrade_tokens "5251:251:ii" >/dev/null
' bash "$runtime_script"
if HOME="$home" XDG_RUNTIME_DIR="$runtime" SAVED_TOKEN="$saved_token" \
  REPLACEMENT_IDENTITY="$replacement_identity" bash -c '
    set -euo pipefail
    . "$1"
    wahrwelt_open_end4_upgrade_state
    wahrwelt_before_end4_upgrade_token_remove_hook() {
      local token="$1"
      local path="$2"

      mv -- "$path" "$SAVED_TOKEN"
      printf "Wahrwelt End4 upgrade process v1\n%s\n" "$token" >"$path"
      chmod 0600 -- "$path"
      stat -c "%d:%i" -- "$path" >"$REPLACEMENT_IDENTITY"
    }
    wahrwelt_remove_end4_upgrade_tokens "5251:251:ii" >/dev/null
  ' bash "$runtime_script" >"$test_root/end4-upgrade-remove-race.out" 2>&1; then
  fail 'End4 upgrade compare-and-clear consumed a raced replacement inode'
fi
assert_file "$saved_token" $'Wahrwelt End4 upgrade process v1\n5251:251:ii'
assert_file "$state_dir/5251:251:ii" $'Wahrwelt End4 upgrade process v1\n5251:251:ii'
[ "$(stat -c '%d:%i' -- "$state_dir/5251:251:ii")" = "$(cat "$replacement_identity")" ] ||
  fail 'End4 upgrade compare-and-clear did not restore the raced replacement inode'
if find "$state_dir" -maxdepth 1 -name '.consumed.*' -print -quit | grep -q .; then
  fail 'End4 upgrade compare-and-clear left a false consumed tombstone after raced replacement'
fi

for collision in unknown symlink hardlink writable content marker; do
  runtime="$test_root/runtime-end4-upgrade-collision-$collision"
  state_dir="$runtime/wahrwelt-end4-upgrade"
  mkdir -m 0700 -- "$runtime"
  HOME="$home" XDG_RUNTIME_DIR="$runtime" bash -c '
    set -euo pipefail
    . "$1"
    wahrwelt_open_end4_upgrade_state
    wahrwelt_merge_end4_upgrade_tokens "5301:301:ii" >/dev/null
  ' bash "$runtime_script"
  case "$collision" in
    unknown)
      printf '%s\n' 'unknown state bytes' >"$state_dir/unknown"
      ;;
    symlink)
      mv -- "$state_dir/5301:301:ii" "$state_dir/token-saved"
      ln -s -- "$state_dir/token-saved" "$state_dir/5301:301:ii"
      ;;
    hardlink)
      ln -- "$state_dir/5301:301:ii" "$state_dir/token-hardlink"
      ;;
    writable)
      chmod 0666 -- "$state_dir/5301:301:ii"
      ;;
    content)
      printf '%s\n' '5301:999:ii' >"$state_dir/5301:301:ii"
      ;;
    marker)
      printf '%s\n' 'unknown marker bytes' >"$state_dir/.wahrwelt-state-owner"
      ;;
  esac
  if HOME="$home" XDG_RUNTIME_DIR="$runtime" bash -c '
    set -euo pipefail
    . "$1"
    wahrwelt_open_end4_upgrade_state
    wahrwelt_read_end4_upgrade_tokens >/dev/null
  ' bash "$runtime_script" >"$test_root/end4-upgrade-collision-$collision.out" 2>&1; then
    fail "End4 upgrade state accepted $collision collision"
  fi
done

if grep -Fq 'os.unlink(marker, dir_fd=parent_fd)' "$runtime_script"; then
  fail 'legacy log marker exception cleanup still unlinks a public name after validation'
fi
if grep -Fq 'os.unlink(marker_name, dir_fd=directory_fd)' "$runtime_script"; then
  fail 'legacy state marker exception cleanup still unlinks a public name after validation'
fi

printf 'OK shell runtime directory ownership and creator-token races\n'
