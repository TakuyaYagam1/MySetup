{
  end4Lib,
  inputs,
  pkgs,
  ...
}:

let
  dotfilesSource = inputs.end4-dotfiles;
  inherit (end4Lib) pythonEnv;
  quickshellPatcher = ./quickshell.py;

  patchedQuickshell =
    pkgs.runCommand "end4-quickshell-patched"
      {
        buildInputs = [
          pkgs.bash
          pythonEnv
        ];
      }
      ''
        cp -r ${dotfilesSource}/dots/.config/quickshell $out
        chmod -R +w $out

        # Replace upstream venv shebangs with /usr/bin/env so our Nix pythonEnv runs scripts.
        find $out -name '*.py' -print0 | xargs -0 sed -i 's|^#!.*ILLOGICAL_IMPULSE_VIRTUAL_ENV.*|#!/usr/bin/env python3|'
        # Suppress missing-pty error spam from applycolor.sh on headless TTYs.
        sed -i 's|/dev/pts/\*|/dev/pts/* 2>/dev/null|' $out/ii/scripts/colors/applycolor.sh

        # Re-route keybind lookups to the end4 namespace and our MySetup overrides.
        substituteInPlace $out/ii/services/HyprlandKeybinds.qml \
          --replace-fail "\''${Directories.config}/hypr/hyprland/keybinds.conf" "\''${Directories.config}/hypr/end4/hyprland/keybinds.conf" \
          --replace-fail "\''${Directories.config}/hypr/custom/keybinds.conf" "\''${Directories.config}/hypr/end4/mysetup/keybinds.conf"

        # Use the NixOS snowflake icon instead of the upstream illogical-impulse one.
        substituteInPlace $out/ii/modules/settings/About.qml \
          --replace-fail 'Quickshell.iconPath("illogical-impulse")' 'Quickshell.iconPath("nix-snowflake")'

        ${pythonEnv}/bin/python ${quickshellPatcher} "$out"

        # Force contentTransparency to 0 when transparency is disabled (upstream
        # ignores the toggle and keeps surfaces translucent).
        substituteInPlace $out/ii/modules/common/Appearance.qml \
          --replace-fail 'property real contentTransparency: Config?.options.appearance.transparency.automatic ? autoContentTransparency : Config?.options.appearance.transparency.contentTransparency' \
                         'property real contentTransparency: Config?.options.appearance.transparency.enable ? (Config?.options.appearance.transparency.automatic ? autoContentTransparency : Config?.options.appearance.transparency.contentTransparency) : 0'

        patchShebangs $out
      '';
in
{
  xdg.configFile."quickshell/ii" = {
    force = true;
    source = "${patchedQuickshell}/ii";
  };
}
