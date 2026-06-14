{
  inputs,
  mysetupLib,
  pkgs-stable,
}:

{ config, ... }:

{
  home-manager = {
    useGlobalPkgs = true;
    useUserPackages = true;
    backupFileExtension = "backup";
    overwriteBackup = true;
    users.${config.mysetup.user.username} = import ../home/home.nix;
    extraSpecialArgs = {
      inherit
        inputs
        mysetupLib
        pkgs-stable
        ;
      inherit (config) mysetup;
    };
  };
}
