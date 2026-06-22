{
  config,
  lib,
  mysetupLib,
  pkgs,
  ...
}:

let
  grafanaSecretKeyPath = "/var/lib/grafana/secret_key";
  ensureGrafanaSecretKey = pkgs.writeShellScript "mysetup-grafana-secret-key" ''
    set -eu

    data_dir=/var/lib/grafana
    secret_key="$data_dir/secret_key"

    ${pkgs.coreutils}/bin/mkdir -p "$data_dir"
    ${pkgs.coreutils}/bin/chown grafana:grafana "$data_dir"
    ${pkgs.coreutils}/bin/chmod 0750 "$data_dir"

    if [ ! -s "$secret_key" ]; then
      umask 077
      ${pkgs.openssl}/bin/openssl rand -hex 32 > "$secret_key"
      ${pkgs.coreutils}/bin/chown grafana:grafana "$secret_key"
      ${pkgs.coreutils}/bin/chmod 0600 "$secret_key"
    fi
  '';
in
{
  config = lib.mkIf config.mysetup.features.observability {
    services = {
      grafana = {
        enable = true;
        settings = {
          server = {
            http_addr = "127.0.0.1";
            http_port = mysetupLib.ports.grafana;
            domain = "localhost";
          };

          security.secret_key = "$__file{${grafanaSecretKeyPath}}";
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

    systemd.services.mysetup-grafana-secret-key = {
      description = "Ensure Grafana secret key exists";
      before = [ "grafana.service" ];
      requiredBy = [ "grafana.service" ];
      serviceConfig = {
        Type = "oneshot";
        ExecStart = ensureGrafanaSecretKey;
      };
    };
  };
}
