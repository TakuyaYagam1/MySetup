#!/usr/bin/env bash
# shellcheck shell=bash
# shellcheck disable=SC2154

dedupe_colon_path() {
  local raw="$1"
  local old_ifs="$IFS"
  local item result=""

  IFS=':'
  for item in $raw; do
    IFS="$old_ifs"
    case "$item" in
      ""|\$*) ;;
      *)
        case ":$result:" in
          *":$item:"*) ;;
          *) result="${result:+$result:}$item" ;;
        esac
        ;;
    esac
    IFS=':'
  done
  IFS="$old_ifs"

  printf '%s' "$result"
}

build_icon_theme_data_dirs() {
  local icon_root icon_path resolved data_dir result=""

  for icon_root in \
    "$HOME/.local/share/icons" \
    "$HOME/.nix-profile/share/icons" \
    "/etc/profiles/per-user/$user_name/share/icons" \
    "/run/current-system/sw/share/icons" \
    "/nix/profile/share/icons" \
    "/nix/var/nix/profiles/default/share/icons"; do
    [ -d "$icon_root" ] || continue

    for icon_path in "$icon_root"/*; do
      [ -e "$icon_path" ] || continue
      resolved="$(readlink -f "$icon_path" 2>/dev/null || true)"

      case "$resolved" in
        /nix/store/*/share/icons/*)
          data_dir="${resolved%%/share/icons/*}/share"
          result="${result:+$result:}$data_dir"
          ;;
      esac
    done
  done

  dedupe_colon_path "$result"
}

build_runtime_xdg_data_dirs() {
  local icon_theme_dirs

  icon_theme_dirs="$(build_icon_theme_data_dirs)"
  dedupe_colon_path "$HOME/.nix-profile/share:$HOME/.local/share:$HOME/.local/share/flatpak/exports/share:/etc/profiles/per-user/$user_name/share:/run/current-system/sw/share:/var/lib/flatpak/exports/share:/nix/profile/share:/nix/var/nix/profiles/default/share:/usr/local/share:/usr/share${icon_theme_dirs:+:$icon_theme_dirs}${XDG_DATA_DIRS:+:$XDG_DATA_DIRS}"
}

build_runtime_path() {
  dedupe_colon_path "$HOME/.nix-profile/bin:/etc/profiles/per-user/$user_name/bin:/run/current-system/sw/bin:/nix/profile/bin:/nix/var/nix/profiles/default/bin:/usr/local/bin:/usr/bin:/bin${PATH:+:$PATH}"
}

prepare_runtime_environment() {
  export XDG_DATA_DIRS
  export PATH
  export QS_ICON_THEME
  export QT_QPA_PLATFORMTHEME

  XDG_DATA_DIRS="$(build_runtime_xdg_data_dirs)"
  PATH="$(build_runtime_path)"
  QS_ICON_THEME="${QS_ICON_THEME:-Papirus-Dark}"
  QT_QPA_PLATFORMTHEME="${QT_QPA_PLATFORMTHEME:-qt6ct}"
}
