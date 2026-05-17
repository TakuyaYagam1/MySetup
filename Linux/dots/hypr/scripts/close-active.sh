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
      hyprctl dispatch 'hl.dsp.workspace.toggle_special("music")' >/dev/null 2>&1 || true
    else
      hyprctl dispatch "hl.dsp.window.move({ workspace = \"special:music\", window = \"address:$address\" })" >/dev/null 2>&1 || true
    fi
    ;;
  *)
    hyprctl dispatch "hl.dsp.window.close(\"address:$address\")" >/dev/null 2>&1 ||
      hyprctl dispatch 'hl.dsp.window.kill()' >/dev/null 2>&1 ||
      true
    ;;
esac
