{ lib, pkgs, var, ... }:

let
  preset = var.packagePreset or "personal";
  enabled = lib.elem preset [ "minimal" "desktop" "developer" "personal" ];
in
{
  config = lib.mkIf enabled {
    home.packages = (with pkgs; [
    # Wayland tools
    hyprpicker
    wl-clipboard
    wtype
    grim
    slurp
    swappy

    # Hyprland session components
    hyprlock
    hypridle
    hyprpaper
    uwsm
    ])
    ++ lib.optional (var.shellProfile != "caelestia") pkgs.caelestia-cli;
  };
}
