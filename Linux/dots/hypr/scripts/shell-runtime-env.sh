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
      "" | \$*) ;;
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
  dedupe_colon_path "/run/wrappers/bin:$HOME/.nix-profile/bin:/etc/profiles/per-user/$user_name/bin:/run/current-system/sw/bin:/nix/profile/bin:/nix/var/nix/profiles/default/bin:/usr/local/bin:/usr/bin:/bin${PATH:+:$PATH}"
}

clear_legacy_end4_environment() {
  local name
  local -a candidates=(
    WAHRWELT_END4_PROFILE
    WAHRWELT_QS_CONFIG
    qsConfig
    ILLOGICAL_IMPULSE_DOTFILES_SOURCE
    ILLOGICAL_IMPULSE_VIRTUAL_ENV
  )
  local -a names=()
  local -a empty_assignments=()
  local -A seen=()

  for name in "${candidates[@]}"; do
    [[ "$name" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]] || continue
    [ -z "${seen[$name]+x}" ] || continue
    seen[$name]=1
    names+=("$name")
    empty_assignments+=("$name=")
    unset "$name"
  done

  if command -v dbus-update-activation-environment >/dev/null 2>&1; then
    # D-Bus exposes no portable unset operation. Empty values remove the old
    # exact marker semantics while preventing stale End4 values from reaching
    # newly activated services.
    dbus-update-activation-environment "${empty_assignments[@]}" >/dev/null 2>&1 || true
  fi

  if command -v systemctl >/dev/null 2>&1; then
    systemctl --user unset-environment "${names[@]}" >/dev/null 2>&1 || true
  fi
}

prepare_runtime_environment() {
  clear_legacy_end4_environment

  export XDG_DATA_DIRS
  export PATH
  export QS_ICON_THEME
  export QT_QPA_PLATFORMTHEME

  XDG_DATA_DIRS="$(build_runtime_xdg_data_dirs)"
  PATH="$(build_runtime_path)"
  QS_ICON_THEME="${QS_ICON_THEME:-Papirus-Dark}"
  QT_QPA_PLATFORMTHEME="${QT_QPA_PLATFORMTHEME:-qt6ct}"
}

propagate_runtime_environment() {
  local vars=(
    XDG_DATA_DIRS
    PATH
    XDG_CURRENT_DESKTOP
    XDG_SESSION_DESKTOP
    QT_QPA_PLATFORMTHEME
    QS_ICON_THEME
    GTK_THEME
    HYPRLAND_INSTANCE_SIGNATURE
  )

  if command -v dbus-update-activation-environment >/dev/null 2>&1; then
    dbus-update-activation-environment --systemd "${vars[@]}" >/dev/null 2>&1 || true
  fi

  if command -v systemctl >/dev/null 2>&1; then
    systemctl --user import-environment "${vars[@]}" >/dev/null 2>&1 || true
  fi

  if command -v hyprctl >/dev/null 2>&1; then
    local v val
    for v in "${vars[@]}"; do
      val="$(printf '%s' "${!v-}")"
      [ -n "$val" ] || continue
      hyprctl setenv "$v" "$val" >/dev/null 2>&1 || true
    done
  fi

  local daemon
  for daemon in Thunar thunar tumblerd; do
    pkill -u "$USER" -x "$daemon" >/dev/null 2>&1 || true
  done
  if command -v systemctl >/dev/null 2>&1; then
    systemctl --user try-restart tumblerd.service >/dev/null 2>&1 || true
  fi
}
