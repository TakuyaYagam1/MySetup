{ pkgs-stable, ... }:

# Password attacks, hash cracking, wordlists, classical crypto attacks.

{
  environment.systemPackages = with pkgs-stable; [
    aesfix
    aeskeyfind
    bruteforce-luks
    bruteforce-salted-openssl
    bruteforce-wallet
    brutespray
    ccrypt
    cewl
    crowbar
    crunch
    fcrackzip
    hash-identifier
    hashcat
    hashcat-utils
    hashid
    hashrat
    hydra
    john
    johnny
    maskprocessor
    medusa
    ophcrack
    pack
    padbuster
    pdfcrack
    rsmangler
    seclists
    truecrack
    wordlists
    xortool
  ];
}
