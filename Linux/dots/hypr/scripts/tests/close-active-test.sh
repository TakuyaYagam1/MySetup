#!/usr/bin/env bash
set -euo pipefail

scripts_dir="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
close_active="$scripts_dir/close-active.sh"
test_root="$(mktemp -d)"
trap 'rm -rf -- "$test_root"' EXIT

fake_bin="$test_root/bin"
active_json="$test_root/active.json"
dispatch_log="$test_root/dispatch.log"
mkdir -p "$fake_bin"

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

assert_log() {
  local expected="$1"
  local message="$2"
  local actual=""

  if [ -f "$dispatch_log" ]; then
    actual="$(cat "$dispatch_log")"
  fi
  if [ "$actual" != "$expected" ]; then
    printf 'FAIL: %s\nexpected:\n%s\nactual:\n%s\n' "$message" "$expected" "$actual" >&2
    exit 1
  fi
}

cat >"$fake_bin/hyprctl" <<'EOF'
#!/usr/bin/env bash
set -u

if [ "${1:-}" = "activewindow" ] && [ "${2:-}" = "-j" ]; then
  case "${WAHRWELT_CLOSE_ACTIVE_MODE:-json}" in
    missing) exit 1 ;;
    empty) exit 0 ;;
    json) cat "$WAHRWELT_CLOSE_ACTIVE_JSON" ;;
  esac
  exit 0
fi

if [ "${1:-}" = "dispatch" ]; then
  shift
  printf '%s\n' "$*" >>"$WAHRWELT_CLOSE_ACTIVE_LOG"
  if [ "${WAHRWELT_CLOSE_ACTIVE_FAIL_CLOSE:-0}" = 1 ] &&
    [[ "${1:-}" == 'hl.dsp.window.close('* ]]; then
    exit 1
  fi
  exit 0
fi

exit 64
EOF
chmod 0755 "$fake_bin/hyprctl"

run_case() {
  local mode="$1"
  local json="$2"
  local fail_close="${3:-0}"

  : >"$dispatch_log"
  printf '%s' "$json" >"$active_json"
  PATH="$fake_bin:$PATH" \
    WAHRWELT_CLOSE_ACTIVE_MODE="$mode" \
    WAHRWELT_CLOSE_ACTIVE_JSON="$active_json" \
    WAHRWELT_CLOSE_ACTIVE_LOG="$dispatch_log" \
    WAHRWELT_CLOSE_ACTIVE_FAIL_CLOSE="$fail_close" \
    "$close_active"
}

[ -x "$close_active" ] || fail "close-active.sh is not executable"

run_case missing ""
assert_log "" "missing active window is a safe no-op"

run_case empty ""
assert_log "" "empty active window JSON is a safe no-op"

run_case json '{malformed'
assert_log "" "malformed active window JSON is a safe no-op"

run_case json '{}'
assert_log "" "missing active window fields are a safe no-op"

for invalid_address in 0x0 0x00 invalid '0x12;reload'; do
  run_case json "{\"address\":\"$invalid_address\",\"class\":\"foot\",\"workspace\":{\"name\":\"1\"}}"
  assert_log "" "invalid address $invalid_address does not dispatch"
done

run_case json '{"address":"0xAbC123","class":"sPoTiFy","workspace":{"name":"special:music"}}'
assert_log 'hl.dsp.workspace.toggle_special("music")' "Spotify special workspace toggles closed"

run_case json '{"address":"0xabc123","class":"SPOTIFY","workspace":{"name":"2"}}'
assert_log 'hl.dsp.window.move({ workspace = "special:music", window = "address:0xabc123" })' \
  "Spotify routing is case-insensitive and addressed"

run_case json '{"address":"0x42","class":"foot","workspace":{"name":"1"}}'
assert_log 'hl.dsp.window.close({ window = "address:0x42" })' "normal windows close by address"

run_case json '{"address":"0x42","class":"foot","workspace":{"name":"1"}}' 1
assert_log $'hl.dsp.window.close({ window = "address:0x42" })\nhl.dsp.window.kill()' \
  "close failure falls back to active window kill"

printf 'OK close-active behavior\n'
