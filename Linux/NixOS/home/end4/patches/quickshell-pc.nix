{
  end4Lib,
  inputs,
  pkgs,
  ...
}:

let
  dotfilesSource = inputs.end4-pc;
  inherit (end4Lib) pythonEnv;
  quickshellPatcher = ./quickshell-pc.py;
  updaterNotice = "Wahrwelt manages end4-pC updates through the flake input";

  patchedQuickshellPC =
    pkgs.runCommand "end4-pc-quickshell-patched"
      {
        buildInputs = [
          pkgs.bash
          pythonEnv
        ];
      }
      ''
        cp -r ${dotfilesSource} $out
        chmod -R +w $out

        find $out -name '*.py' -print0 | xargs -0 sed -i 's|^#!.*ILLOGICAL_IMPULSE_VIRTUAL_ENV.*|#!/usr/bin/env python3|'
        sed -i 's|/dev/pts/\*|/dev/pts/* 2>/dev/null|' $out/scripts/colors/applycolor.sh

        ${pythonEnv}/bin/python ${quickshellPatcher} "$out" ${pkgs.lib.escapeShellArg updaterNotice}

        substituteInPlace $out/modules/common/Appearance.qml \
          --replace-fail 'property real contentTransparency: Config?.options.appearance.transparency.automatic ? autoContentTransparency : Config?.options.appearance.transparency.contentTransparency' \
                         'property real contentTransparency: Config?.options.appearance.transparency.enable ? (Config?.options.appearance.transparency.automatic ? autoContentTransparency : Config?.options.appearance.transparency.contentTransparency) : 0'

        substituteInPlace $out/services/SessionWarnings.qml \
          --replace-fail 'root.downloadRunning = (exitCode === 0);' \
                         'root.downloadRunning = false;'

        test -f "$out/shell.qml"
        patchShebangs $out
      '';
in
{
  # Both end-4 variants read the user-owned dynamic configuration from
  # ~/.config/illogical-impulse/config.json.
  xdg.configFile."quickshell/end4-pC" = {
    force = true;
    source = "${patchedQuickshellPC}";
  };
}
