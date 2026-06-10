{
  inputs,
  mysetupLib,
  pkgs-bleeding,
  pkgs-stable,
}:

{ config, ... }:

{
  home-manager = {
    useGlobalPkgs = true;
    useUserPackages = true;
    backupFileExtension = "backup";
    users.${config.mysetup.user.username} = import ../home/home.nix;
    extraSpecialArgs = {
      inherit
        inputs
        mysetupLib
        pkgs-bleeding
        pkgs-stable
        ;
      inherit (config) mysetup;
    };
  };
}
