{ config, pkgs, ... }:

# Zapret for russian users
# Based on 
# [https://github.com/kartavkun/zapret-discord-youtube](https://github.com/kartavkun/zapret-discord-youtube)
# [https://github.com/bol-van/zapret](https://github.com/bol-van/zapret)

{
  services.zapret.enable = false; 

  services.zapret-discord-youtube = {
    enable = true;
    
    config = "general (FAKE_TLS_AUTO_ALT3)"; 
  };
}

