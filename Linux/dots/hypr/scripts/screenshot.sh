#!/usr/bin/env bash
set -euo pipefail

mode="${1:-full}"
dir="${XDG_PICTURES_DIR:-$HOME/Pictures}/Screenshots"
mkdir -p "$dir"

file="$dir/screenshot-$(date +%Y%m%d-%H%M%S).png"

copy_to_clipboard() {
  wl-copy --type image/png <"$file"
}

notify_saved() {
  if command -v notify-send >/dev/null 2>&1; then
    notify-send -i "$file" "Screenshot saved" "$file"
  fi
}

case "$mode" in
  full)
    grim "$file"
    copy_to_clipboard
    notify_saved
    ;;
  region)
    geometry="$(slurp)"
    grim -g "$geometry" "$file"
    copy_to_clipboard
    notify_saved
    ;;
  edit)
    geometry="$(slurp)"
    grim -g "$geometry" "$file"
    copy_to_clipboard
    swappy -f "$file"
    ;;
  *)
    echo "Usage: $0 [full|region|edit]" >&2
    exit 2
    ;;
esac
