{ pkgs, pkgs-stable, ... }:

{
  home.packages = with pkgs; [
    # Audio & Video
    cava
    app2unit
    aubio
    gpu-screen-recorder
    
    # Desktop utilities
    swappy
    libqalculate
    amnezia-vpn
    
    # Wayland tools
    xdg-desktop-portal-hyprland
    xdg-desktop-portal-gtk
    hyprpicker
    wl-clipboard
    
    # Office & Communication
    libreoffice-qt6-fresh
    wpsoffice
    onlyoffice-desktopeditors
    spotify
    telegram-desktop
    
    # Audio control
    pwvucontrol
    pkgs-stable.vesktop
    
    # Terminal & Shell
    tmux
    zellij
    foot
    starship
    
    # Databases
    dbeaver-bin
    jetbrains.datagrip
    pgadmin4-desktopmode
    sqlit-tui
    
    # API tools
    yaak
    insomnia
    warp-terminal
    termius
    
    # Development
    vscode
    code-cursor
    (pkgs.zed-editor.overrideAttrs (old: {
      doCheck = false;
      checkPhase = "";
    }))
    flutter
    android-studio-full
    android-tools
    scrcpy
    qtcreator
    obsidian
    
    # Screenshot & Screen capture
    grim
    slurp
    
    # Containers
    podman-desktop
    
    # System utilities
    wtype
    pamixer
    pavucontrol
    playerctl
    
    # Browsers
    firefox
    google-chrome
    
    # Other tools
    fastfetch
    bulky
    qalculate-gtk
    hyprlock
    hypridle
    hyprpaper
    uwsm

    # games
    lutris
    heroic
  ];
}
