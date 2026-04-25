{ var, ... }:

{
  caelestiaShellSettings.session = {
    dragThreshold = 30;
    enabled = true;
    vimKeybinds = false;
    icons = {
      logout = "logout";
      shutdown = "power_settings_new";
      hibernate = "downloading";
      reboot = "cached";
    };
    commands = {
      logout = [ "pkill" "-KILL" "-u" var.username ];
      shutdown = [ "systemctl" "poweroff" ];
      hibernate = [ "systemctl" "hibernate" ];
      reboot = [ "systemctl" "reboot" ];
    };
  };

  caelestiaShellSettings.lock = {
    recolourLogo = false;
    hideNotifs = true;
  };
}
