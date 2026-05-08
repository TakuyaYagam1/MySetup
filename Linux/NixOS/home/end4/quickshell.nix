{ inputs, lib, pkgs, ... }:

let
  system = pkgs.stdenv.hostPlatform.system;
  qsPackage = inputs.quickshell-end4.packages.${system}.default;
  qtRuntime = with pkgs; [
    qt6.qtbase
    qt6.qtdeclarative
    qt6.qt5compat
    qt6.qtimageformats
    qt6.qtmultimedia
    qt6.qtpositioning
    qt6.qtquicktimeline
    qt6.qtsensors
    qt6.qtsvg
    qt6.qttools
    qt6.qttranslations
    qt6.qtvirtualkeyboard
    qt6.qtwayland
    qt6.qtwebsockets
    kdePackages.kirigami
    kdePackages.syntax-highlighting
    gsettings-desktop-schemas
    adwaita-icon-theme
    hicolor-icon-theme
    papirus-icon-theme
  ];
in
{
  home.packages = qtRuntime ++ [
    (pkgs.writeShellScriptBin "qs" ''
      export QT_PLUGIN_PATH="${lib.makeSearchPath "lib/qt-6/plugins" qtRuntime}:${lib.makeSearchPath "lib/qt6/plugins" qtRuntime}:${lib.makeSearchPath "lib/plugins" qtRuntime}"
      export QML2_IMPORT_PATH="${lib.makeSearchPath "lib/qt-6/qml" qtRuntime}:${lib.makeSearchPath "lib/qt6/qml" qtRuntime}"
      export XDG_DATA_DIRS="${lib.makeSearchPath "share" qtRuntime}:$HOME/.nix-profile/share:$HOME/.local/share:$HOME/.local/share/flatpak/exports/share:/etc/profiles/per-user/$USER/share:/run/current-system/sw/share:/var/lib/flatpak/exports/share:/usr/local/share:/usr/share:$XDG_DATA_DIRS"
      export QT_WAYLAND_DISABLE_WINDOWDECORATION=1
      export QT_QPA_PLATFORMTHEME=gtk3

      exec ${qsPackage}/bin/qs "$@"
    '')
  ];
}
