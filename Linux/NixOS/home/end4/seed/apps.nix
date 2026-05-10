{
  end4Lib,
  inputs,
  lib,
  ...
}:

let
  dotfilesSource = inputs.end4-dotfiles;
  inherit (end4Lib) dotfilesLib;
in
{
  home.activation.end4SeedAppConfig = lib.hm.dag.entryAfter [ "writeBoundary" ] ''
    kitty_target="$HOME/.config/kitty"
    fuzzel_target="$HOME/.config/fuzzel"
    applications_dir="$HOME/.local/share/applications"
    kdeglobals_target="$HOME/.config/kdeglobals"
    konsole_target="$HOME/.local/share/konsole"
    $DRY_RUN_CMD mkdir -p "$applications_dir"
    $DRY_RUN_CMD mkdir -p "$konsole_target"
    ${dotfilesLib.mutableJsonShellHelpers}

    # section: kdeglobals - drop store symlink, seed mutable copy, ensure writable
    if [ -L "$kdeglobals_target" ]; then
      $DRY_RUN_CMD rm -f "$kdeglobals_target"
    fi
    if [ ! -e "$kdeglobals_target" ]; then
      $DRY_RUN_CMD install -m 644 "${dotfilesSource}/dots/.config/kdeglobals" "$kdeglobals_target"
    fi
    if [ -e "$kdeglobals_target" ]; then
      $DRY_RUN_CMD chmod u+w "$kdeglobals_target"
    fi

    # section: konsole - seed default profile if missing
    drop_store_symlink "$konsole_target/Profile 1.profile"
    if [ ! -e "$konsole_target/Profile 1.profile" ]; then
      $DRY_RUN_CMD install -m 644 "${dotfilesSource}/dots/.local/share/konsole/Profile 1.profile" "$konsole_target/Profile 1.profile"
    fi

    # section: kitty + fuzzel - replace store symlinks with mutable dirs, seed contents
    for runtime_dir in "$kitty_target" "$fuzzel_target"; do
      if [ -L "$runtime_dir" ]; then
        $DRY_RUN_CMD rm -f "$runtime_dir"
      fi
    done
    if [ ! -e "$kitty_target" ]; then
      $DRY_RUN_CMD mkdir -p "$kitty_target"
    fi
    if [ ! -e "$fuzzel_target" ]; then
      $DRY_RUN_CMD mkdir -p "$fuzzel_target"
    fi
    $DRY_RUN_CMD cp -rn "${dotfilesSource}/dots/.config/kitty/." "$kitty_target/" 2>/dev/null || true
    $DRY_RUN_CMD cp -rn "${dotfilesSource}/dots/.config/fuzzel/." "$fuzzel_target/" 2>/dev/null || true
    $DRY_RUN_CMD chmod -R u+rwX "$kitty_target" "$fuzzel_target" 2>/dev/null || true

    # section: thunar - patch userapp-thunar-*.desktop entries that lack an Icon=
    for desktop_file in "$applications_dir"/userapp-thunar-*.desktop; do
      if [ -f "$desktop_file" ] && ! grep -q '^Icon=' "$desktop_file" && grep -q '^Exec=.*thunar' "$desktop_file"; then
        $DRY_RUN_CMD chmod u+w "$desktop_file" 2>/dev/null || true
        $DRY_RUN_CMD sed -i '/^\[Desktop Entry\]/a Icon=Thunar' "$desktop_file"
      fi
    done

    # section: rfdump - copy system desktop entry with absolute icon path
    rfdump_desktop="$applications_dir/rfdump.desktop"
    rfdump_icon="/run/current-system/sw/share/pixmaps/rfdump.png"
    if [ -f "/run/current-system/sw/share/applications/rfdump.desktop" ] && [ -f "$rfdump_icon" ]; then
      if [ -e "$rfdump_desktop" ]; then
        $DRY_RUN_CMD chmod u+w "$rfdump_desktop" 2>/dev/null || true
        $DRY_RUN_CMD rm -f "$rfdump_desktop"
      fi
      $DRY_RUN_CMD install -m 644 "/run/current-system/sw/share/applications/rfdump.desktop" "$rfdump_desktop"
      $DRY_RUN_CMD sed -i "s|^Icon=.*|Icon=$rfdump_icon|" "$rfdump_desktop"
    fi
  '';
}
