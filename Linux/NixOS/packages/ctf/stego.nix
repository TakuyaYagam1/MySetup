{ config, lib, pkgs-stable, ... }:

# Steganography: hidden data in images/audio/files.

{
  config = lib.mkIf (config.var.features.ctfTools or false) {
  environment.systemPackages = with pkgs-stable; [
    steghide
    stegseek
    zsteg
  ];
  };
}
