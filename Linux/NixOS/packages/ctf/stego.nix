{ pkgs-stable, ... }:

# Steganography: hidden data in images/audio/files.

{
  environment.systemPackages = with pkgs-stable; [
    steghide
    stegseek
    zsteg
  ];
}
