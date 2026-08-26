#!/usr/bin/env bash
# shellcheck disable=SC2016,SC2329
set -euo pipefail

scripts_dir="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
test_root="$(mktemp -d)"
lock_test_pid=""
trap '[ -z "$lock_test_pid" ] || kill "$lock_test_pid" 2>/dev/null || true; rm -rf -- "$test_root"' EXIT

export HOME="$test_root/home"
export XDG_CONFIG_HOME="$HOME/.config"
export XDG_STATE_HOME="$HOME/.local/state"
export XDG_RUNTIME_DIR="$test_root/runtime"
mkdir -p "$HOME" "$XDG_RUNTIME_DIR"
chmod 0700 "$XDG_RUNTIME_DIR"

# shellcheck source=Linux/dots/hypr/scripts/shell-runtime.sh
. "$scripts_dir/shell-runtime.sh"

lock_race_dir="$test_root/stale-lock-race"
lock_known_recovery="$test_root/stale-lock-original"
mkdir -p "$lock_race_dir"
printf '%s\n' 999999 >"$lock_race_dir/pid"
printf '%s\n' wahrwelt-test-lock >"$lock_race_dir/owner"
lock_expected_identity="$(wahrwelt_lock_identity "$lock_race_dir")"
lock_swap_once=1
wahrwelt_before_lock_exchange_hook() {
  [ "$1" = "$lock_race_dir" ] || return 0
  if [ "$lock_swap_once" -eq 1 ]; then
    lock_swap_once=0
    command mv -- "$lock_race_dir" "$lock_known_recovery"
    mkdir -p "$lock_race_dir/unknown-tree"
    printf '%s\n' preserve >"$lock_race_dir/unknown-tree/preserve"
  fi
}
if stale_recovery="$(wahrwelt_release_owned_lock "$lock_race_dir" "$lock_expected_identity")"; then
  printf 'FAIL: swapped stale lock unexpectedly released at %s\n' "$stale_recovery" >&2
  exit 1
fi
unset -f wahrwelt_before_lock_exchange_hook
if [ "$(tr -d '\n' <"$lock_race_dir/unknown-tree/preserve")" != preserve ]; then
  printf 'FAIL: stale-lock cleanup changed swapped unknown tree\n' >&2
  exit 1
fi
[ -f "$lock_known_recovery/owner" ] || {
  printf 'FAIL: stale-lock race discarded original managed lock\n' >&2
  exit 1
}

assert_eq() {
  local want="$1"
  local got="$2"
  local message="$3"

  if [ "$got" != "$want" ]; then
    printf 'FAIL: %s: got %q, want %q\n' "$message" "$got" "$want" >&2
    exit 1
  fi
}

assert_matches() {
  local pattern="$1"
  local value="$2"
  local message="$3"

  if ! printf '%s\n' "$value" | grep -Eq -- "$pattern"; then
    printf 'FAIL: %s: %q does not match %q\n' "$message" "$value" "$pattern" >&2
    exit 1
  fi
}

assert_not_matches() {
  local pattern="$1"
  local value="$2"
  local message="$3"

  if printf '%s\n' "$value" | grep -Eq -- "$pattern"; then
    printf 'FAIL: %s: %q unexpectedly matches %q\n' "$message" "$value" "$pattern" >&2
    exit 1
  fi
}

normal_lock_dir="$test_root/normal-lock"
mkdir -p "$normal_lock_dir"
printf '%s\n' 999999 >"$normal_lock_dir/pid"
printf '%s\n' wahrwelt-shell-selector >"$normal_lock_dir/owner"
normal_lock_identity="$(wahrwelt_lock_identity "$normal_lock_dir")"
if ! wahrwelt_release_owned_lock "$normal_lock_dir" "$normal_lock_identity"; then
  printf 'FAIL: owned lock cleanup did not retain its exact recovery\n' >&2
  exit 1
fi
normal_lock_cleanup="$wahrwelt_lock_recovery_exact_path"
assert_eq "$normal_lock_identity" "$wahrwelt_lock_recovery_identity" \
  "owned lock cleanup binds its retained recovery identity"
assert_matches '^/.*/\.wahrwelt-lock-quarantine-[^/]+$' "$normal_lock_cleanup" \
  "owned lock cleanup reports its exact retained recovery"
if [ -e "$normal_lock_dir" ] || [ -L "$normal_lock_dir" ]; then
  printf 'FAIL: successful owned lock cleanup left the public lock name\n' >&2
  exit 1
fi
if [ "$(tr -d '\n' <"$normal_lock_cleanup/owner")" != wahrwelt-shell-selector ]; then
  printf 'FAIL: successful owned lock cleanup did not retain exact recovery\n' >&2
  exit 1
fi

cleanup_race_lock="$test_root/cleanup-lock-race"
mkdir -p "$cleanup_race_lock"
printf '%s\n' 999999 >"$cleanup_race_lock/pid"
printf '%s\n' wahrwelt-shell-selector >"$cleanup_race_lock/owner"
cleanup_race_identity="$(wahrwelt_lock_identity "$cleanup_race_lock")"
wahrwelt_before_lock_release_delete_hook() {
  mkdir -p "$1/unknown-tree"
  printf '%s\n' preserve >"$1/unknown-tree/preserve"
}
if wahrwelt_release_owned_lock "$cleanup_race_lock" "$cleanup_race_identity"; then
  printf 'FAIL: lock cleanup deleted a post-validation unknown child\n' >&2
  exit 1
fi
unset -f wahrwelt_before_lock_release_delete_hook
cleanup_recovery="$wahrwelt_lock_recovery_exact_path"
assert_eq "$cleanup_race_identity" "$wahrwelt_lock_recovery_identity" \
  "lock cleanup collision retains the classified lock identity"
if [ -z "$cleanup_recovery" ] || [ "$(tr -d '\n' <"$cleanup_recovery/unknown-tree/preserve")" != preserve ]; then
  printf 'FAIL: lock cleanup did not retain injected unknown child\n' >&2
  exit 1
fi

# A staged lock is not published until all metadata is durable. A same-UID
# target created immediately before publication must remain untouched.
new_lock_dir="$test_root/new-lock-race"
new_lock_swap_once=1
wahrwelt_before_lock_directory_publish_hook() {
  [ "$1" = "$new_lock_dir" ] || return 0
  [ "$new_lock_swap_once" -eq 1 ] || return 0
  new_lock_swap_once=0
  mkdir -p "$new_lock_dir"
  printf '%s\n' preserve >"$new_lock_dir/unknown"
}
if wahrwelt_acquire_lock "$new_lock_dir" "$new_lock_dir/pid" "$new_lock_dir/owner" \
  wahrwelt-shell-selector 'never-matches'; then
  printf 'FAIL: lock acquisition accepted a parent replacement after mkdir\n' >&2
  exit 1
fi
unset -f wahrwelt_before_lock_directory_publish_hook
if [ "$(tr -d '\n' <"$new_lock_dir/unknown")" != preserve ] ||
  [ -e "$new_lock_dir/pid" ] || [ -e "$new_lock_dir/owner" ]; then
  printf 'FAIL: lock acquisition wrote metadata into a replacement lock directory\n' >&2
  exit 1
fi

fifo_lock_dir="$test_root/fifo-lock-collision"
mkdir -p "$fifo_lock_dir"
mkfifo "$fifo_lock_dir/pid"
printf '%s\n' wahrwelt-shell-selector >"$fifo_lock_dir/owner"
if timeout 2 bash -c \
  '. "$1"; wahrwelt_acquire_lock "$2" "$2/pid" "$2/owner" wahrwelt-shell-selector never-matches 1 0' \
  bash "$scripts_dir/shell-runtime.sh" "$fifo_lock_dir"; then
  printf 'FAIL: FIFO lock collision unexpectedly acquired or did not fail closed\n' >&2
  exit 1
fi
if [ ! -p "$fifo_lock_dir/pid" ]; then
  printf 'FAIL: FIFO lock collision was changed while being rejected\n' >&2
  exit 1
fi

classified_lock_dir="$test_root/classified-stale-lock"
classified_lock_original="$test_root/classified-stale-original"
mkdir -p "$classified_lock_dir"
printf '%s\n' 999999 >"$classified_lock_dir/pid"
printf '%s\n' wahrwelt-shell-selector >"$classified_lock_dir/owner"
classified_lock_swap_once=1
wahrwelt_after_lock_classification_hook() {
  [ "$1" = "$classified_lock_dir" ] || return 0
  [ "$classified_lock_swap_once" -eq 1 ] || return 0
  classified_lock_swap_once=0
  mv -- "$classified_lock_dir" "$classified_lock_original"
  mkdir -p "$classified_lock_dir/unknown-tree"
  printf '%s\n' preserve >"$classified_lock_dir/unknown-tree/preserve"
}
if wahrwelt_acquire_lock "$classified_lock_dir" "$classified_lock_dir/pid" "$classified_lock_dir/owner" \
  wahrwelt-shell-selector never-matches 1 0; then
  printf 'FAIL: stale lock classification quarantined a replacement winner\n' >&2
  exit 1
fi
unset -f wahrwelt_after_lock_classification_hook
if [ "$(tr -d '\n' <"$classified_lock_dir/unknown-tree/preserve")" != preserve ] ||
  [ ! -f "$classified_lock_original/owner" ]; then
  printf 'FAIL: stale lock classification did not preserve both winner and original\n' >&2
  exit 1
fi

# Runtime state must never be updated in place through a hardlink whose other
# name lies outside the managed runtime tree.
hardlink_root="$test_root/runtime-hardlink"
hardlink_outside="$test_root/runtime-hardlink-outside"
hardlink_target="$hardlink_root/runtime-state"
mkdir -p "$hardlink_root"
printf '%s\n' outside-bytes >"$hardlink_outside"
ln -- "$hardlink_outside" "$hardlink_target"

wahrwelt_valid_shell_profile end4-pc
assert_eq end4 "$(wahrwelt_shell_family end4)" "Official family"
assert_eq end4 "$(wahrwelt_shell_family end4-pc)" "pC family"
assert_eq ii "$(wahrwelt_end4_quickshell_config end4)" "Official config"
assert_eq end4-pC "$(wahrwelt_end4_quickshell_config end4-pc)" "pC config"
assert_eq "$XDG_CONFIG_HOME/quickshell/ii" "$(wahrwelt_end4_quickshell_path end4)" "Official runtime path"
assert_eq "$XDG_CONFIG_HOME/quickshell/end4-pC" "$(wahrwelt_end4_quickshell_path end4-pc)" "pC runtime path"
assert_eq end4 "$(wahrwelt_read_end4_variant)" "missing state fallback"
assert_matches "$wahrwelt_end4_official_env_pattern" "WAHRWELT_END4_PROFILE=end4" "Official exact process marker"
assert_matches "$wahrwelt_end4_pc_env_pattern" "WAHRWELT_END4_PROFILE=end4-pc" "pC exact process marker"
assert_not_matches "$wahrwelt_end4_env_pattern" "qsConfig=$XDG_CONFIG_HOME/quickshell/ii" "generic qsConfig does not identify End4"
assert_not_matches "$wahrwelt_end4_env_pattern" "ILLOGICAL_IMPULSE_DOTFILES_SOURCE=$XDG_CONFIG_HOME" "generic End4 variable does not identify End4"
assert_not_matches "$wahrwelt_end4_env_pattern" "WAHRWELT_END4_PROFILE=caelestia" "Caelestia with stale generic End4 variables is not End4"

legacy_entrypoint="$test_root/legacy-hyprland.lua"
legacy_fixture_dir="$scripts_dir/../../../NixOS/home/shells/legacy-hypr-runtime"
for variant in end4 end4-pc; do
  cp -- "$legacy_fixture_dir/$variant.lua" "$legacy_entrypoint"
  wahrwelt_is_legacy_direct_end4_entrypoint "$legacy_entrypoint" "$XDG_CONFIG_HOME"

  {
    printf '%s\n' '-- prefix'
    cat "$legacy_fixture_dir/$variant.lua"
  } >"$legacy_entrypoint"
  if wahrwelt_is_legacy_direct_end4_entrypoint "$legacy_entrypoint" "$XDG_CONFIG_HOME"; then
    printf 'FAIL: prefixed %s legacy payload unexpectedly matched\n' "$variant" >&2
    exit 1
  fi

  {
    cat "$legacy_fixture_dir/$variant.lua"
    printf '%s\n' '-- suffix'
  } >"$legacy_entrypoint"
  if wahrwelt_is_legacy_direct_end4_entrypoint "$legacy_entrypoint" "$XDG_CONFIG_HOME"; then
    printf 'FAIL: suffixed %s legacy payload unexpectedly matched\n' "$variant" >&2
    exit 1
  fi

  legacy_text="$(cat "$legacy_fixture_dir/$variant.lua")"
  printf '%s' "$legacy_text" >"$legacy_entrypoint"
  if wahrwelt_is_legacy_direct_end4_entrypoint "$legacy_entrypoint" "$XDG_CONFIG_HOME"; then
    printf 'FAIL: %s legacy payload without final newline unexpectedly matched\n' "$variant" >&2
    exit 1
  fi
done
printf 'dofile("%s/hypr/end4/hyprland.lua")\n' "$XDG_CONFIG_HOME" >"$legacy_entrypoint"
if wahrwelt_is_legacy_direct_end4_entrypoint "$legacy_entrypoint" "$XDG_CONFIG_HOME"; then
  printf 'FAIL: unsupported absolute End4 shorthand unexpectedly matched\n' >&2
  exit 1
fi
printf '%s\n' '-- Active Hyprland profile: end4' 'dofile(end4_root .. "/hyprland.lua")' >"$legacy_entrypoint"
if wahrwelt_is_legacy_direct_end4_entrypoint "$legacy_entrypoint" "$XDG_CONFIG_HOME"; then
  printf 'FAIL: truncated End4 payload unexpectedly matched\n' >&2
  exit 1
fi

legacy_user_entrypoint="$test_root/legacy-user-hyprland.lua"
for payload in runtime home-manager; do
  case "$payload" in
    runtime) wahrwelt_print_legacy_user_entrypoint >"$legacy_user_entrypoint" ;;
    home-manager) wahrwelt_print_legacy_home_manager_user_entrypoint >"$legacy_user_entrypoint" ;;
  esac
  wahrwelt_is_legacy_user_entrypoint "$legacy_user_entrypoint"
  legacy_user_text="$(cat "$legacy_user_entrypoint")"
  for form in prefix suffix missing-newline arbitrary-root; do
    case "$form" in
      prefix) printf '%s\n%s\n' '-- prefix' "$legacy_user_text" >"$legacy_user_entrypoint" ;;
      suffix) printf '%s\n%s\n' "$legacy_user_text" '-- suffix' >"$legacy_user_entrypoint" ;;
      missing-newline) printf '%s' "$legacy_user_text" >"$legacy_user_entrypoint" ;;
      arbitrary-root) printf '%s\n' 'dofile("/tmp/arbitrary/wahrwelt/hyprland.lua")' >"$legacy_user_entrypoint" ;;
    esac
    if wahrwelt_is_legacy_user_entrypoint "$legacy_user_entrypoint"; then
      printf 'FAIL: non-legacy-user %s %s payload unexpectedly matched\n' "$payload" "$form" >&2
      exit 1
    fi
  done
done

canonical_entrypoint="$test_root/canonical-hyprland.lua"
for payload in runtime home-manager stable; do
  case "$payload" in
    runtime) wahrwelt_print_canonical_runtime_entrypoint >"$canonical_entrypoint" ;;
    home-manager) wahrwelt_print_home_manager_initial_entrypoint >"$canonical_entrypoint" ;;
    stable) wahrwelt_print_stable_runtime_entrypoint >"$canonical_entrypoint" ;;
  esac
  wahrwelt_is_canonical_entrypoint "$canonical_entrypoint"
done

wahrwelt_print_canonical_runtime_entrypoint >"$canonical_entrypoint"
canonical_text="$(cat "$canonical_entrypoint")"
for form in prefix suffix missing-newline absolute-shorthand arbitrary-root; do
  case "$form" in
    prefix) printf '%s\n%s\n' '-- prefix' "$canonical_text" >"$canonical_entrypoint" ;;
    suffix) printf '%s\n%s\n' "$canonical_text" '-- suffix' >"$canonical_entrypoint" ;;
    missing-newline) printf '%s' "$canonical_text" >"$canonical_entrypoint" ;;
    absolute-shorthand) printf 'dofile("%s/hypr/wahrwelt/hyprland.lua")\n' "$XDG_CONFIG_HOME" >"$canonical_entrypoint" ;;
    arbitrary-root) printf '%s\n' 'dofile("/tmp/arbitrary/wahrwelt/hyprland.lua")' >"$canonical_entrypoint" ;;
  esac
  if wahrwelt_is_canonical_entrypoint "$canonical_entrypoint"; then
    printf 'FAIL: non-canonical %s payload unexpectedly matched\n' "$form" >&2
    exit 1
  fi
done

selector_script="$scripts_dir/shell-selector.sh"
if ! grep -Fq 'if wahrwelt_is_canonical_entrypoint "$entrypoint_path"; then' "$selector_script" ||
  ! grep -Fq 'if wahrwelt_is_legacy_user_entrypoint "$entrypoint_path"; then' "$selector_script" ||
  ! grep -Fq 'if wahrwelt_is_legacy_direct_end4_entrypoint "$entrypoint_path" "$config_home"; then' "$selector_script"; then
  printf 'FAIL: shell selector does not delegate entrypoint recognition to exact payload matchers\n' >&2
  exit 1
fi
if grep -Eq 'grep .*wahrwelt/hyprland|grep .*end4/hyprland' "$selector_script"; then
  printf 'FAIL: shell selector retains line or substring entrypoint recognition\n' >&2
  exit 1
fi

start_shell="$scripts_dir/start-shell.sh"
# shellcheck disable=SC2016
for marker in \
  'WAHRWELT_END4_PROFILE="$profile"' \
  'WAHRWELT_QS_CONFIG="$end4_quickshell_path"' \
  'qsConfig="$end4_quickshell_path"' \
  'ILLOGICAL_IMPULSE_DOTFILES_SOURCE="$wahrwelt_config_home"' \
  'ILLOGICAL_IMPULSE_VIRTUAL_ENV="$wahrwelt_state_home/quickshell/.venv"'; do
  if ! grep -Fq -- "$marker" "$start_shell"; then
    printf 'FAIL: start-shell local End4 launch environment missing %s\n' "$marker" >&2
    exit 1
  fi
done

if ! grep -Fq 'ensure_end4_idle || return 1' "$start_shell"; then
  printf 'FAIL: End4 startup does not propagate managed hypridle failure\n' >&2
  exit 1
fi
if grep -Fq 'ensure_end4_idle || true' "$start_shell"; then
  printf 'FAIL: End4 startup still ignores managed hypridle failure\n' >&2
  exit 1
fi

start_profile_function="$test_root/start-profile-shell.sh"
awk '
  /^start_profile_shell\(\) \{$/ { capture = 1 }
  capture { print }
  capture && /^}$/ { exit }
' "$start_shell" >"$start_profile_function"
if ! grep -Fqx 'start_profile_shell() {' "$start_profile_function"; then
  printf 'FAIL: could not extract actual start_profile_shell implementation\n' >&2
  exit 1
fi

if (
  # shellcheck source=/dev/null
  . "$start_profile_function"
  profile=end4
  end4_handle=end4-family
  end4_official_handle=end4-official
  end4_pc_handle=end4-pc
  log() { :; }
  ensure_end4_idle() { return 1; }
  dedupe_shell() { return 0; }
  is_running() { return 1; }
  command() { return 0; }
  start_with_retry() { return 0; }
  start_profile_shell
); then
  printf 'FAIL: actual End4 start_profile_shell accepted managed hypridle failure\n' >&2
  exit 1
fi

selector_handle=selector
caelestia_handle=caelestia
caelestia_resizer_handle=caelestia-resizer
noctalia_handle=noctalia
end4_handle=end4-family
end4_official_handle=end4-official
end4_pc_handle=end4-pc
end4_idle_handle=end4-idle
end4_idle_config="$wahrwelt_hypr_runtime_dir/hypridle.conf"
user_name="$wahrwelt_user_name"
selector_pattern="$wahrwelt_selector_pattern"
caelestia_pattern="$wahrwelt_caelestia_pattern"
end4_env_pattern="$wahrwelt_end4_env_pattern"
fake_process_env="WAHRWELT_END4_PROFILE=caelestia
qsConfig=$XDG_CONFIG_HOME/quickshell/ii
ILLOGICAL_IMPULSE_DOTFILES_SOURCE=$XDG_CONFIG_HOME"

wahrwelt_quickshell_pids() {
  printf '%s\n' 4242
}

wahrwelt_pid_has_env_regex() {
  printf '%s\n' "$fake_process_env" | grep -qE -- "$2"
}

# shellcheck source=Linux/dots/hypr/scripts/shell-process.sh
. "$scripts_dir/shell-process.sh"

assert_eq "" "$(matching_pids "$end4_handle")" "Caelestia with stale generic End4 variables stays outside End4 family"
fake_process_env="WAHRWELT_END4_PROFILE=end4-pc
qsConfig=$XDG_CONFIG_HOME/quickshell/end4-pC"
assert_eq 4242 "$(matching_pids "$end4_pc_handle")" "exact pC process marker identifies variant"

fake_quickshell_pid_list=$'5101\n5102\n5103\n5104\n5105\n5106\n5107\n5109'
wahrwelt_quickshell_pids() {
  printf '%s\n' "$fake_quickshell_pid_list"
}
wahrwelt_pid_has_env_regex() {
  local process_env=""

  case "$1" in
    5101 | 5102 | 5106 | 5107 | 5109) process_env="" ;;
    5103) process_env='WAHRWELT_END4_PROFILE=end4' ;;
    5104) process_env='WAHRWELT_END4_PROFILE=end4-pc' ;;
    5105) process_env="qsConfig=$XDG_CONFIG_HOME/quickshell/ii
ILLOGICAL_IMPULSE_DOTFILES_SOURCE=$XDG_CONFIG_HOME" ;;
  esac
  printf '%s\n' "$process_env" | grep -qE -- "$2"
}
wahrwelt_pid_has_adjacent_args() {
  local config=""

  [ "$2" = -c ] || return 1
  case "$1" in
    5101 | 5103 | 5109) config=ii ;;
    5102 | 5104) config=end4-pC ;;
    5105) config=caelestia ;;
    5106) config=custom-shell ;;
    5107) config=ii-extra ;;
  esac
  [ "$config" = "$3" ]
}
wahrwelt_pid_is_legacy_end4_upgrade_token() {
  local token="$1"
  local pid="${token%%:*}"

  case "$token" in
    5101:101:ii | 5102:102:end4-pC) ;;
    *) return 1 ;;
  esac
  printf '%s\n' "$fake_quickshell_pid_list" | grep -Fqx -- "$pid"
}

assert_eq "" "$(wahrwelt_legacy_end4_upgrade_pids "")" \
  "normal runtime never scans unmarked QuickShell argv without positive upgrade provenance"
assert_eq $'5101\n5102' "$(wahrwelt_legacy_end4_upgrade_pids '5101:101:ii,5102:102:end4-pC')" \
  "positive migration provenance binds cleanup to exact historical process identities"
assert_eq $'5103\n5104' "$(matching_pids "$end4_handle")" \
  "canonical End4 runtime still uses only the exact environment marker"

killed_legacy_pids=""
kill() {
  local pid="$2"

  killed_legacy_pids+="${killed_legacy_pids:+$'\n'}$pid"
  fake_quickshell_pid_list="$(printf '%s\n' "$fake_quickshell_pid_list" | grep -vx -- "$pid" || true)"
}
sleep() {
  :
}
log() {
  :
}
wahrwelt_open_end4_upgrade_state
legacy_end4_upgrade_tokens="$(
  wahrwelt_merge_end4_upgrade_tokens '5101:101:ii,5102:102:end4-pC'
)"
cleanup_legacy_end4_processes
assert_eq $'5101\n5102' "$(printf '%s\n' "$killed_legacy_pids" | sort -u)" \
  "upgrade cleanup stops both historical End4 variants"
assert_eq "" "$legacy_end4_upgrade_tokens" "upgrade cleanup consumes process provenance exactly once"
cleanup_legacy_end4_processes
assert_eq $'5101\n5102' "$(printf '%s\n' "$killed_legacy_pids" | sort -u)" \
  "consumed upgrade provenance cannot kill a later process"
if ! printf '%s\n' "$fake_quickshell_pid_list" | grep -Fqx 5109; then
  printf 'FAIL: unrelated unmarked qs -c ii process was killed during proven upgrade cleanup\n' >&2
  exit 1
fi

fake_quickshell_pid_list+=$'\n5108'
wahrwelt_pid_has_env_regex() {
  local process_env=""

  case "$1" in
    5103 | 5108) process_env='WAHRWELT_END4_PROFILE=end4' ;;
    5104) process_env='WAHRWELT_END4_PROFILE=end4-pc' ;;
    5105) process_env="qsConfig=$XDG_CONFIG_HOME/quickshell/ii
ILLOGICAL_IMPULSE_DOTFILES_SOURCE=$XDG_CONFIG_HOME" ;;
  esac
  printf '%s\n' "$process_env" | grep -qE -- "$2"
}
wahrwelt_pid_has_adjacent_args() {
  local config=""

  [ "$2" = -c ] || return 1
  case "$1" in
    5103 | 5108 | 5109) config=ii ;;
    5104) config=end4-pC ;;
    5105) config=caelestia ;;
    5106) config=custom-shell ;;
    5107) config=ii-extra ;;
  esac
  [ "$config" = "$3" ]
}
assert_eq "" "$(wahrwelt_legacy_end4_upgrade_pids "$legacy_end4_upgrade_tokens")" \
  "cleanup does not retain a permanent argv recognizer"
assert_eq $'5103\n5108' "$(matching_pids "$end4_official_handle")" \
  "historical process replacement leaves only exact-marked Official End4 instances"

unset -f kill log sleep wahrwelt_pid_is_legacy_end4_upgrade_token

proc_fixture="$test_root/proc"
for pid in 7101 7102 7103 7104 7105; do
  mkdir -p "$proc_fixture/$pid"
done
printf '%s\0' hypridle --verbose -c "$end4_idle_config" --debug >"$proc_fixture/7101/cmdline"
printf '%s\0' hypridle -c "${end4_idle_config}.unmanaged" >"$proc_fixture/7102/cmdline"
printf '%s\0' hypridle "--label=-c" "$end4_idle_config" -c "$test_root/other.conf" >"$proc_fixture/7103/cmdline"
printf '%s\0' hypridle -c "prefix${end4_idle_config}" >"$proc_fixture/7104/cmdline"
printf '%s\0' hypridle --debug --socket test -c "$end4_idle_config" --verbose >"$proc_fixture/7105/cmdline"

managed_idle_pids="$({
  pgrep() {
    printf '%s\n' 7101 7102 7103 7104 7105
  }
  wahrwelt_pid_has_adjacent_args() {
    wahrwelt_cmdline_has_adjacent_args "$proc_fixture/$1/cmdline" "$2" "$3"
  }
  matching_pids "$end4_idle_handle"
})"
assert_eq $'7101\n7105' "$managed_idle_pids" "only exact adjacent End4 hypridle argv matches the stop handle"

mkdir -p "$(dirname -- "$wahrwelt_end4_variant_state")"
printf '%s\n' end4-pc >"$wahrwelt_end4_variant_state"
assert_eq end4-pc "$(wahrwelt_read_end4_variant)" "remembered pC variant"

printf '%s\n' '../../arbitrary-command' >"$wahrwelt_end4_variant_state"
assert_eq end4 "$(wahrwelt_read_end4_variant)" "invalid state fallback"

printf '%s' end4-pc >"$wahrwelt_end4_variant_state"
assert_eq end4 "$(wahrwelt_read_end4_variant)" "missing newline fallback"

printf 'e n d 4-pc\n' >"$wahrwelt_end4_variant_state"
assert_eq end4 "$(wahrwelt_read_end4_variant)" "embedded whitespace fallback"

printf 'end4-pc\n\n' >"$wahrwelt_end4_variant_state"
assert_eq end4 "$(wahrwelt_read_end4_variant)" "extra line fallback"

printf '%s\n' end4-pc >"$wahrwelt_end4_variant_state"

hypr_runtime_dir="$wahrwelt_hypr_runtime_dir"
persistent_state_file="$wahrwelt_active_shell_state"
profile=end4-pc

log() {
  :
}

hypr_dir() {
  wahrwelt_hypr_dir_path
}

# shellcheck source=Linux/dots/hypr/scripts/shell-profile-sync.sh
. "$scripts_dir/shell-profile-sync.sh"

new_exact_test_snapshot() {
  wahrwelt_begin_exact_snapshot "$wahrwelt_runtime_session_dir" "$1" test
}

if write_regular_file "$hardlink_target" managed-runtime; then
  printf 'FAIL: runtime hardlink publication unexpectedly succeeded\n' >&2
  exit 1
fi
assert_eq outside-bytes "$(tr -d '\n' <"$hardlink_outside")" \
  "runtime hardlink collision leaves external bytes unchanged"

# A link added after the initial nlink check is still harmless: replacement is
# candidate/exchange based, so the external alias is never truncated.  The
# changed old inode makes the transaction fail closed and is restored intact.
late_hardlink_target="$test_root/runtime-late-hardlink"
late_hardlink_outside="$test_root/runtime-late-hardlink-outside"
printf '%s\n' managed-before-link >"$late_hardlink_target"
wahrwelt_before_runtime_candidate_exchange_hook() {
  [ "$1" = "$late_hardlink_target" ] || return 0
  ln -- "$late_hardlink_target" "$late_hardlink_outside"
}
if write_regular_file "$late_hardlink_target" managed-after-link; then
  printf 'FAIL: runtime publication accepted a post-preflight hardlink\n' >&2
  exit 1
fi
unset -f wahrwelt_before_runtime_candidate_exchange_hook
assert_eq managed-before-link "$(tr -d '\n' <"$late_hardlink_target")" \
  "post-preflight hardlink restores the canonical managed entry"
assert_eq managed-before-link "$(tr -d '\n' <"$late_hardlink_outside")" \
  "post-preflight hardlink leaves the external alias unchanged"

# Publishing is not committed until every enclosing transaction journal has
# accepted the exact post-publication identity.  A journal write failure must
# immediately restore a pre-existing entry, and must leave an absent target
# absent while retaining its candidate for recovery.
record_failure_target="$test_root/record-failure-existing"
printf '%s\n' prior-record-failure >"$record_failure_target"
new_exact_test_snapshot .record-failure-
record_failure_snapshot="$wahrwelt_new_snapshot_dir"
snapshot_exact_paths "$record_failure_snapshot" "$record_failure_target"
record_exact_snapshot_mutation_original="$(declare -f record_exact_snapshot_mutation)"
record_exact_snapshot_mutation() {
  [ "$1" = "$record_failure_target" ] && return 1
  return 1
}
if write_regular_file "$record_failure_target" candidate-record-failure; then
  printf 'FAIL: publication unexpectedly committed after journal rejection\n' >&2
  exit 1
fi
assert_eq prior-record-failure "$(tr -d '\n' <"$record_failure_target")" \
  "journal rejection restores the exact prior regular entry"
eval "$record_exact_snapshot_mutation_original"
wahrwelt_unregister_exact_snapshot "$record_failure_snapshot"

record_failure_absent="$test_root/record-failure-absent"
new_exact_test_snapshot .record-failure-absent-
record_failure_absent_snapshot="$wahrwelt_new_snapshot_dir"
snapshot_exact_paths "$record_failure_absent_snapshot" "$record_failure_absent"
record_exact_snapshot_mutation_original="$(declare -f record_exact_snapshot_mutation)"
record_exact_snapshot_mutation() {
  [ "$1" = "$record_failure_absent" ] && return 1
  return 1
}
if write_regular_file "$record_failure_absent" candidate-record-failure; then
  printf 'FAIL: absent publication unexpectedly committed after journal rejection\n' >&2
  exit 1
fi
if [ -e "$record_failure_absent" ] || [ -L "$record_failure_absent" ]; then
  printf 'FAIL: journal rejection left an owned candidate at the absent target\n' >&2
  exit 1
fi
record_failure_recovery="$(find "$test_root" -maxdepth 1 -type f -name '.wahrwelt-runtime-quarantine-*' -print -quit)"
if [ -z "$record_failure_recovery" ] ||
  [ "$(tr -d '\n' <"$record_failure_recovery")" != candidate-record-failure ]; then
  printf 'FAIL: journal rejection did not retain the absent candidate for recovery\n' >&2
  exit 1
fi
eval "$record_exact_snapshot_mutation_original"
wahrwelt_unregister_exact_snapshot "$record_failure_absent_snapshot"

cleanup_snapshot_target="$test_root/snapshot-cleanup-target"
printf '%s\n' snapshot-original >"$cleanup_snapshot_target"
new_exact_test_snapshot .cleanup-snapshot-
cleanup_snapshot_dir="$wahrwelt_new_snapshot_dir"
snapshot_exact_paths "$cleanup_snapshot_dir" "$cleanup_snapshot_target"
wahrwelt_before_snapshot_cleanup_delete_hook() {
  mkdir -p "$1/unknown-tree"
  printf '%s\n' preserve >"$1/unknown-tree/preserve"
}
if remove_exact_path_snapshot "$cleanup_snapshot_dir" "$cleanup_snapshot_target"; then
  printf 'FAIL: snapshot cleanup deleted a post-validation unknown child\n' >&2
  exit 1
fi
unset -f wahrwelt_before_snapshot_cleanup_delete_hook
cleanup_snapshot_recovery="$cleanup_snapshot_dir"
if [ "$(tr -d '\n' <"$cleanup_snapshot_recovery/unknown-tree/preserve")" != preserve ]; then
  printf 'FAIL: snapshot cleanup did not retain injected unknown child\n' >&2
  exit 1
fi
wahrwelt_unregister_exact_snapshot "$cleanup_snapshot_dir"

# A committed replacement keeps the prior regular inode in a named stage file
# until its last transaction snapshot is discarded. The stage stays as a
# recovery because a same-UID writer can replace any pathname before unlink.
stage_cleanup_target="$test_root/stage-cleanup-target"
printf '%s\n' stage-cleanup-original >"$stage_cleanup_target"
new_exact_test_snapshot .stage-cleanup-
stage_cleanup_snapshot="$wahrwelt_new_snapshot_dir"
snapshot_exact_paths "$stage_cleanup_snapshot" "$stage_cleanup_target"
write_regular_file "$stage_cleanup_target" stage-cleanup-current
stage_cleanup_stage="${wahrwelt_snapshot_owned_recoveries[$(snapshot_parent_key "$stage_cleanup_snapshot" 0)]:-}"
if [ -z "$stage_cleanup_stage" ]; then
  printf 'FAIL: replacement did not retain a stage recovery before commit\n' >&2
  exit 1
fi
if ! remove_exact_path_snapshot "$stage_cleanup_snapshot" "$stage_cleanup_target"; then
  printf 'FAIL: committed transaction snapshot did not retain its exact recovery\n' >&2
  exit 1
fi
if [ "$(tr -d '\n' <"$stage_cleanup_stage")" != stage-cleanup-original ]; then
  printf 'FAIL: committed transaction did not retain the exact stage recovery\n' >&2
  exit 1
fi
assert_eq stage-cleanup-current "$(tr -d '\n' <"$stage_cleanup_target")" \
  "stage cleanup preserves the committed runtime result"

# Cleanup binds the stage identity before retention. A same-UID replacement
# at the deterministic cleanup barrier is a collision, not a deletion target.
stage_cleanup_race_target="$test_root/stage-cleanup-race-target"
printf '%s\n' stage-cleanup-race-original >"$stage_cleanup_race_target"
new_exact_test_snapshot .stage-cleanup-race-
stage_cleanup_race_snapshot="$wahrwelt_new_snapshot_dir"
snapshot_exact_paths "$stage_cleanup_race_snapshot" "$stage_cleanup_race_target"
write_regular_file "$stage_cleanup_race_target" stage-cleanup-race-current
stage_cleanup_race_stage="${wahrwelt_snapshot_owned_recoveries[$(snapshot_parent_key "$stage_cleanup_race_snapshot" 0)]:-}"
if [ -z "$stage_cleanup_race_stage" ]; then
  printf 'FAIL: race cleanup setup did not retain a stage recovery\n' >&2
  exit 1
fi
wahrwelt_before_runtime_stage_cleanup_hook() {
  mv -T --no-copy -- "$stage_cleanup_race_stage" "$stage_cleanup_race_stage.owned"
  printf '%s\n' concurrent-winner >"$stage_cleanup_race_stage"
}
if remove_exact_path_snapshot "$stage_cleanup_race_snapshot" "$stage_cleanup_race_target"; then
  printf 'FAIL: stage cleanup accepted a replacement winner\n' >&2
  exit 1
fi
unset -f wahrwelt_before_runtime_stage_cleanup_hook
assert_eq concurrent-winner "$(tr -d '\n' <"$stage_cleanup_race_stage")" \
  "stage cleanup preserves a concurrent replacement winner"
assert_eq stage-cleanup-race-original "$(tr -d '\n' <"$stage_cleanup_race_stage.owned")" \
  "stage cleanup retains the owned recovery after a collision"
wahrwelt_unregister_exact_snapshot "$stage_cleanup_race_snapshot"

# Managed legacy generated regular files are quarantined with a recovery path
# that rollback can consume. The path is part of the transaction journal, not
# merely a diagnostic log line.
legacy_regular_path="$hypr_runtime_dir/shell-profile.conf"
mkdir -p "$(dirname -- "$legacy_regular_path")" "$wahrwelt_hypr_dir/wahrwelt"
legacy_regular_expected="$(printf '# Runtime shell launcher\nexec-once = %s\n' "$(hypr_dir)/scripts/start-shell.sh")"
printf '%s\n' "$legacy_regular_expected" >"$legacy_regular_path"
new_exact_test_snapshot .legacy-regular-
legacy_regular_snapshot="$wahrwelt_new_snapshot_dir"
mapfile -t legacy_runtime_paths < <(legacy_hyprland_runtime_paths)
snapshot_exact_paths "$legacy_regular_snapshot" "${legacy_runtime_paths[@]}"
prune_legacy_hyprland_runtime_files
if [ -e "$legacy_regular_path" ] || [ -L "$legacy_regular_path" ]; then
  printf 'FAIL: known legacy regular runtime was not quarantined\n' >&2
  exit 1
fi
if ! restore_exact_paths "$legacy_regular_snapshot" "${legacy_runtime_paths[@]}"; then
  printf 'FAIL: known legacy regular runtime did not restore from recovery\n' >&2
  exit 1
fi
assert_eq "$legacy_regular_expected" "$(cat "$legacy_regular_path")" \
  "legacy regular runtime rollback restores exact bytes"
wahrwelt_unregister_exact_snapshot "$legacy_regular_snapshot"

# The symlink proof is equally exact. Use a later managed candidate so prune
# can quarantine the link and its target in deterministic order.
legacy_link_path="$hypr_runtime_dir/shell-profile.conf"
legacy_link_target="$hypr_runtime_dir/shell-launcher.conf"
legacy_link_expected="$(printf '# Active shell launcher profile: end4\nsource = %s\n' "$(hypr_dir)/end4/launcher.conf")"
printf '%s\n' "$legacy_link_expected" >"$legacy_link_target"
rm -f -- "$legacy_link_path"
ln -s -- "$legacy_link_target" "$legacy_link_path"
new_exact_test_snapshot .legacy-link-
legacy_link_snapshot="$wahrwelt_new_snapshot_dir"
mapfile -t legacy_runtime_paths < <(legacy_hyprland_runtime_paths)
snapshot_exact_paths "$legacy_link_snapshot" "${legacy_runtime_paths[@]}"
prune_legacy_hyprland_runtime_files
if [ -e "$legacy_link_path" ] || [ -L "$legacy_link_path" ] ||
  [ -e "$legacy_link_target" ] || [ -L "$legacy_link_target" ]; then
  printf 'FAIL: known legacy runtime symlink and target were not quarantined\n' >&2
  exit 1
fi
if ! restore_exact_paths "$legacy_link_snapshot" "${legacy_runtime_paths[@]}"; then
  printf 'FAIL: known legacy runtime symlink rollback failed\n' >&2
  exit 1
fi
if [ ! -L "$legacy_link_path" ] || [ "$(readlink -- "$legacy_link_path")" != "$legacy_link_target" ]; then
  printf 'FAIL: legacy runtime symlink rollback did not restore exact link\n' >&2
  exit 1
fi
wahrwelt_unregister_exact_snapshot "$legacy_link_snapshot"

# Pruning uses the transaction's retained parent descriptor. A canonical
# parent replacement after snapshot therefore stays untouched.
rm -f -- "$legacy_link_path" "$legacy_link_target"
printf '%s\n' "$legacy_regular_expected" >"$legacy_regular_path"
new_exact_test_snapshot .legacy-prune-parent-
legacy_prune_snapshot="$wahrwelt_new_snapshot_dir"
mapfile -t legacy_runtime_paths < <(legacy_hyprland_runtime_paths)
snapshot_exact_paths "$legacy_prune_snapshot" "${legacy_runtime_paths[@]}"
legacy_prune_original="$test_root/legacy-prune-original-runtime"
legacy_prune_swap_once=1
wahrwelt_before_runtime_quarantine_exchange_hook() {
  [ "$legacy_prune_swap_once" -eq 1 ] || return 0
  legacy_prune_swap_once=0
  mv -- "$hypr_runtime_dir" "$legacy_prune_original"
  mkdir -p "$hypr_runtime_dir"
  printf '%s\n' canonical-winner >"$hypr_runtime_dir/shell-profile.conf"
}
prune_legacy_hyprland_runtime_files
unset -f wahrwelt_before_runtime_quarantine_exchange_hook
assert_eq canonical-winner "$(tr -d '\n' <"$hypr_runtime_dir/shell-profile.conf")" \
  "legacy prune preserves canonical parent-swap winner"
wahrwelt_unregister_exact_snapshot "$legacy_prune_snapshot"

parent_swap_root="$test_root/absent-parent-swap"
managed_parent="$parent_swap_root/managed"
managed_target="$managed_parent/absent.lua"
mkdir -p "$managed_parent"
new_exact_test_snapshot .parent-rollback-
parent_snapshot="$wahrwelt_new_snapshot_dir"
snapshot_exact_paths "$parent_snapshot" "$managed_target"
mv -- "$managed_parent" "$parent_swap_root/original-managed"
mkdir -p "$parent_swap_root/victim"
mv -- "$parent_swap_root/victim" "$managed_parent"
if preflight_regular_file_target "$managed_target"; then
  printf 'FAIL: absent target under swapped ordinary parent passed preflight\n' >&2
  exit 1
fi
if [ -e "$managed_target" ] || [ -L "$managed_target" ]; then
  printf 'FAIL: absent target parent-swap preflight wrote into victim\n' >&2
  exit 1
fi
wahrwelt_unregister_exact_snapshot "$parent_snapshot"

write_swap_root="$test_root/write-parent-swap"
write_swap_parent="$write_swap_root/managed"
write_swap_target="$write_swap_parent/runtime.lua"
mkdir -p "$write_swap_parent"
printf '%s\n' original-runtime >"$write_swap_target"
new_exact_test_snapshot .write-parent-rollback-
write_swap_snapshot="$wahrwelt_new_snapshot_dir"
snapshot_exact_paths "$write_swap_snapshot" "$write_swap_target"
write_swap_once=1
wahrwelt_after_runtime_preflight_hook() {
  [ "$1" = "$write_swap_target" ] || return 0
  [ "$write_swap_once" -eq 1 ] || return 0
  write_swap_once=0
  mv -- "$write_swap_parent" "$write_swap_root/original-managed"
  mkdir -p "$write_swap_parent"
  printf '%s\n' concurrent-runtime-winner >"$write_swap_target"
}
if write_regular_file "$write_swap_target" managed-runtime; then
  printf 'FAIL: runtime write succeeded after canonical parent swap\n' >&2
  exit 1
fi
unset -f wahrwelt_after_runtime_preflight_hook
assert_eq concurrent-runtime-winner "$(tr -d '\n' <"$write_swap_target")" \
  "pinned runtime write preserves canonical parent-swap winner"
wahrwelt_unregister_exact_snapshot "$write_swap_snapshot"

snapshot_swap_root="$test_root/snapshot-parent-swap"
snapshot_swap_parent="$snapshot_swap_root/managed"
snapshot_swap_target="$snapshot_swap_parent/absent-runtime-state"
mkdir -p "$snapshot_swap_parent"
new_exact_test_snapshot .snapshot-parent-rollback-
snapshot_swap_dir="$wahrwelt_new_snapshot_dir"
snapshot_swap_once=1
wahrwelt_before_snapshot_parent_pin_hook() {
  [ "$1" = "$snapshot_swap_target" ] || return 0
  [ "$snapshot_swap_once" -eq 1 ] || return 0
  snapshot_swap_once=0
  mv -- "$snapshot_swap_parent" "$snapshot_swap_root/original-managed"
  mkdir -p "$snapshot_swap_parent"
}
if snapshot_exact_paths "$snapshot_swap_dir" "$snapshot_swap_target"; then
  printf 'FAIL: snapshot accepted an empty target after its parent swapped before pinning\n' >&2
  exit 1
fi
unset -f wahrwelt_before_snapshot_parent_pin_hook
if [ -e "$snapshot_swap_target" ] || [ -L "$snapshot_swap_target" ]; then
  printf 'FAIL: snapshot parent-swap preflight created victim target\n' >&2
  exit 1
fi

missing_snapshot_target="$test_root/snapshot-missing-parent/managed/absent-runtime-state"
new_exact_test_snapshot .snapshot-missing-parent-
missing_snapshot_dir="$wahrwelt_new_snapshot_dir"
if snapshot_exact_paths "$missing_snapshot_dir" "$missing_snapshot_target"; then
  printf 'FAIL: snapshot accepted a target with no begin-time parent anchor\n' >&2
  exit 1
fi
if [ -e "$missing_snapshot_target" ] || [ -L "$missing_snapshot_target" ]; then
  printf 'FAIL: missing-parent snapshot created a target\n' >&2
  exit 1
fi

assert_pinned_rollback_preserves_swapped_parent() {
  local prior_kind="$1"
  local rollback_root="$test_root/rollback-parent-$prior_kind"
  local rollback_parent="$rollback_root/managed"
  local rollback_target="$rollback_parent/runtime-state"
  local rollback_original="$rollback_root/original-managed"
  local rollback_snapshot owned_type owned_identity owned_parent old_source new_source

  mkdir -p "$rollback_parent"
  case "$prior_kind" in
    regular)
      printf '%s\n' prior-regular >"$rollback_target"
      ;;
    symlink)
      old_source="$rollback_root/prior-link-source"
      printf '%s\n' prior-symlink >"$old_source"
      ln -s -- "$old_source" "$rollback_target"
      ;;
    absent)
      ;;
  esac
  new_exact_test_snapshot .rollback-parent-
  rollback_snapshot="$wahrwelt_new_snapshot_dir"
  snapshot_exact_paths "$rollback_snapshot" "$rollback_target"
  case "$prior_kind" in
    regular)
      printf '%s\n' transaction-regular >"$rollback_target"
      owned_type=regular
      ;;
    symlink)
      new_source="$rollback_root/transaction-link-source"
      printf '%s\n' transaction-symlink >"$new_source"
      rm -f -- "$rollback_target"
      ln -s -- "$new_source" "$rollback_target"
      owned_type=symlink
      ;;
    absent)
      printf '%s\n' transaction-created >"$rollback_target"
      owned_type=regular
      ;;
  esac
  owned_identity="$(runtime_state_identity "$rollback_target")"
  owned_parent="$(runtime_parent_identity "$rollback_target")"
  record_exact_snapshot_mutation "$rollback_target" "$owned_type" "$owned_identity" "$owned_parent"
  mv -- "$rollback_parent" "$rollback_original"
  mkdir -p "$rollback_parent"
  printf '%s\n' canonical-winner >"$rollback_target"
  if ! restore_exact_paths "$rollback_snapshot" "$rollback_target"; then
    printf 'FAIL: %s rollback did not use pinned original parent after canonical swap\n' "$prior_kind" >&2
    exit 1
  fi
  assert_eq canonical-winner "$(tr -d '\n' <"$rollback_target")" \
    "$prior_kind rollback preserves canonical parent-swap winner"
  case "$prior_kind" in
    regular)
      assert_eq prior-regular "$(tr -d '\n' <"$rollback_original/runtime-state")" \
        "regular rollback restores detached original parent"
      ;;
    symlink)
      if [ ! -L "$rollback_original/runtime-state" ]; then
        printf 'FAIL: symlink rollback did not restore detached original parent\n' >&2
        exit 1
      fi
      assert_eq "$old_source" "$(readlink -- "$rollback_original/runtime-state")" \
        "symlink rollback restores detached original parent"
      ;;
    absent)
      if [ -e "$rollback_original/runtime-state" ] || [ -L "$rollback_original/runtime-state" ]; then
        printf 'FAIL: absent rollback left transaction result in detached original parent\n' >&2
        exit 1
      fi
      ;;
  esac
  wahrwelt_unregister_exact_snapshot "$rollback_snapshot"
}

assert_pinned_rollback_preserves_swapped_parent regular
assert_pinned_rollback_preserves_swapped_parent symlink
assert_pinned_rollback_preserves_swapped_parent absent

persist_profile
assert_eq end4-pc "$(tr -d '[:space:]' <"$persistent_state_file")" "active profile persistence"
assert_eq end4-pc "$(tr -d '[:space:]' <"$wahrwelt_end4_variant_state")" "variant persistence"

profile=noctalia
persist_profile
assert_eq noctalia "$(tr -d '[:space:]' <"$persistent_state_file")" "non-end4 active persistence"
assert_eq end4-pc "$(tr -d '[:space:]' <"$wahrwelt_end4_variant_state")" "non-end4 preserves remembered variant"

printf '%s\n' 'prior active state' 'with exact bytes' >"$persistent_state_file"
chmod 0600 "$persistent_state_file"
variant_prior="$test_root/prior-end4-variant"
printf '%s\n' end4 >"$variant_prior"
rm -f -- "$wahrwelt_end4_variant_state"
ln -s -- "$variant_prior" "$wahrwelt_end4_variant_state"
split_state_write_attempt=0
wahrwelt_after_runtime_publication_hook() {
  case "$1" in
    "$wahrwelt_end4_variant_state" | "$persistent_state_file") ;;
    *) return 0 ;;
  esac
  split_state_write_attempt=$((split_state_write_attempt + 1))
  if [ "$split_state_write_attempt" -eq 2 ]; then
    return 1
  fi
  return 0
}
profile=end4-pc
if persist_profile; then
  printf 'FAIL: split-state commit failure unexpectedly succeeded\n' >&2
  exit 1
fi
assert_eq $'prior active state\nwith exact bytes' "$(cat "$persistent_state_file")" "active state bytes roll back after second commit failure"
assert_eq 600 "$(stat -c %a "$persistent_state_file")" "active state mode rolls back after second commit failure"
if [ ! -L "$wahrwelt_end4_variant_state" ]; then
  printf 'FAIL: remembered End4 variant symlink was not restored\n' >&2
  exit 1
fi
assert_eq "$variant_prior" "$(readlink -- "$wahrwelt_end4_variant_state")" "remembered variant symlink target rolls back"
unset -f wahrwelt_after_runtime_publication_hook

printf '%s\n' outer-prior-active >"$persistent_state_file"
rm -f -- "$wahrwelt_end4_variant_state"
printf '%s\n' end4 >"$wahrwelt_end4_variant_state"
new_exact_test_snapshot .outer-state-rollback-
outer_state_snapshot="$wahrwelt_new_snapshot_dir"
snapshot_exact_paths "$outer_state_snapshot" "$persistent_state_file" "$wahrwelt_end4_variant_state"
nested_state_write_attempt=0
wahrwelt_after_runtime_publication_hook() {
  case "$1" in
    "$wahrwelt_end4_variant_state" | "$persistent_state_file") ;;
    *) return 0 ;;
  esac
  nested_state_write_attempt=$((nested_state_write_attempt + 1))
  if [ "$nested_state_write_attempt" -eq 2 ]; then
    return 1
  fi
  return 0
}
profile=end4-pc
if persist_profile; then
  printf 'FAIL: nested state persistence failure unexpectedly succeeded\n' >&2
  exit 1
fi
unset -f wahrwelt_after_runtime_publication_hook
if ! restore_exact_paths "$outer_state_snapshot" "$persistent_state_file" "$wahrwelt_end4_variant_state"; then
  printf 'FAIL: outer state snapshot treated inner rollback as a concurrent winner\n' >&2
  exit 1
fi
assert_eq outer-prior-active "$(tr -d '\n' <"$persistent_state_file")" \
  "outer rollback retains original active state after inner persistence rollback"
assert_eq end4 "$(tr -d '\n' <"$wahrwelt_end4_variant_state")" \
  "outer rollback retains original variant after inner persistence rollback"
wahrwelt_unregister_exact_snapshot "$outer_state_snapshot"

printf '%s\n' 'prior active transaction state' >"$persistent_state_file"
rm -f -- "$wahrwelt_end4_variant_state"
printf '%s\n' end4-pc >"$wahrwelt_end4_variant_state"
profile=end4
wahrwelt_after_runtime_publication_hook() {
  local path="$1"

  [ "$path" = "$persistent_state_file" ] || return 0
  mv -- "$path" "$test_root/transaction-owned-active-state"
  printf '%s\n' 'concurrent active-state winner' >"$path"
  chmod 0600 "$path"
}
if persist_profile; then
  printf 'FAIL: concurrent active-state winner unexpectedly committed\n' >&2
  exit 1
fi
unset -f wahrwelt_after_runtime_publication_hook
assert_eq 'concurrent active-state winner' "$(tr -d '\n' <"$persistent_state_file")" \
  "state rollback preserves winner swapped between publication and ownership record"
if ! find "$XDG_RUNTIME_DIR" -maxdepth 1 -type d -name '.state-rollback-*' -print -quit | grep -q .; then
  printf 'FAIL: state concurrent winner did not retain rollback recovery\n' >&2
  exit 1
fi

mkdir -p "$wahrwelt_hypr_dir"
hm_generation="$test_root/current-home-generation"
hm_end4="$hm_generation/home-files/.config/hypr/end4"
hm_end4_artifact="$test_root/hm-end4-artifact"
hm_gcroot="$HOME/.local/state/home-manager/gcroots/current-home"
mkdir -p "$hm_end4_artifact" "$(dirname -- "$hm_end4")" "$(dirname -- "$hm_gcroot")" "$wahrwelt_config_home/quickshell"
printf '%s\n' '-- exact HM End4 source' >"$hm_end4_artifact/hyprland.lua"
ln -s -- "$hm_end4_artifact" "$hm_end4"
ln -s -- "$hm_generation" "$hm_gcroot"

printf '%s\n' collision >"$wahrwelt_hypr_dir/end4"
profile=end4
if validate_end4_profile_tree; then
  printf 'FAIL: unknown end4 collision unexpectedly passed validation\n' >&2
  exit 1
fi
assert_eq collision "$(tr -d '[:space:]' <"$wahrwelt_hypr_dir/end4")" "end4 collision remains untouched"
mv -- "$wahrwelt_hypr_dir/end4" "$test_root/end4-file-collision"

mkdir -p "$wahrwelt_hypr_dir/end4"
for name in hyprland.lua hyprlock.conf hypridle.conf launcher.lua; do
  printf '%s\n' "unknown $name" >"$wahrwelt_hypr_dir/end4/$name"
done
if validate_end4_profile_tree; then
  printf 'FAIL: unknown end4 directory with expected files unexpectedly passed ownership validation\n' >&2
  exit 1
fi
assert_eq "unknown hyprland.lua" "$(tr -d '\n' <"$wahrwelt_hypr_dir/end4/hyprland.lua")" "unknown end4 directory remains untouched"
mv -- "$wahrwelt_hypr_dir/end4" "$test_root/end4-directory-collision"

ln -s -- "$test_root/missing-home-manager-source" "$wahrwelt_hypr_dir/end4"
if validate_end4_profile_tree; then
  printf 'FAIL: broken End4 target unexpectedly passed ownership validation\n' >&2
  exit 1
fi
assert_eq "$test_root/missing-home-manager-source" "$(readlink -- "$wahrwelt_hypr_dir/end4")" "broken End4 link remains untouched"
mv -- "$wahrwelt_hypr_dir/end4" "$test_root/end4-broken-link"

wrong_generation="$test_root/wrong-home-generation/.config/hypr/end4"
mkdir -p "$wrong_generation"
printf '%s\n' '-- wrong generation' >"$wrong_generation/hyprland.lua"
ln -s -- "$wrong_generation" "$wahrwelt_hypr_dir/end4"
if validate_end4_profile_tree; then
  printf 'FAIL: wrong-generation End4 link unexpectedly passed ownership validation\n' >&2
  exit 1
fi
assert_eq "$wrong_generation" "$(readlink -- "$wahrwelt_hypr_dir/end4")" "wrong-generation End4 link remains untouched"
mv -- "$wahrwelt_hypr_dir/end4" "$test_root/end4-wrong-generation-link"

untrusted_root="$test_root/untrusted-home-manager-files"
untrusted_end4="$untrusted_root/.config/hypr/end4"
untrusted_quickshell="$untrusted_root/.config/quickshell/ii"
mkdir -p "$untrusted_end4" "$untrusted_quickshell"
printf '%s\n' '-- self-controlled suffix source' >"$untrusted_end4/hyprland.lua"
ln -s -- "$untrusted_quickshell" "$wahrwelt_config_home/quickshell/ii"
ln -s -- "$untrusted_end4" "$wahrwelt_hypr_dir/end4"
if validate_end4_profile_tree; then
  printf 'FAIL: suffix-shaped QuickShell source unexpectedly proved End4 ownership\n' >&2
  exit 1
fi
assert_eq "$untrusted_end4" "$(readlink -- "$wahrwelt_hypr_dir/end4")" "untrusted suffix-shaped End4 link remains untouched"
mv -- "$wahrwelt_hypr_dir/end4" "$test_root/end4-untrusted-source-link"

ln -s -- "$hm_end4" "$wahrwelt_hypr_dir/end4"
validate_end4_profile_tree

fake_bin="$test_root/fake-bin"
lock_log="$test_root/lock.log"
mkdir -p "$fake_bin" "$(dirname -- "$wahrwelt_active_shell_state")"
printf '%s\n' \
  '#!/usr/bin/env bash' \
  'printf "%s\n" "$WAHRWELT_LOCK_TEST_PID"' \
  >"$fake_bin/pgrep"
printf '%s\n' \
  '#!/usr/bin/env bash' \
  'printf "hyprctl %s\n" "$*" >>"$WAHRWELT_LOCK_TEST_LOG"' \
  >"$fake_bin/hyprctl"
printf '%s\n' \
  '#!/usr/bin/env bash' \
  'printf "hyprlock %s\n" "$*" >>"$WAHRWELT_LOCK_TEST_LOG"' \
  >"$fake_bin/hyprlock"
chmod 0755 "$fake_bin/pgrep" "$fake_bin/hyprctl" "$fake_bin/hyprlock"

WAHRWELT_END4_PROFILE=end4 sleep 30 &
lock_test_pid=$!
for _ in $(seq 1 50); do
  if tr '\0' '\n' <"/proc/$lock_test_pid/environ" 2>/dev/null | grep -Fqx 'WAHRWELT_END4_PROFILE=end4'; then
    break
  fi
  sleep 0.01
done
printf '%s\n' end4 >"$wahrwelt_active_shell_state"
: >"$lock_log"
PATH="$fake_bin:$PATH" \
  WAHRWELT_LOCK_TEST_PID="$lock_test_pid" \
  WAHRWELT_LOCK_TEST_LOG="$lock_log" \
  "$scripts_dir/lock-active.sh"
assert_eq "hyprctl dispatch global quickshell:lock" "$(tr -d '\n' <"$lock_log")" "active exact End4 marker uses native lock"

: >"$lock_log"
PATH="$fake_bin:$PATH" \
  WAHRWELT_LOCK_TEST_PID="$lock_test_pid" \
  WAHRWELT_LOCK_TEST_LOG="$lock_log" \
  "$scripts_dir/lock-active.sh" end4-pc
assert_eq "hyprlock -c $wahrwelt_hypr_runtime_dir/hyprlock.conf" "$(tr -d '\n' <"$lock_log")" "mismatched End4 marker uses hyprlock"

kill "$lock_test_pid" 2>/dev/null || true
wait "$lock_test_pid" 2>/dev/null || true
lock_test_pid=""

start_lock_fixture="$test_root/start-shell-lock"
mkdir -p "$start_lock_fixture/runtime" "$start_lock_fixture/state"
cp -- "$scripts_dir/start-shell.sh" "$start_lock_fixture/start-shell.sh"
chmod 0755 "$start_lock_fixture/start-shell.sh"
printf '%s\n' \
  'wahrwelt_runtime_session_dir="$WAHRWELT_START_LOCK_FIXTURE/runtime"' \
  'wahrwelt_active_shell_state="$WAHRWELT_START_LOCK_FIXTURE/state/active-shell"' \
  'wahrwelt_log_file="$WAHRWELT_START_LOCK_FIXTURE/start-shell.log"' \
  'wahrwelt_hypr_runtime_dir="$WAHRWELT_START_LOCK_FIXTURE/state/hypr-runtime"' \
  'wahrwelt_end4_variant_state="$WAHRWELT_START_LOCK_FIXTURE/state/end4-variant"' \
  'wahrwelt_user_name=tester' \
  'wahrwelt_selector_pattern=selector' \
  'wahrwelt_caelestia_pattern=caelestia' \
  'wahrwelt_end4_env_pattern=end4' \
  'wahrwelt_default_shell_profile=end4' \
  'wahrwelt_valid_shell_profile() { [ "$1" = end4 ]; }' \
  'wahrwelt_open_end4_upgrade_state() { :; }' \
  'wahrwelt_merge_end4_upgrade_tokens() {' \
  '  printf "%s" "$1" >"$WAHRWELT_START_LOCK_FIXTURE/durable-tokens"' \
  '  printf "%s" "$1"' \
  '}' \
  'wahrwelt_read_end4_upgrade_tokens() {' \
  '  [ -f "$WAHRWELT_START_LOCK_FIXTURE/durable-tokens" ] || return 0' \
  '  cat "$WAHRWELT_START_LOCK_FIXTURE/durable-tokens"' \
  '}' \
  'wahrwelt_remove_end4_upgrade_tokens() {' \
  '  : >"$WAHRWELT_START_LOCK_FIXTURE/durable-tokens"' \
  '}' \
  'wahrwelt_begin_new_lock_directory() {' \
  '  [ -f "$WAHRWELT_START_LOCK_FIXTURE/lock-available" ] || return 1' \
  '  wahrwelt_new_lock_fd=99' \
  '  return 0' \
  '}' \
  'wahrwelt_write_new_pinned_regular_file() { :; }' \
  'wahrwelt_finish_new_lock_directory() {' \
  '  if [ -f "$WAHRWELT_START_LOCK_FIXTURE/finish-collision" ]; then' \
  '    wahrwelt_new_lock_publish_state=collision' \
  '    return 1' \
  '  fi' \
  '  wahrwelt_acquired_lock_identity=1:2' \
  '}' \
  'wahrwelt_close_new_lock_directory() { :; }' \
  'wahrwelt_fixture_known_calls=0' \
  'wahrwelt_lock_path_absent() {' \
  '  [ -f "$WAHRWELT_START_LOCK_FIXTURE/first-known-absent.consumed" ] ||' \
  '    [ -f "$WAHRWELT_START_LOCK_FIXTURE/second-known-absent.consumed" ] ||' \
  '    [ -f "$WAHRWELT_START_LOCK_FIXTURE/quarantine-absent.consumed" ]' \
  '}' \
  'wahrwelt_known_lock_directory() {' \
  '  wahrwelt_fixture_known_calls=$((wahrwelt_fixture_known_calls + 1))' \
  '  if [ -f "$WAHRWELT_START_LOCK_FIXTURE/first-known-absent" ] &&' \
  '    [ "$wahrwelt_fixture_known_calls" -eq 1 ]; then' \
  '    : >"$WAHRWELT_START_LOCK_FIXTURE/first-known-absent.consumed"' \
  '    return 1' \
  '  fi' \
  '  if [ -f "$WAHRWELT_START_LOCK_FIXTURE/second-known-absent" ] &&' \
  '    [ "$wahrwelt_fixture_known_calls" -eq 2 ]; then' \
  '    : >"$WAHRWELT_START_LOCK_FIXTURE/second-known-absent.consumed"' \
  '    return 1' \
  '  fi' \
  '  wahrwelt_known_lock_identity=1:2' \
  '  return 0' \
  '}' \
  'wahrwelt_read_known_lock_field() {' \
  '  case "$2" in' \
  '    owner) printf "%s\n" wahrwelt-start-shell ;;' \
  '    pid) printf "%s\n" 4242 ;;' \
  '    profile) printf "%s\n" end4 ;;' \
  '  esac' \
  '}' \
  'wahrwelt_pid_matches() {' \
  '  if [ -f "$WAHRWELT_START_LOCK_FIXTURE/second-known-absent" ] &&' \
  '    [ ! -f "$WAHRWELT_START_LOCK_FIXTURE/second-known-absent.consumed" ]; then return 1; fi' \
  '  if [ -f "$WAHRWELT_START_LOCK_FIXTURE/quarantine-absent" ] &&' \
  '    [ ! -f "$WAHRWELT_START_LOCK_FIXTURE/quarantine-absent.consumed" ]; then return 1; fi' \
  '  return 0' \
  '}' \
  'wahrwelt_quarantine_owned_lock() {' \
  '  if [ -f "$WAHRWELT_START_LOCK_FIXTURE/quarantine-absent" ]; then' \
  '    : >"$WAHRWELT_START_LOCK_FIXTURE/quarantine-absent.consumed"' \
  '  fi' \
  '  return 1' \
  '}' \
  'sleep() { :; }' \
  >"$start_lock_fixture/shell-runtime.sh"
printf '%s\n' 'prepare_runtime_environment() { :; }' \
  >"$start_lock_fixture/shell-runtime-env.sh"
printf '%s\n' \
  'runtime_bundle_paths() { :; }' \
  'wahrwelt_begin_exact_snapshot() { wahrwelt_new_snapshot_dir=snapshot; }' \
  'snapshot_exact_paths() { :; }' \
  'remove_exact_path_snapshot() { :; }' \
  'prepare_profile_or_fallback() { :; }' \
  >"$start_lock_fixture/shell-profile-sync.sh"
printf '%s\n' \
  'cleanup_legacy_end4_processes() {' \
  '  printf "%s" "$legacy_end4_upgrade_tokens" >"$WAHRWELT_START_LOCK_FIXTURE/cleanup-tokens"' \
  '  legacy_end4_upgrade_tokens="$(wahrwelt_remove_end4_upgrade_tokens "$legacy_end4_upgrade_tokens")"' \
  '  switch_transaction_active=0' \
  '  lock_identity=""' \
  '  exit 0' \
  '}' \
  >"$start_lock_fixture/shell-process.sh"

: >"$start_lock_fixture/start-shell.log"
if ! WAHRWELT_START_LOCK_FIXTURE="$start_lock_fixture" \
  "$start_lock_fixture/start-shell.sh" \
  --persist-end4-upgrade-processes 5099:99:end4-pC; then
  printf 'FAIL: persist-only End4 upgrade provenance mode touched shell lifecycle\n' >&2
  exit 1
fi
if [ "$(cat "$start_lock_fixture/durable-tokens" 2>/dev/null || true)" != '5099:99:end4-pC' ]; then
  printf 'FAIL: persist-only End4 upgrade provenance was not durable before lifecycle\n' >&2
  exit 1
fi
: >"$start_lock_fixture/durable-tokens"

if ! WAHRWELT_START_LOCK_FIXTURE="$start_lock_fixture" \
  "$start_lock_fixture/start-shell.sh" end4; then
  printf 'FAIL: ordinary same-profile start-shell invocation did not reuse the active owner\n' >&2
  exit 1
fi
if ! grep -Fq 'another start-shell instance is already running for profile=end4 pid=4242' \
  "$start_lock_fixture/start-shell.log"; then
  printf 'FAIL: ordinary same-profile start-shell reuse was not recorded\n' >&2
  exit 1
fi

: >"$start_lock_fixture/lock-available"
: >"$start_lock_fixture/finish-collision"
: >"$start_lock_fixture/start-shell.log"
if ! WAHRWELT_START_LOCK_FIXTURE="$start_lock_fixture" \
  "$start_lock_fixture/start-shell.sh" end4; then
  printf 'FAIL: staged start-shell publisher loser did not reuse the exact same-profile winner\n' >&2
  exit 1
fi
if ! grep -Fq 'another start-shell instance is already running for profile=end4 pid=4242' \
  "$start_lock_fixture/start-shell.log"; then
  printf 'FAIL: staged start-shell publisher loser skipped exact winner classification\n' >&2
  exit 1
fi
mv -- "$start_lock_fixture/lock-available" "$start_lock_fixture/lock-available.used"
mv -- "$start_lock_fixture/finish-collision" "$start_lock_fixture/finish-collision.used"

for absent_boundary in first-known-absent second-known-absent quarantine-absent; do
  : >"$start_lock_fixture/$absent_boundary"
  : >"$start_lock_fixture/start-shell.log"
  if ! WAHRWELT_START_LOCK_FIXTURE="$start_lock_fixture" \
    "$start_lock_fixture/start-shell.sh" end4; then
    printf 'FAIL: start-shell treated transient %s as an ownership collision\n' "$absent_boundary" >&2
    exit 1
  fi
  if [ ! -f "$start_lock_fixture/$absent_boundary.consumed" ] ||
    ! grep -Fq 'another start-shell instance is already running for profile=end4 pid=4242' \
      "$start_lock_fixture/start-shell.log"; then
    printf 'FAIL: start-shell did not retry exact %s boundary\n' "$absent_boundary" >&2
    exit 1
  fi
  mv -- "$start_lock_fixture/$absent_boundary" \
    "$start_lock_fixture/$absent_boundary.used"
  mv -- "$start_lock_fixture/$absent_boundary.consumed" \
    "$start_lock_fixture/$absent_boundary.consumed.used"
done

: >"$start_lock_fixture/start-shell.log"
if WAHRWELT_START_LOCK_FIXTURE="$start_lock_fixture" \
  "$start_lock_fixture/start-shell.sh" \
  --legacy-direct-end4-upgrade-processes 5101:101:ii end4; then
  printf 'FAIL: token-bearing same-profile start-shell invocation silently reused another owner\n' >&2
  exit 1
fi
if ! grep -Fq 'waiting for start-shell upgrade lock; requested=end4 active=end4 pid=4242' \
  "$start_lock_fixture/start-shell.log"; then
  printf 'FAIL: token-bearing same-profile start-shell invocation did not wait for ownership\n' >&2
  exit 1
fi
if ! grep -Fq 'failed to acquire start-shell lock; profile=end4' \
  "$start_lock_fixture/start-shell.log"; then
  printf 'FAIL: token-bearing same-profile start-shell invocation did not fail closed\n' >&2
  exit 1
fi
if [ "$(cat "$start_lock_fixture/durable-tokens" 2>/dev/null || true)" != '5101:101:ii' ]; then
  printf 'FAIL: failed token-bearing start-shell run did not retain durable provenance\n' >&2
  exit 1
fi

: >"$start_lock_fixture/lock-available"
: >"$start_lock_fixture/start-shell.log"
if ! WAHRWELT_START_LOCK_FIXTURE="$start_lock_fixture" \
  WAYLAND_DISPLAY=wayland-1 HYPRLAND_INSTANCE_SIGNATURE=test \
  "$start_lock_fixture/start-shell.sh" end4; then
  printf 'FAIL: argumentless retry did not resume durable End4 upgrade cleanup\n' >&2
  exit 1
fi
if [ "$(cat "$start_lock_fixture/cleanup-tokens" 2>/dev/null || true)" != '5101:101:ii' ]; then
  printf 'FAIL: argumentless retry did not load exact durable End4 provenance\n' >&2
  exit 1
fi
if [ -s "$start_lock_fixture/durable-tokens" ]; then
  printf 'FAIL: successful argumentless retry did not consume durable provenance\n' >&2
  exit 1
fi

printf 'OK end4 runtime variants\n'
