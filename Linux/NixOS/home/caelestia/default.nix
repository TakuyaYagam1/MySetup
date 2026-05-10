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

    home.activation.caelestiaSeedShellJson = homeLibs.shellSeed.mkSeedActivation {
      dirs = [ "$HOME/.config/caelestia" ];
      body = ''
        seed_json_object "$HOME/.config/caelestia/shell.json" "${shellJson}" "" '
            if .bar.excludedScreens? then .bar.excludedScreens |= map(select(. != "")) else . end |
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

    home.packages = [ (mysetupPkgs.quickshell or pkgs.quickshell) ];
  };
}
