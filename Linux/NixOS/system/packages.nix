{
  config,
  lib,
  wahrweltLib,
  pkgs,
  pkgs-stable,
  ...
}:

let
  inherit (wahrweltLib) presets;
  cfg = config.wahrwelt;
  packageSets = import ../lib/package-sets.nix {
    inherit
      lib
      pkgs
      pkgs-stable
      ;
  };
in
{
  environment.pathsToLink = [ "/share/icons" ];

  environment.systemPackages =
    packageSets.system.base
    ++ lib.optionals (presets.desktopOrMore cfg) packageSets.system.desktop
    ++ lib.optionals (presets.developerOrMore cfg) packageSets.system.developer
    ++ lib.optionals (presets.personal cfg) packageSets.system.personal
    ++ lib.optionals (presets.personal cfg) packageSets.system.personalStable;
}
