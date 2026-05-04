{ config, lib, pkgs, ... }:

{
  config = lib.mkIf (config.var.zapret.enable or false) {
  # bol-van's upstream module collides on the same nftables hooks; keep one active.
  services.zapret.enable = false;

  services.zapret-discord-youtube = {
    enable = true;
    config = config.var.zapret.config or "general (FAKE_TLS_AUTO_ALT3)";
  };
  };
}
