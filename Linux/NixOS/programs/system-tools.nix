{ config, pkgs, lib, ... }:

let
  amnezia-vpn-x11 = pkgs.symlinkJoin {
    name = "amnezia-vpn-xwayland";
    paths = [ pkgs.amnezia-vpn ];
    buildInputs = [ pkgs.makeWrapper ];
    postBuild = ''
      wrapProgram $out/bin/AmneziaVPN \
        --set QT_QPA_PLATFORM xcb
        
      if [ -f $out/share/applications/AmneziaVPN.desktop ]; then
        sed -i 's|^Exec=.*|Exec='$out'/bin/AmneziaVPN|g' $out/share/applications/AmneziaVPN.desktop
      fi
    '';
  };
in
{
  programs.amnezia-vpn = {
    enable = true;
    package = amnezia-vpn-x11;
  };

  environment.systemPackages = [
    pkgs.amneziawg-tools
  ];

  boot.extraModulePackages = [ config.boot.kernelPackages.amneziawg ];
  boot.kernelModules = [ "amneziawg" ];
  programs.dconf.enable = true;
  
  programs.appimage = {
    enable = true;
    binfmt = true;
  };

  networking.firewall.checkReversePath = lib.mkForce "loose";
}