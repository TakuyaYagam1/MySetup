{ config, lib, ... }:

{
  config = lib.mkIf config.wahrwelt.features.secureBoot {
    boot = {
      loader = {
        grub.enable = lib.mkForce false;
        systemd-boot.enable = lib.mkForce false;
      };

      plymouth.enable = lib.mkForce false;

      lanzaboote = {
        enable = true;
        pkiBundle = "/var/lib/sbctl";
      };
    };
  };
}
