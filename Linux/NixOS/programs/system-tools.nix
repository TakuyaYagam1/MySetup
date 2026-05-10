{
  config,
  lib,
  mysetupLib,
  pkgs,
  ...
}:

let
  inherit (mysetupLib) presets;
  cfg = config.mysetup;
  cfgDir = cfg.host.configDirectory;
  desktopOrMore = presets.desktopOrMore cfg;
  personal = presets.personal cfg;
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
  programs = {
    amnezia-vpn = lib.mkIf personal {
      enable = true;
      package = amnezia-vpn-x11;
    };

    dconf.enable = true;

    appimage = lib.mkIf desktopOrMore {
      enable = true;
      binfmt = true;
    };

    nh = {
      enable = true;
      clean.enable = true;
      clean.extraArgs = "--keep-since 4d --keep 3";
      flake = cfgDir;
    };
  };

  environment.systemPackages = lib.optionals personal [
    pkgs.amneziawg-tools
  ];

  boot.extraModulePackages = lib.optionals personal [ config.boot.kernelPackages.amneziawg ];
  boot.kernelModules = lib.optionals personal [ "amneziawg" ];

  # AmneziaWG sends packets whose reverse path doesn't match the routing table;
  # strict rp_filter (set in system/networking.nix) drops them. Override to "loose".
  networking.firewall.checkReversePath = lib.mkIf personal (lib.mkForce "loose");
}
