{
  lib,
  mysetup,
  mysetupLib,
  pkgs,
  pkgs-stable ? pkgs,
  ...
}:

let
  packageSets = import ../../lib/package-sets.nix {
    inherit lib pkgs pkgs-stable;
  };
  home = packageSets.home { };
in
{
  config = lib.mkMerge [
    (mysetupLib.mkIfPresetOrMore "minimal" mysetup {
      home.packages = home.media ++ packageSets.runtime.waylandTools;
    })
    (mysetupLib.mkIfPresetOrMore "desktop" mysetup {
      home.packages = home.desktop ++ home.misc;
    })
    (mysetupLib.mkIfPresetOrMore "developer" mysetup {
      home.packages = home.apiTools ++ home.containers ++ home.dev;
    })
    (lib.mkIf (mysetupLib.presets.personal mysetup) {
      home.packages = home.games;
    })
  ];
}
