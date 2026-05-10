{ lib, pkgs }:

let
  waylandCore = with pkgs; [
    cliphist
    hyprpicker
    wl-clipboard
    wtype
    grim
    slurp
    swappy
    hyprlock
    hypridle
    hyprpaper
    playerctl
    uwsm
    ydotool
  ];
  waylandTools =
    waylandCore
    ++ (with pkgs; [
      bluez
      gammastep
      gnome-keyring
      polkit_gnome
    ]);
in
{
  inherit waylandCore waylandTools;
  end4BinPackages =
    waylandTools
    ++ (with pkgs; [
      bc
      curl
      ddcutil
      eza
      ffmpeg
      geoclue2
      hyprshot
      hyprsunset
      imagemagick
      jq
      kitty
      lxqt.pavucontrol-qt
      matugen
      mpvpaper
      pulseaudio
      ripgrep
      rsync
      awww
      translate-shell
      upower
      uv
      wf-recorder
      wget
      wlogout
      kdePackages.bluedevil
      kdePackages.kconfig
      kdePackages.kde-cli-tools
      kdePackages.kdialog
      kdePackages.plasma-nm
      kdePackages.plasma-workspace
      kdePackages.systemsettings
    ])
    ++ lib.optionals (pkgs ? fuzzel) [ pkgs.fuzzel ]
    ++ lib.optionals (pkgs ? songrec) [ pkgs.songrec ];
  end4QtPackages = with pkgs; [
    glib
    gobject-introspection
    libdbusmenu-gtk3
    libnotify
    libqalculate
    libsoup_3
    libportal-gtk4
    libsecret
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
    qt6Packages.qt6ct
    kdePackages.kirigami.unwrapped
    kdePackages.syntax-highlighting
    gsettings-desktop-schemas
    adwaita-icon-theme
    hicolor-icon-theme
    papirus-icon-theme
  ];
}
