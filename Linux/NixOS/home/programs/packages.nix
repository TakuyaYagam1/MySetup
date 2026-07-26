{
  lib,
  mysetup,
  wahrweltLib,
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
    (wahrweltLib.mkIfPresetOrMore "minimal" mysetup {
      home.packages = home.media ++ packageSets.runtime.waylandTools;
    })
    (wahrweltLib.mkIfPresetOrMore "desktop" mysetup {
      home.packages = home.desktop ++ home.misc;
    })
    (wahrweltLib.mkIfPresetOrMore "developer" mysetup {
      home.packages = home.apiTools ++ home.containers ++ home.dev;
    })
    (lib.mkIf (wahrweltLib.presets.personal mysetup) {
      home.packages = home.personal ++ home.games;
    })
  ];
}
