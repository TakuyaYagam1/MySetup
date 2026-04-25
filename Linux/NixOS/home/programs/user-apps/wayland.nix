{ pkgs, ... }:

{
  home.packages = with pkgs; [
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
  ];
}
