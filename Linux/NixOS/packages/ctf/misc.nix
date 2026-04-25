{ pkgs-stable, ... }:

# CTF-specific extras that don't fit a narrower bucket: download/archive,
# headless browser, notes, shells.

{
  environment.systemPackages = with pkgs-stable; [
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
    unblob
    wgetpaste
    zim
    zsh
    zsh-autosuggestions
    zsh-syntax-highlighting
  ];
}
