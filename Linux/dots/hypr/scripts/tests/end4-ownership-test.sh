#!/usr/bin/env bash
set -euo pipefail

repo_root="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../../../../.." && pwd)"
guard="$repo_root/Linux/NixOS/home/end4/patches/end4-ownership-guard.sh"
test_root="$(mktemp -d)"
trap 'rm -rf -- "$test_root"' EXIT

target="$test_root/home/.config/hypr/end4"
current_source="$test_root/current-home/home-files/.config/hypr/end4"
artifact="$test_root/end4-artifact"
mkdir -p "$(dirname -- "$target")" "$(dirname -- "$current_source")" "$artifact"
printf '%s\n' '-- managed End4 artifact' >"$artifact/hyprland.lua"
ln -s -- "$artifact" "$current_source"

assert_rejected_unchanged() {
  local label="$1"
  local before="$2"

  if bash "$guard" "$target" "$current_source" >/dev/null 2>&1; then
    printf 'FAIL: %s unexpectedly passed End4 ownership guard\n' "$label" >&2
    exit 1
  fi
  if [ -L "$target" ]; then
    [ "$(readlink -- "$target")" = "$before" ] || {
      printf 'FAIL: %s symlink changed during rejection\n' "$label" >&2
      exit 1
    }
  elif [ -f "$target" ]; then
    [ "$(cat "$target")" = "$before" ] || {
      printf 'FAIL: %s file changed during rejection\n' "$label" >&2
      exit 1
    }
  elif [ -d "$target" ]; then
    [ -f "$target/sentinel" ] && [ "$(cat "$target/sentinel")" = "$before" ] || {
      printf 'FAIL: %s directory changed during rejection\n' "$label" >&2
      exit 1
    }
  else
    printf 'FAIL: %s disappeared during rejection\n' "$label" >&2
    exit 1
  fi
}

bash "$guard" "$target" "$current_source"

ln -s -- "$current_source" "$target"
bash "$guard" "$target" "$current_source"
rm -- "$target"

printf '%s' 'foreign file' >"$target"
assert_rejected_unchanged 'foreign regular file' 'foreign file'
rm -- "$target"

mkdir "$target"
printf '%s' 'foreign directory' >"$target/sentinel"
assert_rejected_unchanged 'foreign directory' 'foreign directory'
rm -r -- "$target"

foreign="$test_root/foreign-end4"
mkdir "$foreign"
printf '%s\n' '-- foreign End4 tree' >"$foreign/hyprland.lua"
ln -s -- "$foreign" "$target"
assert_rejected_unchanged 'foreign symlink' "$foreign"
rm -- "$target"

broken="$test_root/missing-end4"
ln -s -- "$broken" "$target"
assert_rejected_unchanged 'broken symlink' "$broken"

printf 'OK End4 Home Manager ownership guard\n'
