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

  dconf.settings = {
    "org/gnome/desktop/interface" = {
      color-scheme = "prefer-dark";
    };
  };

  wayland.windowManager.hyprland.enable = false;

  home.sessionVariables = {
    QS_ICON_THEME = qtIconTheme;
    QT_QPA_PLATFORMTHEME = qtPlatformTheme;
    QT_PLUGIN_PATH = "${qtPluginPath}:$QT_PLUGIN_PATH";
    XDG_DATA_DIRS = "${qtIconDataDirs}:$HOME/.nix-profile/share:/run/current-system/sw/share:$XDG_DATA_DIRS";
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
