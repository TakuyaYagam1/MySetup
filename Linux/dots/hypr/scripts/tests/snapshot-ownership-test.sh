#!/usr/bin/env bash
set -euo pipefail

scripts_dir="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
installer_dir="$(CDPATH='' cd -- "$scripts_dir/../../../installer" && pwd)"
test_root="$(mktemp -d)"
trap 'rm -rf -- "$test_root"' EXIT

helper="$test_root/wahrwelt-fs-helper"
session="$test_root/session"
runtime="$test_root/runtime"
mkdir -p "$session" "$runtime"
chmod 0700 "$session" "$runtime"
(cd "$installer_dir" && go build -o "$helper" ./cmd/wahrwelt-fs-helper)

begin() {
  "$helper" runtime-begin --root "$session" --kind runtime "$@"
}

write() {
  local transaction="$1"
  local target="$2"
  local payload="$3"

  printf '%s\n' "$payload" |
    "$helper" runtime-write --transaction "$transaction" --target "$target" --mode 0644
}

target="$runtime/shell-keybinds.lua"
printf '%s\n' old >"$target"
transaction="$(begin "$target")"
write "$transaction" "$target" committed
"$helper" runtime-commit "$transaction"
[ "$(tr -d '\n' <"$target")" = committed ] || {
  printf 'FAIL: committed payload missing\n' >&2
  exit 1
}
if find "$session" "$runtime" -maxdepth 1 \
  \( -name '.runtime-rollback-*' -o -name '.wahrwelt-runtime-stage-*' \) -print -quit |
  grep -q .; then
  printf 'FAIL: successful commit retained transaction residue\n' >&2
  exit 1
fi

printf '%s\n' exact-original >"$target"
before="$(stat -c '%d:%i' "$target")"
transaction="$(begin "$target")"
write "$transaction" "$target" replacement
"$helper" runtime-rollback "$transaction"
after="$(stat -c '%d:%i' "$target")"
[ "$after" = "$before" ] && [ "$(tr -d '\n' <"$target")" = exact-original ] || {
  printf 'FAIL: rollback did not restore exact original inode\n' >&2
  exit 1
}
"$helper" runtime-commit "$transaction"

printf '%s\n' collision-original >"$target"
transaction="$(begin "$target")"
write "$transaction" "$target" managed
printf '%s\n' concurrent >"$target.concurrent"
mv -T -- "$target.concurrent" "$target"
if "$helper" runtime-rollback "$transaction" 2>/dev/null; then
  printf 'FAIL: rollback replaced a concurrent winner\n' >&2
  exit 1
fi
[ "$(tr -d '\n' <"$target")" = concurrent ] || {
  printf 'FAIL: concurrent winner changed\n' >&2
  exit 1
}
[ -d "$transaction" ] || {
  printf 'FAIL: collision did not retain exact recovery journal\n' >&2
  exit 1
}

unknown="$session/.runtime-rollback-v1-unknown"
mkdir "$unknown"
printf '%s\n' preserve >"$unknown/foreign"
if "$helper" runtime-scavenge --root "$session" --kind runtime 2>/dev/null; then
  printf 'FAIL: scavenger accepted an unknown matching directory\n' >&2
  exit 1
fi
[ "$(tr -d '\n' <"$unknown/foreign")" = preserve ] || {
  printf 'FAIL: scavenger changed an unknown directory\n' >&2
  exit 1
}

rm -rf -- "$unknown"
session="$test_root/stale-session"
mkdir "$session"
chmod 0700 "$session"
printf '%s\n' stale-original >"$target"
transaction="$(begin "$target")"
write "$transaction" "$target" stale-managed
"$helper" runtime-scavenge --root "$session" --kind runtime
[ "$(tr -d '\n' <"$target")" = stale-original ] || {
  printf 'FAIL: scavenger did not roll back an owned active journal\n' >&2
  exit 1
}
[ ! -e "$transaction" ] || {
  printf 'FAIL: scavenger retained a completed owned journal\n' >&2
  exit 1
}

printf 'OK fd transaction ownership\n'
