{ config, pkgs, ... }:

{
  programs.amnezia-vpn.enable = true;

  boot.extraModulePackages = [ config.boot.kernelPackages.amneziawg ];
  
  environment.systemPackages = [ pkgs.amneziawg-tools ];
}
