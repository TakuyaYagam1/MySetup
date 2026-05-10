{
  lib,
  mysetup,
  mysetupLib,
  pkgs,
  ...
}:

let
  packageSets = import ../lib/package-sets.nix { inherit lib pkgs; };
in
{
  config = mysetupLib.mkIfPresetOrMore "developer" mysetup {
    home.packages = [ packageSets.development.python ];
  };
}
