{ config, pkgs, lib, ... }:

let
  cfgDir = config.var.configDirectory;
  preset = config.var.packagePreset or "personal";
  desktopOrMore = lib.elem preset [ "desktop" "developer" "personal" ];
  personal = preset == "personal";
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
  programs.amnezia-vpn = lib.mkIf personal {
    enable = true;
    package = amnezia-vpn-x11;
  };

  environment.systemPackages = lib.optionals personal [
    pkgs.amneziawg-tools
  ];

  boot.extraModulePackages = lib.optionals personal [ config.boot.kernelPackages.amneziawg ];
  boot.kernelModules = lib.optionals personal [ "amneziawg" ];
  programs.dconf.enable = true;

  programs.appimage = lib.mkIf desktopOrMore {
    enable = true;
    binfmt = true;
  };

  programs.nh = {
    enable = true;
    clean.enable = true;
    clean.extraArgs = "--keep-since 4d --keep 3";
    flake = cfgDir;
  };

  # AmneziaWG sends packets whose reverse path doesn't match the routing table;
  # strict rp_filter (set in system/networking.nix) drops them. Override to "loose".
  networking.firewall.checkReversePath = lib.mkIf personal (lib.mkForce "loose");
}
