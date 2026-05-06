{ lib, pkgs, var, ... }:

let
  preset = var.packagePreset or "personal";
  enabled = lib.elem preset [ "desktop" "developer" "personal" ];
in
{
  config = lib.mkIf enabled {
    home.packages = with pkgs; [
      # Calculators
      libqalculate
      qalculate-gtk

      # System info / utilities
      bulky
      xneur
      app2unit

      # Terminal tools
      tmux
      zellij

      # drawing
      drawing
      ksnip
      pinta
    ];
  };
}
