{
  config,
  homeLibs,
  pkgs,
  lib,
  ...
}:

let
  dotfilesLib = homeLibs.dotfiles;
  trans = homeLibs.transparency;
  mysetupPkgs = pkgs.mysetup or { };
  shellJson = pkgs.writeText "caelestia-shell.json" (builtins.toJSON config.caelestiaShellSettings);
in
{
  imports = [
    ./appearance.nix
    ./background.nix
    ./bar.nix
    ./general.nix
    ./launcher.nix
    ./services.nix
    ./session.nix
    ./utilities.nix
  ];

  options.caelestiaShellSettings = lib.mkOption {
    type = lib.types.attrsOf lib.types.anything;
    default = { };
    description = "Aggregated settings tree merged from slice modules and seeded into ~/.config/caelestia/shell.json.";
  };

  config = {
    programs.caelestia = {
      enable = true;
      package = mysetupPkgs.caelestia-shell or pkgs.caelestia-shell;

      systemd = {
        enable = false;
        target = "graphical-session.target";
        environment = [ ];
      };

      cli = {
        enable = true;
        settings.theme.enableGtk = false;
      };
    };

    home = {
      activation = {
        caelestiaSeedShellJson = homeLibs.shellSeed.mkSeedActivation {
          dirs = [ "$HOME/.config/caelestia" ];
          body = ''
            seed_json_object "$HOME/.config/caelestia/shell.json" "${shellJson}" "" '
                if .bar.excludedScreens? then .bar.excludedScreens |= map(select(. != "")) else . end |
                .general //= {} |
                .general.logo = "${config.caelestiaShellSettings.general.logo}" |
                .appearance //= {} |
                .appearance.transparency //= {} |
                .appearance.transparency.enabled //= true |
                ${dotfilesLib.mkOpacityDefault trans ".appearance.transparency.base"} |
                .appearance.transparency.layers //= ${toString trans.content} |
                .background //= {} |
                .background.desktopClock //= {} |
                .background.desktopClock.shadow //= {} |
                ${dotfilesLib.mkOpacityDefault trans ".background.desktopClock.shadow.opacity"} |
                .background.desktopClock.background //= {} |
                ${dotfilesLib.mkOpacityDefault trans ".background.desktopClock.background.opacity"}
            '
          '';
        };

        caelestiaSeedDynamicScheme = lib.hm.dag.entryAfter [ "writeBoundary" ] ''
          scheme_state="$HOME/.local/state/caelestia/scheme.json"
          if [ ! -e "$scheme_state" ]; then
            $DRY_RUN_CMD ${pkgs.coreutils}/bin/mkdir -p "$HOME/.local/state/caelestia"
            if command -v caelestia >/dev/null 2>&1; then
              $DRY_RUN_CMD caelestia scheme set -n dynamic -v rainbow >/dev/null 2>&1 || true
            fi
          fi
        '';
      };

      packages = [
        (mysetupPkgs.quickshell or pkgs.quickshell)
        # caelestia-shell uses xmllint to resolve XKB layout descriptions.
        pkgs.libxml2
      ];
    };
  };
}
