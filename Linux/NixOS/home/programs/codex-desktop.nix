{
  lib,
  pkgs,
  wahrwelt,
  wahrweltLib,
  ...
}:

let
  wahrweltPkgs = pkgs.wahrwelt or (pkgs.mysetup or { });
in
{
  config = lib.mkIf (wahrweltLib.presets.developerOrMore wahrwelt) {
    programs.codexDesktopLinux = {
      enable = true;
      cliPackage = wahrweltPkgs.codex;
    };
  };
}
