{ pkgs, pkgs-stable, ... }:

{
  home.packages = with pkgs; [
    # Browsers
    firefox
    google-chrome

    # Communication
    spotify
    telegram-desktop
    pkgs-stable.vesktop

    # Office
    libreoffice-qt6-fresh
    wpsoffice-cn
    onlyoffice-desktopeditors

    # Notes / PKM
    obsidian
    anytype
  ];
}
