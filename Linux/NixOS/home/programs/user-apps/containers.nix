{ lib, pkgs-stable, var, ... }:

let
  preset = var.packagePreset or "personal";
  enabled = lib.elem preset [ "developer" "personal" ];
in
{
  config = lib.mkIf enabled {
    home.packages = with pkgs-stable; [
      podman-desktop
    ];
  };
}
