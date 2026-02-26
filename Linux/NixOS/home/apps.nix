{ pkgs, pkgs-stable, ... }:

{
  home.packages = with pkgs; [
    # Audio & Video
    cava
    libcava
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
    sqlit-tui
    
    # API tools
    yaak
    insomnia
    warp-terminal
    termius
    
    # Development
    antigravity
    vscode
    code-cursor
    pkgs-stable.zed-editor # in unstable probably u will build zed, is it error or what idk, so use the stable package
    flutter
    android-studio-full
    android-tools
    scrcpy
    qtcreator
    obsidian

    # AI
    gemini-cli
    claude-code
    codex
    
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
    xneur

    # games
    lutris
    heroic
  ];
}
