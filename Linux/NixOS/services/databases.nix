{
  config,
  lib,
  pkgs,
  mysetupLib,
  ...
}:

let
  pgAdminPasswordFile =
    if builtins.hasAttr "pgadmin-password" config.sops.secrets then
      config.sops.secrets."pgadmin-password".path
    else
      config.mysetup.services.pgadminPasswordFile;
in
{
  config = mysetupLib.mkIfPresetOrMore "developer" config.mysetup {
    services = {
      postgresql = {
        enable = true;
        package = pkgs.postgresql_17;
        settings = {
          listen_addresses = lib.mkForce "127.0.0.1";
          port = mysetupLib.ports.postgresql;
        };
      };

      mysql = {
        enable = true;
        package = pkgs.mariadb;
        settings = {
          mysqld = {
            port = mysetupLib.ports.mariadb;
            bind-address = "127.0.0.1";
          };
        };
      };

      pgadmin = {
        enable = true;
        port = mysetupLib.ports.pgadmin;
        initialEmail = config.mysetup.services.pgadminEmail or "admin@localhost.local";
        initialPasswordFile = pgAdminPasswordFile;
      };
    };
  };
}
