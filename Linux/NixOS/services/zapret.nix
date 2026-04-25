{ config, pkgs, ... }:

{
  # bol-van's upstream module collides on the same nftables hooks; keep one active.
  services.zapret.enable = false;

  services.zapret-discord-youtube = {
    enable = true;
    # install.sh patches this line - keep the `config = "..."` shape stable.
    config = "general (FAKE_TLS_AUTO_ALT3)";
  };
}

