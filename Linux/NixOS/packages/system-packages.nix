{ pkgs, pkgs-stable, ... }:

{
  environment.pathsToLink = [ "/share/icons" ];

  environment.systemPackages =
    (with pkgs; [
      # Icons & Qt theming
      kdePackages.qt6ct
      kdePackages.breeze-icons
      adwaita-icon-theme
      hicolor-icon-theme
      papirus-icon-theme
      material-design-icons

      # System utilities
      wl-clipboard
      xclip
      steam-run
      acpi
      powertop
      networkmanagerapplet
      libnotify

      # Core tools
      git
      kitty
      neovim
      fish
      bash
      curl
      wget
      lsof
      nettools
      iproute2
      bind
      bat
      procs

      # Hardware control
      brightnessctl
      ddcutil
      lm_sensors
      inotify-tools

      # Database clients
      sqlite

      # Container tools
      docker-compose
      podman-compose

      # File operations
      imagemagick
      file
      unzip
      zip
      rar
      p7zip
      trash-cli
      cloc
      sbctl
      efibootmgr

      # CLI utilities
      tree
      jq
      ripgrep
      fd
      fzf
      eza

      # System monitoring
      btop
      htop
      neohtop
      glances

      # Torrent
      qbittorrent

      # Proxy
      hysteria
      v2rayn
      clash-meta
      clash-nyanpasu
      mihomo
      sing-box
      sing-geoip
    ])
    ++ (with pkgs-stable; [
      # Wine
      (wineWow64Packages.staging.override {
        waylandSupport = true;
      })
      winetricks
      protontricks
      wineWowPackages.stable
    ]);
}
