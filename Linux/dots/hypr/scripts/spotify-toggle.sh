#!/usr/bin/env bash
set -euo pipefail

spotify="$(hyprctl clients -j | jq -r '
  [.[] | select((.class // "" | ascii_downcase) == "spotify")]
  | sort_by(.focusHistoryID)
  | .[0] // empty
  | @base64
')"

if [[ -z "$spotify" ]]; then
  exec app2unit -- spotify
fi

client="$(base64 -d <<<"$spotify")"
address="$(jq -r '.address' <<<"$client")"
workspace="$(jq -r '.workspace.name' <<<"$client")"

if [[ "$workspace" == special:* ]]; then
  special_name="${workspace#special:}"
  visible_special="$(hyprctl monitors -j | jq -r --arg name "$workspace" '
    [.[] | select(.specialWorkspace.name == $name)] | length
  ')"

  if [[ "$visible_special" == "0" ]]; then
    hyprctl dispatch "hl.dsp.workspace.toggle_special(\"$special_name\")" >/dev/null
  fi

  hyprctl dispatch "hl.dsp.focus({ window = \"address:$address\" })" >/dev/null 2>&1 || true
  exit 0
fi

hyprctl dispatch "hl.dsp.focus({ workspace = \"$workspace\" })" >/dev/null 2>&1 || true
hyprctl dispatch "hl.dsp.focus({ window = \"address:$address\" })" >/dev/null 2>&1 || true
