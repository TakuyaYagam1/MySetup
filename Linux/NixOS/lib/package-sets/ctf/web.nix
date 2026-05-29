{ pkgs-stable, inputs, system }:

let
  burpsuitepro = inputs.burpsuitepro.packages.${system}.default;
  burpsuiteproWithDesktop = pkgs-stable.symlinkJoin {
    name = "burpsuitepro";
    paths = [ burpsuitepro ];
    postBuild = ''
      desktop=$out/share/applications/burpsuitepro.desktop
      rm -f "$desktop"
      install -Dm0644 ${burpsuitepro}/share/applications/burpsuitepro.desktop "$desktop"
      substituteInPlace "$desktop" \
        --replace-fail "Exec=burpsuitepro" "Exec=/run/current-system/sw/bin/burpsuitepro"
    '';
  };
in

with pkgs-stable;
[
  arjun
  burpsuiteproWithDesktop
  cadaver
  commix
  crlfuzz
  dalfox
  davtest
  dirb
  dirbuster
  doona
  eyewitness
  feroxbuster
  ffuf
  gau
  gobuster
  gophish
  gospider
  goshs
  gowitness
  grpcurl
  hakrawler
  httprobe
  httpie
  httpx
  httrack
  hurl
  joomscan
  jwt-cli
  katana
  laudanum
  lbd
  mitmproxy
  nikto
  nuclei
  photon
  proxify
  python3Packages.dirsearch
  siege
  slowhttptest
  sqlmap
  sqlmc
  ssldump
  sslh
  sslscan
  sslsplit
  sslstrip
  unfurl
  wafw00f
  # Temporarily disabled on 26.05: wapiti-arsenic 28.5 requires
  # packaging < 26, while nixpkgs currently provides packaging 26.1.
  # Wait for the nixpkgs/wapiti dependency metadata fix before re-enabling.
  # wapiti
  waybackurls
  websocat
  weevely
  websploit
  wfuzz
  whatweb
  wpprobe
  wpscan
]
