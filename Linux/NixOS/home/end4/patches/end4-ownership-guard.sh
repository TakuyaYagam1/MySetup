#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -ne 2 ]; then
  printf 'usage: %s TARGET CURRENT_HOME_MANAGER_SOURCE\n' "$0" >&2
  exit 2
fi

target="$1"
current_source="$2"

ownership_collision() {
  printf 'Refusing unowned End4 profile collision: %s\n' "$target" >&2
  exit 1
}

if [ ! -L "$target" ]; then
  [ ! -e "$target" ] || ownership_collision
  exit 0
fi

target_resolved="$(readlink -f -- "$target" 2>/dev/null || true)"
if [ -z "$target_resolved" ] || [ ! -d "$target_resolved" ] || [ ! -f "$target_resolved/hyprland.lua" ]; then
  ownership_collision
fi

if [ -L "$current_source" ]; then
  current_resolved="$(readlink -f -- "$current_source" 2>/dev/null || true)"
  if [ -n "$current_resolved" ] && [ "$target_resolved" = "$current_resolved" ]; then
    exit 0
  fi
fi

raw_target="$(readlink -- "$target" 2>/dev/null || true)"
[ -n "$raw_target" ] || ownership_collision
case "$raw_target" in
  /*) lexical_target="$raw_target" ;;
  *) lexical_target="$(dirname -- "$target")/$raw_target" ;;
esac
lexical_target="$(realpath -m -s -- "$lexical_target" 2>/dev/null || true)"

if [[ "$lexical_target" =~ ^/nix/store/[0-9a-df-np-sv-z]{32}-home-manager-files/\.config/hypr/end4$ ]]; then
  exit 0
fi

ownership_collision
