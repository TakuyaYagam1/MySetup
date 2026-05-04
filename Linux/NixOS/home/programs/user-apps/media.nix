{ lib, pkgs, var, ... }:

let
  preset = var.packagePreset or "personal";
  enabled = lib.elem preset [ "minimal" "desktop" "developer" "personal" ];
in
{
  config = lib.mkIf enabled {
    home.packages = with pkgs; [
      # Audio visualisers / recording
      cava
      libcava
      aubio
      gpu-screen-recorder

      # Audio control
      pwvucontrol
      pavucontrol
      pamixer
      playerctl
    ];
  };
}
