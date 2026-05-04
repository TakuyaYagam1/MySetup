{ config, lib, pkgs-stable, ... }:

# OSINT: people search, breach data, social media, image/exif analysis,
# DNS/email/typosquat reconnaissance.

{
  config = lib.mkIf (config.var.features.ctfTools or false) {
  environment.systemPackages = with pkgs-stable; [
    dnstwist
    exiflooter
    exifprobe
    exiftool
    exiv2
    gitleaks
    gitxray
    h8mail
    holehe
    instaloader
    maigret
    recon-ng
    sherlock
    sn0int
    theharvester
    trufflehog
  ];
  };
}
