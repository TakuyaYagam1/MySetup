{ pkgs, lib, ... }:

{
  environment.pathsToLink = [ "/share/icons" ];

  environment.systemPackages = with pkgs; [
    # Icons
    kdePackages.breeze-icons
    adwaita-icon-theme
    hicolor-icon-theme
    papirus-icon-theme
    material-design-icons

    # System utilities
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
    nmap
    bat
    procs
    antigravity
    
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

    # wine
    (pkgs.wineWowPackages.staging.override {
      waylandSupport = true;
    })
    winetricks
    protontricks 

    # yandex music tui
    (pkgs.buildGoModule rec {
      pname = "yamusic-tui";
      version = "0.7.1";

      src = pkgs.fetchFromGitHub {
        owner = "DECE2183";
        repo  = "yamusic-tui";
        rev   = "v${version}";
        hash  = "sha256-OYQpOUrphIIXcQHtzX5lrfEUM7KNY50kaHHln9ya3Z8=";
      };

      vendorHash = "sha256-x1dYqdsJtcankWjoq94CbBDx5iaroN+2aN/QoByq2t0=";

      nativeBuildInputs = [ pkgs.pkg-config ];
      buildInputs       = [ pkgs.alsa-lib ];

      meta = with lib; {
        description = "An unofficial Yandex Music terminal client";
        homepage    = "https://github.com/DECE2183/yamusic-tui";
        license     = licenses.gpl3Plus;
      };
    })
  ];
}
