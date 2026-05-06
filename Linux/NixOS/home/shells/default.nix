{ config, lib, pkgs, var, ... }:

let
  profile = var.shellProfile or "caelestia";

  dotsRoot =
    let
      installedDots = ../../dots;
      repoDots = ../../../dots;
    in
      if builtins.pathExists installedDots then installedDots else repoDots;

  shellKeybindsPath =
    if profile == "caelestia" then dotsRoot + "/hypr/caelestia/keybinds.conf"
    else if profile == "noctalia" then dotsRoot + "/hypr/noctalia/keybinds.conf"
    else dotsRoot + "/hypr/caelestia/keybinds.conf";

  hyprScripts = [
    "close-active.sh"
    "noctalia-launcher.sh"
    "record-toggle.sh"
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
        # Active shell profile: ${profile}
        exec-once = ${config.xdg.configHome}/hypr/scripts/start-shell.sh ${profile}
      '';
    };

    "hypr/shell-keybinds.conf" = {
      force = true;
      source = shellKeybindsPath;
    };
  };

  home.activation.startHyprShell =
    lib.hm.dag.entryAfter [ "writeBoundary" ] ''
      if command -v hyprctl >/dev/null 2>&1 && hyprctl instances >/dev/null 2>&1; then
        $DRY_RUN_CMD hyprctl reload >/dev/null 2>&1 || true
        $DRY_RUN_CMD ${config.xdg.configHome}/hypr/scripts/start-shell.sh ${profile} >/dev/null 2>&1 || true
      fi
    '';
}
