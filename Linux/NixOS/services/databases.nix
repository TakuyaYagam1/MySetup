{
  config,
  lib,
  pkgs,
  mysetupLib,
  ...
}:

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
    };
  };
}
