#!/usr/bin/env bash
set -euo pipefail

mode="hypr"
if [ "${1:-}" = "--quickshell" ]; then
  mode="quickshell"
  shift
fi

artifact="${1:-}"
if [ -z "$artifact" ] || [ ! -d "$artifact" ]; then
  printf 'usage: %s [--quickshell] END4_ARTIFACT\n' "$0" >&2
  exit 64
fi

if [ "$mode" = "hypr" ]; then
  for file in hyprland.lua hyprland/keybinds.lua wahrwelt/keybinds.lua hypridle.conf; do
    if [ ! -f "$artifact/$file" ]; then
      printf 'FAIL: realized End4 artifact is missing %s\n' "$file" >&2
      exit 1
    fi
  done
elif [ ! -f "$artifact/shell.qml" ] && [ ! -f "$artifact/ii/shell.qml" ]; then
  printf 'FAIL: realized End4 QuickShell artifact is missing shell.qml\n' >&2
  exit 1
fi

tests_dir="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
python3 "$tests_dir/end4-artifact-lifecycle.py" "$artifact"
if [ -f "$artifact/ii/settings.qml" ] ||
  [ -f "$artifact/modules/ii/settings/Settings.qml" ] ||
  [ -f "$artifact/hyprland/variables.lua" ]; then
  python3 "$tests_dir/end4-native-settings.py" "$artifact"
fi

if [ "$mode" = "hypr" ]; then
  if ! grep -Fq '/scripts/close-active.sh' "$artifact/wahrwelt/keybinds.lua"; then
    printf 'FAIL: realized End4 artifact is missing the app-aware close binding\n' >&2
    exit 1
  fi

  if grep -R -Fq 'shell-common-rules.lua' "$artifact" ||
    grep -R -Fq 'require("shell-common-rules")' "$artifact"; then
    printf 'FAIL: realized End4 artifact loads shared rules directly\n' >&2
    exit 1
  fi

  for contract in \
    'quickshell:searchToggleRelease' \
    'quickshell:panelFamilyCycle' \
    'quickshell:sidebarRightToggle'; do
    if ! grep -Fq "$contract" "$artifact/hyprland/keybinds.lua"; then
      printf 'FAIL: realized End4 artifact is missing IPC contract %s\n' "$contract" >&2
      exit 1
    fi
  done
  if ! grep -Fq '/hypr/scripts/lock-active.sh' "$artifact/hypridle.conf"; then
    printf 'FAIL: realized End4 artifact is missing the managed lock contract\n' >&2
    exit 1
  fi
fi

printf 'OK realized End4 %s artifact %s\n' "$mode" "$artifact"
