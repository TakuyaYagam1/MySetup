{ pkgs-stable, ... }:

# OSINT: people search, breach data, social media, image/exif analysis,
# DNS/email/typosquat reconnaissance.

{
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
}
