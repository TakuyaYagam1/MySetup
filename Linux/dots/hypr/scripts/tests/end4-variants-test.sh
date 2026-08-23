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

# shellcheck source=Linux/dots/hypr/scripts/shell-runtime.sh
. "$scripts_dir/shell-runtime.sh"

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

wahrwelt_valid_shell_profile end4-pc
assert_eq end4 "$(wahrwelt_shell_family end4)" "Official family"
assert_eq end4 "$(wahrwelt_shell_family end4-pc)" "pC family"
assert_eq ii "$(wahrwelt_end4_quickshell_config end4)" "Official config"
assert_eq end4-pC "$(wahrwelt_end4_quickshell_config end4-pc)" "pC config"
assert_eq "$XDG_CONFIG_HOME/quickshell/ii" "$(wahrwelt_end4_quickshell_path end4)" "Official runtime path"
assert_eq "$XDG_CONFIG_HOME/quickshell/end4-pC" "$(wahrwelt_end4_quickshell_path end4-pc)" "pC runtime path"
assert_eq end4 "$(wahrwelt_read_end4_variant)" "missing state fallback"
assert_matches "$wahrwelt_end4_official_pattern" "qs-end4 -n -d -c ii" "Official process pattern"
assert_matches "$wahrwelt_end4_pc_pattern" "qs-end4 -n -d -c end4-pC" "pC process pattern"
assert_not_matches "$wahrwelt_end4_official_pattern" "qs-end4 -n -d -c end4-pC" "Official pattern excludes pC"
assert_not_matches "$wahrwelt_end4_pc_pattern" "qs-end4 -n -d -c ii" "pC pattern excludes Official"

wahrwelt_export_end4_quickshell_config end4
assert_eq "$XDG_CONFIG_HOME/quickshell/ii" "$WAHRWELT_QS_CONFIG" "Official wrapper override"
assert_eq "$XDG_CONFIG_HOME/quickshell/ii" "$qsConfig" "Official upstream runtime path"
wahrwelt_export_end4_quickshell_config end4-pc
assert_eq "$XDG_CONFIG_HOME/quickshell/end4-pC" "$WAHRWELT_QS_CONFIG" "pC wrapper override"
assert_eq "$XDG_CONFIG_HOME/quickshell/end4-pC" "$qsConfig" "pC upstream runtime path"

quickshell_module="$scripts_dir/../../../NixOS/home/end4/quickshell.nix"
if ! grep -Fq 'WAHRWELT_QS_CONFIG' "$quickshell_module"; then
  printf 'FAIL: qs-end4 wrapper does not honor the variant runtime path\n' >&2
  exit 1
fi

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

config_home="$wahrwelt_config_home"
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

persist_profile
assert_eq end4-pc "$(tr -d '[:space:]' <"$persistent_state_file")" "active profile persistence"
assert_eq end4-pc "$(tr -d '[:space:]' <"$wahrwelt_end4_variant_state")" "variant persistence"

profile=noctalia
persist_profile
assert_eq noctalia "$(tr -d '[:space:]' <"$persistent_state_file")" "non-end4 active persistence"
assert_eq end4-pc "$(tr -d '[:space:]' <"$wahrwelt_end4_variant_state")" "non-end4 preserves remembered variant"

printf 'OK end4 runtime variants\n'
