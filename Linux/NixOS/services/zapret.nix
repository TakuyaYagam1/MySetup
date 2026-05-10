{ config, lib, ... }:

{
  config = lib.mkIf config.mysetup.zapret.enable {
    # bol-van's upstream module collides on the same nftables hooks; keep one active.
    services.zapret.enable = false;

    services.zapret-discord-youtube = {
      enable = true;
      inherit (config.mysetup.zapret) config;
    };
  };
}
