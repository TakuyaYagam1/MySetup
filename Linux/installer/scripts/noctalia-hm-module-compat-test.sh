#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "$script_dir/../../.." && pwd)"

home_manager_without_builtin_ref="github:nix-community/home-manager/ef01547e1a4f01ea29ba813fa92c1a2cd3fad16e"
home_manager_with_builtin_ref="github:nix-community/home-manager/7690127e7e1c90ab1dcaff525d0dafbeb7e14182"

WAHRWELT_NOCTALIA_TEST_REPO_ROOT="$repo_root" \
  WAHRWELT_NOCTALIA_TEST_HM_WITHOUT_BUILTIN="$home_manager_without_builtin_ref" \
  WAHRWELT_NOCTALIA_TEST_HM_WITH_BUILTIN="$home_manager_with_builtin_ref" \
  nix eval --impure --json --file "$script_dir/noctalia-hm-module-compat.nix"

printf 'OK Noctalia Home Manager mixed-version module ownership\n'
