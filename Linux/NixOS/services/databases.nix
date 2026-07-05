{
  config,
  lib,
  pkgs,
  mysetupLib,
  ...
}:

let
  cfg = config.mysetup;
in
{
  config = lib.mkMerge [
    (mysetupLib.mkIfPresetOrMore "developer" cfg {
      services.postgresql = {
        enable = true;
        package = pkgs.postgresql_17;
        settings = {
          listen_addresses = lib.mkForce "127.0.0.1";
          port = mysetupLib.ports.postgresql;
        };
      };
    })

    (lib.mkIf (mysetupLib.presets.personal cfg) {
      services.mysql = {
        enable = true;
        package = pkgs.mariadb;
        settings = {
          mysqld = {
            port = mysetupLib.ports.mariadb;
            bind-address = "127.0.0.1";
          };
        };
      };
    })
  ];
}
