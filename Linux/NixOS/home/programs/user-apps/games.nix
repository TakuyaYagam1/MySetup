{ lib, pkgs-stable, var, ... }:

let
  preset = var.packagePreset or "personal";
  enabled = preset == "personal";
in
{
  config = lib.mkIf enabled {
    home.packages = with pkgs-stable; [
      lutris
      heroic
    ];
  };
}
