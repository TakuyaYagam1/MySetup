{ lib, pkgs, ... }:

let
  pythonEnv = import ./python-env.nix { inherit pkgs; };
in
{
  home.packages =
    (with pkgs; [
      bc
      cliphist
      curl
      ddcutil
      eza
      ffmpeg
      geoclue2
      glib
      gnome-keyring
      grim
      hypridle
      hyprlock
      hyprpicker
      hyprshot
      hyprsunset
      imagemagick
      jq
      kitty
      libdbusmenu-gtk3
      libnotify
      libqalculate
      libsoup_3
      libportal-gtk4
      libsecret
      lxqt.pavucontrol-qt
      matugen
      mpvpaper
      playerctl
      pulseaudio
      ripgrep
      rsync
      slurp
      swww
      swappy
      translate-shell
      upower
      uv
      wf-recorder
      wget
      wl-clipboard
      wlogout
      wtype
      ydotool
      pythonEnv
      gobject-introspection
      qt6Packages.qt6ct
      kdePackages.bluedevil
      kdePackages.kconfig
      kdePackages.kde-cli-tools
      kdePackages.kdialog
      kdePackages.kirigami
      kdePackages.plasma-nm
      kdePackages.plasma-workspace
      kdePackages.systemsettings
      papirus-icon-theme
      hicolor-icon-theme
      adwaita-icon-theme
    ])
    ++ lib.optionals (pkgs ? fuzzel) [ pkgs.fuzzel ]
    ++ lib.optionals (pkgs ? songrec) [ pkgs.songrec ];
}
