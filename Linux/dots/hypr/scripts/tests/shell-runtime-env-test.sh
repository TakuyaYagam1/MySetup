#!/usr/bin/env bash
set -euo pipefail

tests_dir="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
scripts_dir="$(dirname -- "$tests_dir")"
test_root="$(mktemp -d)"
trap 'rm -rf -- "$test_root"' EXIT

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

write_fake() {
  local name="$1"
  shift
  printf '%s\n' "$@" >"$test_root/bin/$name"
  chmod 0755 "$test_root/bin/$name"
}

mkdir -p "$test_root/bin" "$test_root/home"
export WAHRWELT_ENV_TEST_LOG="$test_root/commands.log"
: >"$WAHRWELT_ENV_TEST_LOG"
printf -v log_path_quoted '%q' "$WAHRWELT_ENV_TEST_LOG"

# shellcheck disable=SC2016
write_fake dbus-update-activation-environment \
  '#!/usr/bin/env bash' \
  "log_path=$log_path_quoted" \
  '{ printf "dbus"; printf "\\t%s" "$@"; printf "\\n"; } >>"$log_path"' \
  'exit 0'

# shellcheck disable=SC2016
write_fake systemctl \
  '#!/usr/bin/env bash' \
  "log_path=$log_path_quoted" \
  '{ printf "systemctl"; printf "\\t%s" "$@"; printf "\\n"; } >>"$log_path"' \
  'if [ "$*" = "--user show-environment" ]; then' \
  '  printf "%s\\n" "ILLOGICAL_IMPULSE_MANAGER_ONLY=stale" "SAFE_MANAGER_VALUE=keep"' \
  'fi' \
  'exit 0'

for command_name in hyprctl pkill; do
  # shellcheck disable=SC2016
  write_fake "$command_name" \
    '#!/usr/bin/env bash' \
    "log_path=$log_path_quoted" \
    '{ printf "helper"; printf "\\t%s" "$@"; printf "\\n"; } >>"$log_path"' \
    'exit 0'
done

export HOME="$test_root/home"
export USER=tester
export user_name=tester
export PATH="$test_root/bin:$PATH"
export XDG_CURRENT_DESKTOP=Hyprland
export XDG_SESSION_DESKTOP=Hyprland
export HYPRLAND_INSTANCE_SIGNATURE=test-instance
export WAHRWELT_END4_PROFILE=end4
export WAHRWELT_QS_CONFIG=/stale/end4
export qsConfig=/stale/ii
export ILLOGICAL_IMPULSE_DOTFILES_SOURCE=/stale/config
export ILLOGICAL_IMPULSE_VIRTUAL_ENV=/stale/venv
export ILLOGICAL_IMPULSE_CURRENT_ONLY=stale

for command_name in dbus-update-activation-environment systemctl hyprctl pkill; do
  if [ "$(command -v "$command_name")" != "$test_root/bin/$command_name" ]; then
    fail "fixture command did not override live executable: $command_name"
  fi
done
dbus-update-activation-environment FIXTURE_PROBE=
if ! grep -Fq $'dbus\tFIXTURE_PROBE=' "$WAHRWELT_ENV_TEST_LOG"; then
  fail "fixture D-Bus logger is not executable"
fi
: >"$WAHRWELT_ENV_TEST_LOG"

# shellcheck source=Linux/dots/hypr/scripts/shell-runtime-env.sh
. "$scripts_dir/shell-runtime-env.sh"

prepare_runtime_environment

for name in \
  WAHRWELT_END4_PROFILE \
  WAHRWELT_QS_CONFIG \
  qsConfig \
  ILLOGICAL_IMPULSE_DOTFILES_SOURCE \
  ILLOGICAL_IMPULSE_VIRTUAL_ENV; do
  if [[ -v "$name" ]]; then
    fail "legacy End4 variable survived in launcher environment: $name"
  fi
done
[ "${ILLOGICAL_IMPULSE_CURRENT_ONLY:-}" = stale ] ||
  fail "unrelated ILLOGICAL_IMPULSE variable was removed from launcher environment"

if env | grep -Eq '^WAHRWELT_END4_PROFILE='; then
  fail "Caelestia child would inherit the exact End4 process marker"
fi

# prepare_runtime_environment intentionally rebuilds PATH. Put fixture tools
# first again so propagation is fully isolated from the live user session.
export PATH="$test_root/bin:$PATH"
propagate_runtime_environment

if grep -Fq $'\t--all' "$WAHRWELT_ENV_TEST_LOG"; then
  fail "runtime environment propagation still uploads the entire environment"
fi

dbus_clear="$(awk -F '\t' '$1 == "dbus" && $2 != "--systemd" { print; exit }' "$WAHRWELT_ENV_TEST_LOG")"
dbus_import="$(awk -F '\t' '$1 == "dbus" && $2 == "--systemd" { print; exit }' "$WAHRWELT_ENV_TEST_LOG")"
systemd_unset="$(awk -F '\t' '$1 == "systemctl" && $2 == "--user" && $3 == "unset-environment" { print; exit }' "$WAHRWELT_ENV_TEST_LOG")"
systemd_import="$(awk -F '\t' '$1 == "systemctl" && $2 == "--user" && $3 == "import-environment" { print; exit }' "$WAHRWELT_ENV_TEST_LOG")"

[ -n "$dbus_clear" ] || fail "D-Bus legacy environment clear was not requested"
[ -n "$dbus_import" ] || fail "D-Bus safe environment import was not requested"
[ -n "$systemd_unset" ] || fail "systemd legacy environment unset was not requested"
[ -n "$systemd_import" ] || fail "systemd safe environment import was not requested"

for name in \
  WAHRWELT_END4_PROFILE \
  WAHRWELT_QS_CONFIG \
  qsConfig \
  ILLOGICAL_IMPULSE_DOTFILES_SOURCE \
  ILLOGICAL_IMPULSE_VIRTUAL_ENV; do
  case "$dbus_clear" in
    *$'\t'"$name="*) ;;
    *) fail "D-Bus clear omitted legacy variable: $name" ;;
  esac
  case "$systemd_unset" in
    *$'\t'"$name"*) ;;
    *) fail "systemd unset omitted legacy variable: $name" ;;
  esac
  case "$dbus_import" in
    *$'\t'"$name"*) fail "D-Bus safe import leaked legacy variable: $name" ;;
  esac
  case "$systemd_import" in
    *$'\t'"$name"*) fail "systemd safe import leaked legacy variable: $name" ;;
  esac
done

for name in ILLOGICAL_IMPULSE_CURRENT_ONLY ILLOGICAL_IMPULSE_MANAGER_ONLY; do
  case "$dbus_clear" in
    *$'\t'"$name="*) fail "D-Bus cleanup broadly removed unrelated variable: $name" ;;
  esac
  case "$systemd_unset" in
    *$'\t'"$name"*) fail "systemd cleanup broadly removed unrelated variable: $name" ;;
  esac
done
if grep -Fq $'systemctl\t--user\tshow-environment' "$WAHRWELT_ENV_TEST_LOG"; then
  fail "legacy cleanup enumerated the complete user manager environment"
fi

for name in XDG_DATA_DIRS PATH XDG_CURRENT_DESKTOP XDG_SESSION_DESKTOP; do
  case "$dbus_import" in
    *$'\t'"$name"*) ;;
    *) fail "D-Bus safe import omitted allowlisted variable: $name" ;;
  esac
  case "$systemd_import" in
    *$'\t'"$name"*) ;;
    *) fail "systemd safe import omitted allowlisted variable: $name" ;;
  esac
done

printf 'OK runtime environment allowlist and legacy End4 cleanup\n'
