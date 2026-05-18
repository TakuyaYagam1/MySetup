{
  config,
  homeLibs,
  lib,
  pkgs,
  ...
}:

# Stylix owns GTK/cursor/fonts. Qt is pinned to qt6ct because Caelestia's Qt
# file dialogs render broken fallback icons through the GTK platform theme.

let
  inherit (homeLibs) qtRuntime;
  qtPlatformTheme = homeLibs.qtDefaults.platformTheme;
  qtIconTheme = "Papirus-Dark";
  qtIconPackages = with pkgs; [
    papirus-icon-theme
    kdePackages.breeze-icons
    adwaita-icon-theme
    hicolor-icon-theme
  ];
  qt6Runtime = with pkgs.kdePackages; [
    breeze
    qt6ct
    qtsvg
    qtimageformats
    qtwayland
  ];
  qtIconDataDirs = lib.makeSearchPath "share" qtIconPackages;
  xdgDataDirs = lib.concatStringsSep ":" [
    qtIconDataDirs
    "${config.home.homeDirectory}/.nix-profile/share"
    "/run/current-system/sw/share"
  ];
  qt5Runtime = with pkgs.qt5; [
    qtsvg
    qtwayland
  ];
  qtPluginPath = lib.concatStringsSep ":" [
    (qtRuntime.mkSearchPath [
      "lib/qt-6/plugins"
      "lib/qt6/plugins"
    ] qt6Runtime)
    (qtRuntime.mkSearchPath [
      "lib/qt-5.15.18/plugins"
      "lib/qt5/plugins"
    ] qt5Runtime)
  ];
  qtctConfig = ''
    [Appearance]
    custom_palette=true
    icon_theme=${qtIconTheme}
    standard_dialogs=default
    style=breeze

    [Fonts]
    fixed="${config.stylix.fonts.monospace.name},${toString config.stylix.fonts.sizes.applications}"
    general="${config.stylix.fonts.sansSerif.name},${toString config.stylix.fonts.sizes.applications}"
  '';
in
{
  home.packages = qtIconPackages ++ qt6Runtime ++ qt5Runtime;

  qt = {
    enable = true;
    platformTheme.name = lib.mkForce qtPlatformTheme;
  };

  gtk = {
    enable = true;
    iconTheme = {
      name = qtIconTheme;
      package = pkgs.papirus-icon-theme;
    };
  };

  # Stylix' Qt target injects XDG_CONFIG_DIRS using shell expansion syntax.
  # Home Manager writes session variables to environment.d, which only accepts
  # literal KEY=VALUE entries, so keep Qt theming explicit via qt6ct instead.
  stylix.targets.qt.enable = lib.mkForce false;

  dconf.settings = {
    "org/gnome/desktop/interface" = {
      color-scheme = "prefer-dark";
      icon-theme = qtIconTheme;
    };
  };

  wayland.windowManager.hyprland.enable = false;

  home.sessionVariables = {
    QS_ICON_THEME = qtIconTheme;
    QT_QPA_PLATFORMTHEME = qtPlatformTheme;
    QT_PLUGIN_PATH = qtPluginPath;
    XDG_DATA_DIRS = xdgDataDirs;
    CAELESTIA_SCREENSHOTS_DIR = "${config.home.homeDirectory}/Pictures/Screenshots";
  };

  xdg.configFile = {
    "qt5ct/qt5ct.conf" = lib.mkForce {
      force = true;
      text = qtctConfig;
    };
    "qt6ct/qt6ct.conf" = lib.mkForce {
      force = true;
      text = qtctConfig;
    };

    "swappy/config".text = ''
      [Default]
      save_dir=$HOME/Pictures/Screenshots
      save_filename_format=screenshot-%Y%m%d-%H%M%S.png
      show_panel=true
    '';
  };
}
