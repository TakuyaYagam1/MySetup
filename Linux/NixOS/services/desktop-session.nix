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
  desktopOrMore = presets.desktopOrMore cfg;
in
{
  services = {
    dbus.enable = true;
    gvfs.enable = desktopOrMore;
    tumbler.enable = desktopOrMore;
    udisks2.enable = desktopOrMore;
    omnirouter.enable = cfg.features.omnirouter;
    portainer.enable = cfg.features.portainer;
  };

  systemd.user.services.nm-applet = lib.mkIf desktopOrMore {
    description = "NetworkManager Applet";
    wantedBy = [ "graphical-session.target" ];
    partOf = [ "graphical-session.target" ];
    serviceConfig = {
      ExecStart = "${pkgs.networkmanagerapplet}/bin/nm-applet --indicator";
      Restart = "on-failure";
    };
  };

  systemd.user.services.polkit-gnome-authentication-agent-1 = lib.mkIf desktopOrMore {
    description = "GNOME Polkit Authentication Agent";
    wantedBy = [ "graphical-session.target" ];
    partOf = [ "graphical-session.target" ];
    serviceConfig = {
      ExecStart = "${pkgs.polkit_gnome}/libexec/polkit-gnome-authentication-agent-1";
      Restart = "on-failure";
    };
  };
}
