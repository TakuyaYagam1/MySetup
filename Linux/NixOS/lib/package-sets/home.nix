{
  pkgs,
  pkgs-stable ? pkgs,
}:

{
  apiTools = with pkgs; [
    yaak
    insomnia
    dbeaver-bin
    sqlit-tui
    ngrok
    pkgs-stable.pgbadger
    warp-terminal
    termius
    jetbrains.datagrip
  ];
  containers = with pkgs-stable; [
    podman-desktop
  ];
  desktop = with pkgs; [
    firefox
    google-chrome
    spotify
    telegram-desktop
    pkgs-stable.vesktop
    libreoffice-qt6-fresh
    wpsoffice
    onlyoffice-desktopeditors
    zathura
    loupe
    mpv
    obsidian
    anytype
  ];
  dev = with pkgs; [
    vscode
    code-cursor
    zed-editor-fhs
    qtcreator
    antigravity
    gemini-cli
    claude-code
    codex
    ollama
    opencode
    opencode-claude-auth
    opencode-desktop
    flutter
    android-studio-full
    android-tools
    scrcpy
  ];
  games = with pkgs-stable; [
    # Temporarily disabled until nixpkgs fixes openblas on i686-linux.
    # lutris
    heroic
  ];
  media = with pkgs; [
    cava
    libcava
    aubio
    gpu-screen-recorder
    pwvucontrol
    pavucontrol
    pamixer
  ];
  misc = with pkgs; [
    libqalculate
    qalculate-gtk
    bulky
    nemo
    app2unit
    cmatrix
    tmux
    zellij
    drawing
    ksnip
    pinta
  ];
}
