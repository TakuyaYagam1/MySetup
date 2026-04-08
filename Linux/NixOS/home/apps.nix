{ pkgs, pkgs-stable, ... }:

{
  # for drag and drop in vm
  xdg.desktopEntries.virt-manager = {
    name = "Virtual Machine Manager";
    exec = "env GDK_BACKEND=x11 virt-manager";
    icon = "virt-manager";
    terminal = false;
    categories = [ "System" ];
  };

  xdg.desktopEntries.thunar = {
    name = "Thunar";
    exec = "env GDK_BACKEND=x11 thunar %F";
    icon = "Thunar";
    terminal = false;
    categories = [ "System" "FileManager" ];
    mimeType = [ "inode/directory" ];
  };

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
    
    # Wayland tools
    xdg-desktop-portal-hyprland
    xdg-desktop-portal-gtk
    hyprpicker
    wl-clipboard
    
    # Office & Communication
    libreoffice-qt6-fresh
    wpsoffice-cn
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
    pkgs-stable.pgbadger
    
    # API tools
    yaak
    insomnia
    warp-terminal
    termius
    
    # Development
    antigravity
    vscode
    jetbrains.goland
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
    pkgs-stable.claude-code
    codex
    opencode
    opencode-claude-auth
    opencode-desktop
    
    # Screenshot & Screen capture
    grim
    slurp
    
    # Containers
    pkgs-stable.podman-desktop
    
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
    pkgs-stable.lutris
    pkgs-stable.heroic
  ];
}
