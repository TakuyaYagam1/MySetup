{
  inputs,
  wahrweltLib,
  pkgs-stable,
}:

{ config, ... }:

{
  home-manager = {
    useGlobalPkgs = true;
    useUserPackages = true;
    backupFileExtension = "backup";
    overwriteBackup = true;
    users.${config.wahrwelt.user.username} = import ../home/home.nix;
    extraSpecialArgs = {
      inherit
        inputs
        wahrweltLib
        pkgs-stable
        ;
      inherit (config) wahrwelt;
      mysetup = config.wahrwelt;
      mysetupLib = wahrweltLib;
    };
  };
}
