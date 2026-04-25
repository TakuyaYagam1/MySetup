{ config, ... }:

{
  # Uncomment if needed:

  # services.grafana = {
  #   enable = true;
  #   settings = {
  #     server = {
  #       http_addr = "0.0.0.0";
  #       http_port = 3010;
  #       domain = "localhost";
  #     };
  #   };
  # };

  # services.prometheus = {
  #   enable = true;
  #   port = 9100;
  #   scrapeConfigs = [
  #     {
  #       job_name = "node";
  #       static_configs = [{
  #         targets = [ "localhost:${toString config.services.prometheus.exporters.node.port}" ];
  #       }];
  #     }
  #   ];
  #   exporters = {
  #     node = {
  #       enable = true;
  #       enabledCollectors = [ "systemd" ];
  #       port = 9101;
  #     };
  #   };
  # };

  # services.loki = {
  #   enable = true;
  #   configuration = {
  #     server.http_listen_port = 3110;
  #     auth_enabled = false;
  #     
  #     common = {
  #       ring = {
  #         instance_addr = "127.0.0.1";
  #         kvstore.store = "inmemory";
  #       };
  #       replication_factor = 1;
  #       path_prefix = "/var/lib/loki";
  #     };
  #
  #     schema_config = {
  #       configs = [{
  #         from = "2020-10-24";
  #         store = "tsdb";
  #         object_store = "filesystem";
  #         schema = "v13";
  #         index = {
  #           prefix = "index_";
  #           period = "24h";
  #         };
  #       }];
  #     };
  #
  #     storage_config = {
  #       filesystem = {
  #         directory = "/var/lib/loki/chunks";
  #       };
  #     };
  #   };
  # };
}
