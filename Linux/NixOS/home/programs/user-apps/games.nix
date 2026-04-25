{ pkgs-stable, ... }:

{
  home.packages = with pkgs-stable; [
    lutris
    heroic
  ];
}
