{ pkgs, pkgs-stable, lib, ... }:

let
  wpsoffice-fixed = pkgs.wpsoffice.overrideAttrs (_: {
    src = pkgs.runCommandLocal "wps-office_11.1.0.11723.XA_amd64.deb"
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
    wpsoffice-fixed
    onlyoffice-desktopeditors

    # Notes / PKM
    obsidian
    anytype
  ];
}
