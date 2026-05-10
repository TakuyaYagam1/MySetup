{
  preset,
  category ? null,
  customSelector ? null,
  needsRussiaMode ? false,
}:

{
  lib,
  mysetup,
  mysetupLib,
  pkgs,
  pkgs-stable ? pkgs,
  ...
}:

let
  enabled = mysetupLib.presets.${preset} mysetup;
  packageSets = import ./package-sets.nix {
    inherit lib pkgs pkgs-stable;
  };
  homeArgs = lib.optionalAttrs needsRussiaMode {
    inherit (mysetup.features) russiaMode;
  };
  homePackages = packageSets.home homeArgs;
  selected =
    if customSelector != null then
      customSelector { inherit packageSets homePackages; }
    else
      homePackages.${category};
in
{
  config = lib.mkIf enabled {
    home.packages = selected;
  };
}
