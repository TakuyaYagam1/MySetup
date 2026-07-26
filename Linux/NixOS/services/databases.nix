{
  config,
  lib,
  pkgs,
  wahrweltLib,
  ...
}:

let
  cfg = config.wahrwelt;
in
{
  config = lib.mkMerge [
    (wahrweltLib.mkIfPresetOrMore "developer" cfg {
      services.postgresql = {
        enable = true;
        package = pkgs.postgresql_17;
        settings = {
          listen_addresses = lib.mkForce "127.0.0.1";
          port = wahrweltLib.ports.postgresql;
        };
      };
    })

    (lib.mkIf (wahrweltLib.presets.personal cfg) {
      services.mysql = {
        enable = true;
        package = pkgs.mariadb;
        settings = {
          mysqld = {
            port = wahrweltLib.ports.mariadb;
            bind-address = "127.0.0.1";
          };
        };
      };
    })
  ];
}
