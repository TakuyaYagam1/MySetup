{ config, lib, pkgs, pkgs-stable, ... }:

let
  preset = config.var.packagePreset or "personal";
  desktopOrMore = lib.elem preset [ "desktop" "developer" "personal" ];
  developerOrMore = lib.elem preset [ "developer" "personal" ];
  personal = preset == "personal";
in
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
      acpi
      powertop
      cifs-utils
      nfs-utils
      dos2unix
      ethtool
      expect
      mc
      plocate
      screen
      vim

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

      # File operations
      imagemagick
      file
      unzip
      zip
      rar
      p7zip
      libarchive
      trash-cli
      cloc
      sbctl
      efibootmgr

      # CLI utilities
      tree
      shellcheck
      jq
      ripgrep
      fd
      fzf
      eza
      zoxide
      atuin

      # System monitoring
      btop
      htop
      neohtop
      glances
    ])
    ++ lib.optionals desktopOrMore (with pkgs; [
      # Desktop integration
      steam-run
      networkmanagerapplet
      libnotify

      # Torrent
      qbittorrent
    ])
    ++ lib.optionals developerOrMore (with pkgs; [
      # Container tools
      docker-compose
      podman-compose

      # Developer terminal utilities
      yazi
      delta
    ])
    ++ lib.optionals personal (with pkgs; [
      # Proxy / VPN
      hysteria
      v2rayn
      clash-meta
      clash-nyanpasu
      mihomo
      sing-box
      sing-geoip
    ])
    ++ lib.optionals personal (with pkgs-stable; [
      # Wine
      (wineWow64Packages.staging.override {
        waylandSupport = true;
      })
      winetricks
      protontricks
      wineWowPackages.stable
    ]);
}
