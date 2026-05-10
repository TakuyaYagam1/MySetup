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

    # Dark Catppuccin Macchiato-ish palette, tweaked for Caelestia visual feel.
    base16Scheme = {
      base00 = "1e1e2e"; # Default Background
      base01 = "181825"; # Lighter BG (status bars)
      base02 = "313244"; # Selection BG
      base03 = "45475a"; # Comments, line highlight
      base04 = "585b70"; # Dark foreground
      base05 = "cdd6f4"; # Default foreground
      base06 = "f5e0dc"; # Light foreground
      base07 = "b4befe"; # Light BG
      base08 = "f38ba8"; # Red / diff deleted
      base09 = "fab387"; # Orange / constants
      base0A = "f9e2af"; # Yellow / classes
      base0B = "a6e3a1"; # Green / strings
      base0C = "94e2d5"; # Cyan / regex
      base0D = "89b4fa"; # Blue / functions
      base0E = "cba6f7"; # Purple / keywords
      base0F = "f2cdcd"; # Pink / embedded tags
    };

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
