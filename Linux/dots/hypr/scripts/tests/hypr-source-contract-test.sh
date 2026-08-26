#!/usr/bin/env bash
set -euo pipefail

repo_root="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../../../../.." && pwd)"
canonical_rules="$repo_root/Linux/dots/hypr/hyprland/rules.lua"
common_rules="$repo_root/Linux/dots/hypr/shell-common-rules.lua"
home_shells="$repo_root/Linux/NixOS/home/shells/default.nix"
installer_sync="$repo_root/Linux/installer/internal/dots/hypr.go"
end4_patch="$repo_root/Linux/NixOS/home/end4/patches/hypr.nix"

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

grep -Fqx 'require("shell-common-rules")' "$canonical_rules" ||
  fail "canonical rules do not load shell-common-rules"
grep -Fq '"hypr/shell-common-rules.lua"' "$home_shells" ||
  fail "Home Manager does not own top-level shell-common-rules.lua"
grep -Fq 'source = dotsRoot + "/hypr/shell-common-rules.lua";' "$home_shells" ||
  fail "Home Manager shared-rules source drifted"
grep -Fq '"shell-common-rules.lua",' "$installer_sync" ||
  fail "installer source validation does not include shell-common-rules.lua"

for workspace in sysmon music communication todo; do
  count="$(grep -Fc -- "workspace = \"special:$workspace\"" "$common_rules")"
  [ "$count" -ge 1 ] || fail "special:$workspace has no shared rule"
done

if grep -Fq 'dofile(config_home .. "/hypr/shell-common-rules.lua")' "$end4_patch"; then
  fail "End4 patch loads shared rules directly"
fi

printf 'OK Hypr shared source contracts\n'
