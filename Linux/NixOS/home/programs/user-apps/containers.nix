{ pkgs-stable, ... }:

{
  home.packages = with pkgs-stable; [
    podman-desktop
  ];
}
