{ config, ... }:

# GTK/Qt/cursor/fonts are now managed by stylix (see themes/caelestia-dark.nix).
# Only the bits stylix does not touch are kept here.

{
  dconf.settings = {
    "org/gnome/desktop/interface" = {
      color-scheme = "prefer-dark";
    };
  };

  wayland.windowManager.hyprland.enable = false;

  home.sessionVariables = {
    XDG_DATA_DIRS = "$HOME/.nix-profile/share:/run/current-system/sw/share:$XDG_DATA_DIRS";
    CAELESTIA_SCREENSHOTS_DIR = "${config.home.homeDirectory}/Pictures/Screenshots";
  };

  xdg.configFile."swappy/config".text = ''
    [Default]
    save_dir=$HOME/Pictures/Screenshots
    save_filename_format=screenshot-%Y%m%d-%H%M%S.png
    show_panel=true
  '';
}
