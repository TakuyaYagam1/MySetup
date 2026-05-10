{
  config,
  lib,
  mysetupLib,
  ...
}:

{
  config = lib.mkIf config.mysetup.features.observability {
    services = {
      grafana = {
        enable = true;
        settings.server = {
          http_addr = "127.0.0.1";
          http_port = mysetupLib.ports.grafana;
          domain = "localhost";
        };
      };

      prometheus = {
        enable = true;
        port = mysetupLib.ports.prometheus;
        scrapeConfigs = [
          {
            job_name = "node";
            static_configs = [
              {
                targets = [
                  "127.0.0.1:${toString config.services.prometheus.exporters.node.port}"
                ];
              }
            ];
          }
        ];
        exporters.node = {
          enable = true;
          enabledCollectors = [ "systemd" ];
          port = mysetupLib.ports.prometheusNodeExporter;
        };
      };

      loki = {
        enable = true;
        configuration = {
          server.http_listen_port = mysetupLib.ports.loki;
          auth_enabled = false;

          common = {
            ring = {
              instance_addr = "127.0.0.1";
              kvstore.store = "inmemory";
            };
            replication_factor = 1;
            path_prefix = "/var/lib/loki";
          };

          schema_config.configs = [
            {
              from = "2020-10-24";
              store = "tsdb";
              object_store = "filesystem";
              schema = "v13";
              index = {
                prefix = "index_";
                period = "24h";
              };
            }
          ];

          storage_config.filesystem.directory = "/var/lib/loki/chunks";
        };
      };
    };
  };
}
