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

        find $out -name '*.py' -print0 | xargs -0 sed -i 's|^#!.*ILLOGICAL_IMPULSE_VIRTUAL_ENV.*|#!/usr/bin/env python3|'
        sed -i 's|/dev/pts/\*|/dev/pts/* 2>/dev/null|' $out/ii/scripts/colors/applycolor.sh

        substituteInPlace $out/ii/services/HyprlandKeybinds.qml \
          --replace-fail "\''${Directories.config}/hypr/hyprland/keybinds.conf" "\''${Directories.config}/hypr/end4/hyprland/keybinds.conf" \
          --replace-fail "\''${Directories.config}/hypr/custom/keybinds.conf" "\''${Directories.config}/hypr/end4/mysetup/keybinds.conf"

        substituteInPlace $out/ii/modules/settings/About.qml \
          --replace-fail 'Quickshell.iconPath("illogical-impulse")' 'Quickshell.iconPath("nix-snowflake")'

        ${pythonEnv}/bin/python ${quickshellPatcher} "$out"

        substituteInPlace $out/ii/modules/common/Appearance.qml \
          --replace-fail 'property real contentTransparency: Config?.options.appearance.transparency.automatic ? autoContentTransparency : Config?.options.appearance.transparency.contentTransparency' \
                         'property real contentTransparency: Config?.options.appearance.transparency.enable ? (Config?.options.appearance.transparency.automatic ? autoContentTransparency : Config?.options.appearance.transparency.contentTransparency) : 0'

        substituteInPlace $out/ii/services/SessionWarnings.qml \
          --replace-fail 'root.downloadRunning = (exitCode === 0);' \
                         'root.downloadRunning = false;'

        patchShebangs $out
      '';
in
{
  xdg.configFile."quickshell/ii" = {
    force = true;
    source = "${patchedQuickshell}/ii";
  };
}
