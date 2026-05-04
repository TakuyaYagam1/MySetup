#!/usr/bin/env bash
set -euo pipefail

active="$(hyprctl activewindow -j 2>/dev/null || true)"
address="$(jq -r '.address // empty' <<<"$active")"
class="$(jq -r '.class // empty' <<<"$active")"
workspace="$(jq -r '.workspace.name // empty' <<<"$active")"

if [[ -z "$address" || "$address" == "0x0" ]]; then
  exit 0
fi

case "${class,,}" in
  spotify)
    if [[ "$workspace" == "special:music" ]]; then
      hyprctl dispatch togglespecialworkspace music >/dev/null 2>&1 || true
    else
      hyprctl dispatch movetoworkspacesilent "special:music,address:$address" >/dev/null 2>&1 || true
    fi
    ;;
  *)
    hyprctl dispatch closewindow "address:$address" >/dev/null 2>&1 \
      || hyprctl dispatch killactive >/dev/null 2>&1 \
      || true
    ;;
esac
