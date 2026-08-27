{
  config,
  lib,
  wahrweltLib,
  pkgs,
  ...
}:

let
  grafanaSecretDirectory = "/var/lib/wahrwelt/grafana";
  grafanaSecretKeyPath = "${grafanaSecretDirectory}/secret_key";
  grafanaSecretKeyTool = pkgs.writeText "wahrwelt-grafana-secret-key.py" (
    builtins.readFile ./grafana-secret-key.py
  );
in
{
  config = lib.mkIf config.wahrwelt.features.observability {
    services = {
      grafana = {
        enable = true;
        settings = {
          server = {
            http_addr = "127.0.0.1";
            http_port = wahrweltLib.ports.grafana;
            domain = "localhost";
          };

          security.secret_key = "$__file{${grafanaSecretKeyPath}}";
        };
      };

      prometheus = {
        enable = true;
        listenAddress = "127.0.0.1";
        port = wahrweltLib.ports.prometheus;
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
          listenAddress = "127.0.0.1";
          port = wahrweltLib.ports.prometheusNodeExporter;
        };
      };

      loki = {
        enable = true;
        configuration = {
          server = {
            http_listen_address = "127.0.0.1";
            http_listen_port = wahrweltLib.ports.loki;
            grpc_listen_address = "127.0.0.1";
          };
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

    systemd.services.wahrwelt-grafana-secret-key = {
      description = "Ensure Grafana secret key exists";
      before = [ "grafana.service" ];
      requiredBy = [ "grafana.service" ];
      serviceConfig = {
        Type = "oneshot";
        Group = "grafana";
        StateDirectory = "wahrwelt/grafana";
        StateDirectoryMode = "0750";
        UMask = "0027";
        ExecStart = "${pkgs.python3}/bin/python3 ${grafanaSecretKeyTool} ${grafanaSecretDirectory} root grafana";
        NoNewPrivileges = true;
        PrivateTmp = true;
        ProtectHome = true;
        ProtectSystem = "strict";
        ReadWritePaths = [ grafanaSecretDirectory ];
      };
    };

    systemd.services.wahrwelt-v1-to-v2-grafana-secret-key = {
      description = "Migrate the v1 Grafana secret into root-owned storage";
      before = [ "wahrwelt-grafana-secret-key.service" ];
      requiredBy = [ "wahrwelt-grafana-secret-key.service" ];
      unitConfig.ConditionPathExists = [
        "/var/lib/grafana/secret_key"
        "!${grafanaSecretKeyPath}"
      ];
      serviceConfig = {
        Type = "oneshot";
        Group = "grafana";
        StateDirectory = "wahrwelt/grafana";
        StateDirectoryMode = "0750";
        UMask = "0027";
        ExecStart = "${pkgs.python3}/bin/python3 ${grafanaSecretKeyTool} ${grafanaSecretDirectory} root grafana /var/lib/grafana/secret_key";
        NoNewPrivileges = true;
        PrivateTmp = true;
        ProtectHome = true;
        ProtectSystem = "strict";
        ReadOnlyPaths = [ "/var/lib/grafana" ];
        ReadWritePaths = [ grafanaSecretDirectory ];
      };
    };
  };
}
