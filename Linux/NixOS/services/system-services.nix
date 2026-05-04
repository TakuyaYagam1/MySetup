{ config, pkgs, lib, ... }:

let
  preset = config.var.packagePreset or "personal";
  desktopOrMore = lib.elem preset [ "desktop" "developer" "personal" ];
  personal = preset == "personal";
in
{
  services.dbus.enable = true;
  services.gvfs.enable = true;
  services.tumbler.enable = true;
  services.sing-box.enable = personal;
  services.omnirouter.enable = config.var.features.omnirouter or false;

  systemd.services.nix-daemon.serviceConfig = {
    LimitNOFILE = lib.mkForce "infinity";
    LimitNPROC = lib.mkForce "infinity";
    LimitMEMLOCK = lib.mkForce "infinity";
  };

  security.pam.loginLimits = [
    {
      domain = "*";
      type = "soft";
      item = "nofile";
      value = "65536";
    }
    {
      domain = "*";
      type = "hard";
      item = "nofile";
      value = "1048576";
    }
  ];

  systemd.settings.Manager = {
    DefaultLimitNOFILE = "65536:1048576";
  };

  nix.settings = {
    sandbox = true;
    max-jobs = "auto";
    auto-optimise-store = true;
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
}
