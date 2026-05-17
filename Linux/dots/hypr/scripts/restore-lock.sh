#!/usr/bin/env bash
set -euo pipefail

script_dir="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=Linux/dots/hypr/scripts/shell-runtime.sh
. "$script_dir/shell-runtime.sh"

profile="${1:-}"

if ! mysetup_valid_shell_profile "$profile"; then
  printf 'unknown shell profile: %s\n' "${profile:-empty}" >&2
  exit 1
fi

"$script_dir/start-shell.sh" "$profile"

wait_for_profile() {
  local _

  for _ in $(seq 1 50); do
    case "$profile" in
      caelestia)
        pgrep -u "$mysetup_user_name" -f "$mysetup_caelestia_pattern" >/dev/null 2>&1 && return 0
        ;;
      noctalia)
        pgrep -u "$mysetup_user_name" -f "$mysetup_noctalia_pattern" >/dev/null 2>&1 && return 0
        ;;
      end4)
        pgrep -u "$mysetup_user_name" -f "$mysetup_end4_pattern" >/dev/null 2>&1 && return 0
        ;;
    esac
    sleep 0.1
  done

  return 1
}

wait_for_profile || true

case "$profile" in
  caelestia)
    hyprctl dispatch 'hl.dsp.global("caelestia:lock")' >/dev/null
    ;;
  noctalia)
    noctalia-shell ipc call lockScreen lock
    ;;
  end4)
    hyprlock -c "$mysetup_hypr_runtime_dir/hyprlock.conf"
    ;;
esac
