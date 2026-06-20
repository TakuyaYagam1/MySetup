{
  config,
  lib,
  mysetupLib,
  pkgs,
  pkgs-stable,
  ...
}:

let
  inherit (mysetupLib) presets;
  cfg = config.mysetup;
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
