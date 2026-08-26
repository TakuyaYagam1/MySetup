#!/usr/bin/env bash
set -euo pipefail

scripts_dir="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
test_root="$(mktemp -d)"
trap 'rm -rf -- "$test_root"' EXIT

export HOME="$test_root/home"
export XDG_CONFIG_HOME="$HOME/.config"
export XDG_STATE_HOME="$HOME/.local/state"
export XDG_RUNTIME_DIR="$test_root/runtime"
mkdir -p "$HOME" "$XDG_RUNTIME_DIR"
chmod 0700 "$XDG_RUNTIME_DIR"

# shellcheck source=Linux/dots/hypr/scripts/shell-runtime.sh
. "$scripts_dir/shell-runtime.sh"

log() {
  printf '%s\n' "$*" >>"$test_root/test.log"
}

# shellcheck source=Linux/dots/hypr/scripts/shell-profile-sync.sh
. "$scripts_dir/shell-profile-sync.sh"

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

assert_file() {
  local path="$1"
  local want="$2"
  local got

  got="$(tr -d '\n' <"$path")"
  [ "$got" = "$want" ] || fail "$path contains $got, want $want"
}

creator_owned=""
creator_replacement=""
wahrwelt_after_private_directory_creator_hook() {
  local kind="$1"
  local pinned="$2"
  local name

  case "$kind" in snapshot-*) ;; *) return 0 ;; esac
  name="${pinned##*/}"
  creator_owned="$XDG_RUNTIME_DIR/$name.owned"
  creator_replacement="$XDG_RUNTIME_DIR/$name"
  mv -T -- "$pinned" "$creator_owned"
  mkdir -- "$pinned"
  printf '%s\n' unknown-creator >"$pinned/unknown.txt"
}
if wahrwelt_begin_exact_snapshot "$wahrwelt_runtime_session_dir" .creator-race- creator; then
  fail "snapshot creator accepted a pre-open replacement"
fi
unset -f wahrwelt_after_private_directory_creator_hook
assert_file "$creator_replacement/unknown.txt" unknown-creator
[ -d "$creator_owned" ] || fail "creator-owned directory was not retained"

wahrwelt_begin_exact_snapshot "$wahrwelt_runtime_session_dir" .leaf-race- leaf
leaf_snapshot="$wahrwelt_new_snapshot_dir"
leaf_target="$test_root/leaf-target"
leaf_victim="$test_root/leaf-victim"
printf '%s\n' original >"$leaf_target"
printf '%s\n' preserve >"$leaf_victim"
wahrwelt_before_snapshot_leaf_write_hook() {
  local fd="$3"

  ln -s -- "$leaf_victim" "/proc/${BASHPID:-$$}/fd/$fd/0.type"
}
if snapshot_exact_paths "$leaf_snapshot" "$leaf_target"; then
  fail "snapshot metadata writer followed an injected symlink"
fi
unset -f wahrwelt_before_snapshot_leaf_write_hook
assert_file "$leaf_victim" preserve
[ -L "$leaf_snapshot/0.type" ] || fail "injected metadata symlink was not retained"
wahrwelt_unregister_exact_snapshot "$leaf_snapshot"

wahrwelt_begin_exact_snapshot "$wahrwelt_runtime_session_dir" .cleanup-race- cleanup
cleanup_snapshot="$wahrwelt_new_snapshot_dir"
cleanup_target="$test_root/cleanup-target"
printf '%s\n' cleanup-original >"$cleanup_target"
snapshot_exact_paths "$cleanup_snapshot" "$cleanup_target"
cleanup_owned=""
cleanup_replacement=""
wahrwelt_before_snapshot_cleanup_delete_hook() {
  local pinned="$1"
  local name

  name="${pinned##*/}"
  cleanup_owned="$XDG_RUNTIME_DIR/$name.owned"
  cleanup_replacement="$XDG_RUNTIME_DIR/$name"
  mv -T -- "$pinned" "$cleanup_owned"
  mkdir -- "$pinned"
  printf '%s\n' unknown-cleanup >"$pinned/unknown.txt"
}
if remove_exact_path_snapshot "$cleanup_snapshot" "$cleanup_target"; then
  fail "snapshot cleanup accepted a public replacement"
fi
unset -f wahrwelt_before_snapshot_cleanup_delete_hook
assert_file "$cleanup_replacement/unknown.txt" unknown-cleanup
assert_file "$cleanup_owned/0.path" "$cleanup_target"
cleanup_recovery="$(wahrwelt_snapshot_directory_recovery_path "$cleanup_snapshot")"
[ "$cleanup_recovery" = "$cleanup_owned" ] ||
  fail "cleanup recovery is $cleanup_recovery, want $cleanup_owned"
wahrwelt_unregister_exact_snapshot "$cleanup_snapshot"

wahrwelt_begin_exact_snapshot "$wahrwelt_runtime_session_dir" .stage-race- stage
stage_snapshot="$wahrwelt_new_snapshot_dir"
stage_target="$test_root/stage-target"
printf '%s\n' stage-original >"$stage_target"
snapshot_exact_paths "$stage_snapshot" "$stage_target"
write_regular_file "$stage_target" stage-current
stage_key="$(snapshot_parent_key "$stage_snapshot" 0)"
stage_recovery="${wahrwelt_snapshot_owned_recoveries[$stage_key]:-}"
[ -n "$stage_recovery" ] || fail "stage recovery was not journaled"
wahrwelt_before_runtime_stage_cleanup_hook() {
  mv -T -- "$stage_recovery" "$stage_recovery.owned"
  printf '%s\n' unknown-stage >"$stage_recovery"
}
if remove_exact_path_snapshot "$stage_snapshot" "$stage_target"; then
  fail "stage cleanup accepted a replacement"
fi
unset -f wahrwelt_before_runtime_stage_cleanup_hook
assert_file "$stage_recovery" unknown-stage
assert_file "$stage_recovery.owned" stage-original
wahrwelt_unregister_exact_snapshot "$stage_snapshot"

wahrwelt_begin_exact_snapshot "$wahrwelt_runtime_session_dir" .report-race- report
report_snapshot="$wahrwelt_new_snapshot_dir"
report_target="$test_root/report-target"
printf '%s\n' report-original >"$report_target"
snapshot_exact_paths "$report_snapshot" "$report_target"
report_expected="${wahrwelt_snapshot_directory_identities[$report_snapshot]}"
report_resolved=""
wahrwelt_before_snapshot_recovery_verify_hook() {
  local _snapshot="$1"
  local resolved="$2"

  report_resolved="$resolved"
  mv -T -- "$resolved" "$resolved.owned"
  mkdir -- "$resolved"
  printf '%s\n' unknown-report >"$resolved/unknown.txt"
}
if remove_exact_path_snapshot "$report_snapshot" "$report_target"; then
  fail "snapshot recovery report accepted a post-readlink replacement"
fi
unset -f wahrwelt_before_snapshot_recovery_verify_hook
[ -n "$report_resolved" ] || fail "snapshot recovery report hook did not run"
assert_file "$report_resolved/unknown.txt" unknown-report
assert_file "$report_resolved.owned/0.path" "$report_target"
[ -z "$wahrwelt_snapshot_recovery_exact_path" ] ||
  fail "unverified snapshot recovery was reported as exact"
[ "$wahrwelt_snapshot_recovery_identity" = "$report_expected" ] ||
  fail "live snapshot recovery identity was not retained"
[ -d "$wahrwelt_snapshot_recovery_fd_path" ] ||
  fail "live snapshot recovery descriptor was not retained"
[ "$(runtime_directory_identity "$wahrwelt_snapshot_recovery_fd_path")" = "$report_expected" ] ||
  fail "live snapshot recovery descriptor changed identity"
[ "$wahrwelt_snapshot_recovery_public_path" = "$report_snapshot" ] ||
  fail "snapshot public collision path was not reported separately"
[ "$wahrwelt_snapshot_recovery_public_identity" != "$report_expected" ] ||
  fail "snapshot public collision identity was reported as owned"

printf 'OK pinned snapshot ownership and retention races\n'
