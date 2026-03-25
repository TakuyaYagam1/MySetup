{ config, pkgs, pkgs-stable, ... }:

{
  programs.amnezia-vpn = {
    enable = true;
    package = pkgs-stable.amnezia-vpn;
  };

  boot.extraModulePackages = [ config.boot.kernelPackages.amneziawg ];

  environment.systemPackages = [
    pkgs-stable.amneziawg-tools
  ];

  programs.dconf.enable = true;
}