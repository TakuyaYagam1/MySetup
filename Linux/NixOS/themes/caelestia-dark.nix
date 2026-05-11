# Caelestia-style dark palette: exports `config.theme.*` (UX knobs) and
# `config.stylix.*` (base16 + fonts + cursor) consumed across the config.
# Switch palettes by editing themes/active.nix.
{
  lib,
  pkgs,
  config,
  ...
}:

{
  options.theme = lib.mkOption {
    type = lib.types.attrs;
    default = {
      # Window chrome
      rounding = 18;
      gaps-in = 6;
      gaps-out = 12;
      border-size = 2;
      active-opacity = 0.97;
      inactive-opacity = 0.92;
      blur = true;
      # Animation pacing
      animation-speed = "fast"; # "fast" | "medium" | "slow"
      # Terminal gimmick (none|neofetch|pfetch|fastfetch)
      fetch = "fastfetch";
      textColorOnWallpaper = config.lib.stylix.colors.base00;
    };
    description = "Theme / UX knobs consumed across the config.";
  };

  config.stylix = {
    enable = true;

    cursor = {
      name = "Bibata-Modern-Classic";
      package = pkgs.bibata-cursors;
      size = 24;
    };

    fonts = {
      monospace = {
        package = pkgs.nerd-fonts.caskaydia-cove;
        name = "CaskaydiaCove Nerd Font";
      };
      sansSerif = {
        package = pkgs.rubik;
        name = "Rubik";
      };
      serif = config.stylix.fonts.sansSerif;
      emoji = {
        package = pkgs.noto-fonts-color-emoji;
        name = "Noto Color Emoji";
      };
      sizes = {
        applications = 11;
        desktop = 11;
        popups = 11;
        terminal = 12;
      };
    };

    polarity = "dark";

    image = ../Wallpapers/1.jpg;

    # Stylix handles GTK/Qt; Caelestia manages its own scheme via smartScheme.
    targets.gtk.enable = true;
    targets.qt.enable = true;
  };
}
