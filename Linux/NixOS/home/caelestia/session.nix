{ wahrwelt, ... }:

{
  caelestiaShellSettings.session = {
    dragThreshold = 30;
    enabled = true;
    vimKeybinds = true;
    icons = {
      logout = "logout";
      shutdown = "power_settings_new";
      hibernate = "downloading";
      reboot = "cached";
    };
    commands = {
      logout = [
        "pkill"
        "-KILL"
        "-u"
        wahrwelt.user.username
      ];
      shutdown = [
        "systemctl"
        "poweroff"
      ];
      hibernate = [
        "systemctl"
        "hibernate"
      ];
      reboot = [
        "systemctl"
        "reboot"
      ];
    };
  };

  caelestiaShellSettings.lock = {
    enabled = true;
    recolourLogo = false;
    hideNotifs = true;
  };
}
