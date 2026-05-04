{ config, pkgs, lib, ... }:

# Boot with secure boot enabled

{
  config = lib.mkIf (config.var.features.secureBoot or false) {
  boot.loader.grub.enable = lib.mkForce false;
  boot.loader.systemd-boot.enable = lib.mkForce false;
  boot.plymouth.enable = lib.mkForce false;

  boot.lanzaboote = {
    enable = true;
    pkiBundle = "/var/lib/sbctl";
  };
  };
}
