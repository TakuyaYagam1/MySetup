{ config, lib, pkgs, ... }:

let
  dotsRoot =
    let
      installedDots = ../../dots;
      repoDots = ../../../dots;
    in
      if builtins.pathExists installedDots then installedDots else repoDots;
  shellSelectorRoot = ./quickshell/mysetup-shell-selector;

  hyprScripts = [
    "close-active.sh"
    "noctalia-launcher.sh"
    "record-toggle.sh"
    "shell-selector.sh"
    "screenshot.sh"
    "spotify-toggle.sh"
    "start-shell.sh"
    "wsaction.fish"
  ];

  hyprScriptFiles = lib.genAttrs (map (name: "hypr/scripts/${name}") hyprScripts) (target: {
    force = true;
    executable = true;
    source = dotsRoot + "/${target}";
  });
in
{
  xdg.configFile = hyprScriptFiles // {
    "hypr/shell-profile.conf" = {
      force = true;
      text = ''
        # Runtime shell launcher. The selected shell is stored in
        # ${config.home.homeDirectory}/.local/state/mysetup/active-shell.
        exec-once = ${config.xdg.configHome}/hypr/scripts/start-shell.sh
      '';
    };

    "hypr/shell-keybinds.conf" = {
      force = true;
      source = dotsRoot + "/hypr/caelestia/keybinds.conf";
    };

    "quickshell/mysetup-shell-selector" = {
      force = true;
      source = shellSelectorRoot;
    };
  };

  home.activation.seedMySetupHyprConfig =
    lib.hm.dag.entryAfter [ "writeBoundary" ] ''
      target="${config.xdg.configHome}/hypr/mysetup/hyprland.conf"
      src="${dotsRoot}/hypr/hyprland.conf"

      $DRY_RUN_CMD mkdir -p "$(dirname "$target")"

      if [ -L "$target" ]; then
        $DRY_RUN_CMD rm -f "$target"
      fi

      if [ ! -e "$target" ]; then
        $DRY_RUN_CMD install -m 644 "$src" "$target"
      fi
    '';

  home.activation.startHyprShell =
    lib.hm.dag.entryAfter [ "seedMySetupHyprConfig" ] ''
      if command -v hyprctl >/dev/null 2>&1 && hyprctl instances >/dev/null 2>&1; then
        $DRY_RUN_CMD hyprctl reload >/dev/null 2>&1 || true
        $DRY_RUN_CMD ${config.xdg.configHome}/hypr/scripts/start-shell.sh >/dev/null 2>&1 || true
      fi
    '';
}
