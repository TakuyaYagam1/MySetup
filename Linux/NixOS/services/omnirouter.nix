{
  config,
  lib,
  pkgs,
  ...
}:
let
  cfg = config.services.omnirouter;
in
{
  options.services.omnirouter = {
    enable = lib.mkEnableOption "OmniRouter service";

    package = lib.mkOption {
      type = lib.types.package;
      description = "Binary package for OmniRouter";
      default = pkgs.callPackage ../packages/omnirouter.nix { };
    };

    extraArgs = lib.mkOption {
      type = lib.types.listOf lib.types.str;
      default = [ ];
      description = "Extra CLI args for omnirouter";
    };
  };

  config = lib.mkIf cfg.enable {
    systemd.services.omnirouter = {
      description = "OmniRouter LLM router";
      wantedBy = [ "multi-user.target" ];
      after = [ "network-online.target" ];
      wants = [ "network-online.target" ];

      path = [ pkgs.nodejs ];

      serviceConfig = {
        ExecStart = ''
          ${cfg.package}/bin/omnirouter \
            ${lib.escapeShellArgs cfg.extraArgs}
        '';
        Restart = "on-failure";

        Environment = [
          "HOME=/var/lib/omnirouter"
        ];

        DynamicUser = true;
        StateDirectory = "omnirouter";
        WorkingDirectory = "/var/lib/omnirouter";

        NoNewPrivileges = true;
        ProtectSystem = "strict";
        ProtectHome = true;
        PrivateTmp = true;
      };
    };
  };
}
