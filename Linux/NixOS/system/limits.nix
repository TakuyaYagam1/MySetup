{ lib, ... }:

{
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
}
