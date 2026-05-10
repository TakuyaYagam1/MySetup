{
  lib,
  pkgs,
  pkgs-stable ? pkgs,
  russiaMode ? false,
}:

let
  wpsoffice-fixed = pkgs.wpsoffice.overrideAttrs (_: {
    src =
      pkgs.runCommandLocal "wps-office_11.1.0.11723.XA_amd64.deb"
        {
          outputHashMode = "recursive";
          outputHashAlgo = "sha256";
          outputHash = "sha256-o8njvwE/UsQpPuLyChxGAZ4euvwfuaHxs5pfUvcM7kI=";
          nativeBuildInputs = [
            pkgs.curl
            pkgs.coreutils
          ];
          impureEnvVars = lib.fetchers.proxyImpureEnvVars;
          SSL_CERT_FILE = "${pkgs.cacert}/etc/ssl/certs/ca-bundle.crt";
        }
        ''
          curl \
            -L \
            -A 'Mozilla/5.0' \
            --retry 3 --retry-delay 3 \
            "https://wdl1.pcfg.cache.wpscdn.com/wpsdl/wpsoffice/download/linux/11723/wps-office_11.1.0.11723.XA_amd64.deb" \
            > "$out"
        '';
  });
in
{
  apiTools =
    (with pkgs; [
      yaak
      insomnia
      dbeaver-bin
      sqlit-tui
      pkgs-stable.pgbadger
      warp-terminal
      termius
    ])
    ++ lib.optionals (!russiaMode) [
      pkgs.jetbrains.datagrip
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
    wpsoffice-fixed
    onlyoffice-desktopeditors
    obsidian
    anytype
  ];

  dev =
    (with pkgs; [
      vscode
      code-cursor
      qtcreator
      antigravity
      gemini-cli
      claude-code
      codex
      opencode
      opencode-claude-auth
      opencode-desktop
      flutter
      android-studio-full
      android-tools
      scrcpy
    ])
    ++ lib.optionals (!russiaMode) [
      pkgs.jetbrains.goland
    ];
  games = with pkgs-stable; [
    lutris
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
    xneur
    app2unit
    cmatrix
    tmux
    zellij
    drawing
    ksnip
    pinta
  ];
}
