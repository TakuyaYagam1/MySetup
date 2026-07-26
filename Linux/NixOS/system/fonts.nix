{
  config,
  lib,
  wahrweltLib,
  pkgs,
  pkgs-stable,
  ...
}:

let
  packageSets = import ../lib/package-sets.nix {
    inherit
      lib
      pkgs
      pkgs-stable
      ;
  };
in
{
  config = wahrweltLib.mkIfPresetOrMore "desktop" config.wahrwelt {
    fonts = {
      enableDefaultPackages = true;

      packages = packageSets.fonts.desktop;

      fontconfig = {
        defaultFonts = {
          serif = [
            "Noto Serif"
            "Liberation Serif"
          ];
          sansSerif = [
            "Noto Sans"
            "Liberation Sans"
          ];
          monospace = [
            "JetBrainsMono Nerd Font Mono"
            "Liberation Mono"
          ];
        };
      };
    };
  };
}
