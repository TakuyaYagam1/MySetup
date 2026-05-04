{ config, lib, pkgs, ... }:

{
  services.postgresql = {
    enable = true;
    package = pkgs.postgresql_17;
    settings = {
      listen_addresses = lib.mkForce "127.0.0.1";
      port = 5442;
    };
  };

  services.mysql = {
    enable = true;
    package = pkgs.mariadb;
    settings = {
      mysqld = {
        port = 3316;
        bind-address = "127.0.0.1";
      };
    };
  };

  services.pgadmin = {
    enable = true;
    port = 5050;
    initialEmail = config.var.services.pgadminEmail or "admin@localhost.local";
    initialPasswordFile = "/etc/nixos/secrets/pgadmin-password";
  };

  # Uncomment if needed:
  
  # services.redis.servers."default" = {
  #   enable = true;
  #   port = 6389;
  #   bind = "0.0.0.0";
  # };

  # services.memcached = {
  #   enable = true;
  #   maxMemory = 256;
  #   listen = "0.0.0.0";
  # };

  # services.clickhouse = {
  #   enable = true;
  #   extraServerConfig = ''
  #     <http_port>8133</http_port>
  #     <tcp_port>9010</tcp_port>
  #   '';
  # };
}
