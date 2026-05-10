{ pkgs-stable }:

let
  # Upstream 25.5.26 currently fails its own filesystem handler pytest cases
  # on our pinned nixpkgs branch. Keep the tool available until nixpkgs picks
  # up a fixed release.
  unblobPatched = pkgs-stable.unblob.overrideAttrs (_: {
    doCheck = false;
    doInstallCheck = false;
  });
in
with pkgs-stable;
[
  above
  aria2
  axel
  cabextract
  cherrytree
  chromium
  cryptsetup
  gemini-cli
  gtkhash
  joplin
  macchanger
  qemu
  rake
  sqlitebrowser
  tzdata
  unar
  unblobPatched
  wgetpaste
  zim
  zsh
  zsh-autosuggestions
  zsh-syntax-highlighting
]
