{ config, ... }:

let
  mkAction =
    {
      name,
      icon,
      description,
      command,
      enabled ? true,
      dangerous ? false,
    }:
    {
      inherit
        name
        icon
        description
        command
        enabled
        dangerous
        ;
    };

  autocomplete = {
    calc = mkAction {
      name = "Calculator";
      icon = "calculate";
      description = "Do simple math equations (powered by Qalc)";
      command = [
        "autocomplete"
        "calc"
      ];
    };
    scheme = mkAction {
      name = "Scheme";
      icon = "palette";
      description = "Change the current colour scheme";
      command = [
        "autocomplete"
        "scheme"
      ];
    };
    wallpaper = mkAction {
      name = "Wallpaper";
      icon = "image";
      description = "Change the current wallpaper";
      command = [
        "autocomplete"
        "wallpaper"
      ];
    };
    variant = mkAction {
      name = "Variant";
      icon = "colors";
      description = "Change the current scheme variant";
      command = [
        "autocomplete"
        "variant"
      ];
    };
    transparency = mkAction {
      name = "Transparency";
      icon = "opacity";
      description = "Change shell transparency";
      command = [
        "autocomplete"
        "transparency"
      ];
    };
  };

  schemeMode = {
    light = mkAction {
      name = "Light";
      icon = "light_mode";
      description = "Change the scheme to light mode";
      command = [
        "setMode"
        "light"
      ];
    };
    dark = mkAction {
      name = "Dark";
      icon = "dark_mode";
      description = "Change the scheme to dark mode";
      command = [
        "setMode"
        "dark"
      ];
    };
  };

  session = {
    random = mkAction {
      name = "Random";
      icon = "casino";
      description = "Switch to a random wallpaper";
      command = [
        "caelestia"
        "wallpaper"
        "-r"
      ];
    };
    lock = mkAction {
      name = "Lock";
      icon = "lock";
      description = "Lock the current session";
      command = [
        "loginctl"
        "lock-session"
      ];
    };
    settings = mkAction {
      name = "Settings";
      icon = "settings";
      description = "Configure the shell";
      command = [
        "caelestia"
        "shell"
        "controlCenter"
        "open"
      ];
    };
  };

  systemPower = {
    shutdown = mkAction {
      name = "Shutdown";
      icon = "power_settings_new";
      description = "Shutdown the system";
      command = [
        "systemctl"
        "poweroff"
      ];
      dangerous = true;
    };
    reboot = mkAction {
      name = "Reboot";
      icon = "cached";
      description = "Reboot the system";
      command = [
        "systemctl"
        "reboot"
      ];
      dangerous = true;
    };
    logout = mkAction {
      name = "Logout";
      icon = "exit_to_app";
      description = "Log out of the current session";
      command = [
        "pkill"
        "-KILL"
        "-u"
        config.home.username
      ];
      dangerous = true;
    };
    sleep = mkAction {
      name = "Sleep";
      icon = "bedtime";
      description = "Suspend then hibernate";
      command = [
        "systemctl"
        "suspend-then-hibernate"
      ];
    };
  };
in
{
  caelestiaShellSettings.launcher = {
    actionPrefix = ">";
    actions = [
      autocomplete.calc
      autocomplete.scheme
      autocomplete.wallpaper
      autocomplete.variant
      autocomplete.transparency
      session.random
      schemeMode.light
      schemeMode.dark
      systemPower.shutdown
      systemPower.reboot
      systemPower.logout
      session.lock
      systemPower.sleep
      session.settings
    ];
    dragThreshold = 50;
    vimKeybinds = true;
    enableDangerousActions = true;
    maxShown = 7;
    maxWallpapers = 9;
    specialPrefix = "@";
    useFuzzy = {
      apps = false;
      actions = false;
      schemes = false;
      variants = false;
      wallpapers = false;
    };
    showOnHover = true;
    favouriteApps = [ ];
    hiddenApps = [ ];
  };
}
