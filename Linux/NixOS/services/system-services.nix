{ pkgs, lib, ... }:

{
  services.dbus.enable = true;
  services.gvfs.enable = true; 
  services.tumbler.enable = true;
  services.sing-box.enable = true;
  # services.omnirouter.enable = true;
  
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
    sandbox = false;
    max-jobs = 1;
    cores = 1;
    auto-optimise-store = true;
    experimental-features = [ "nix-command" "flakes" ];
  };
  
  systemd.user.services.nm-applet = {
    description = "NetworkManager Applet";
    wantedBy = [ "graphical-session.target" ];
    partOf = [ "graphical-session.target" ];
    serviceConfig = {
      ExecStart = "${pkgs.networkmanagerapplet}/bin/nm-applet --indicator";
      Restart = "on-failure";
    };
  };
}
