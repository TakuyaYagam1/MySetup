{ var, ... }:

{
  caelestiaShellSettings.launcher = {
    actionPrefix = ">";
    actions = [
      { name = "Calculator";   icon = "calculate";          description = "Do simple math equations (powered by Qalc)"; command = [ "autocomplete" "calc" ];        enabled = true;  dangerous = false; }
      { name = "Scheme";       icon = "palette";            description = "Change the current colour scheme";          command = [ "autocomplete" "scheme" ];      enabled = true;  dangerous = false; }
      { name = "Wallpaper";    icon = "image";              description = "Change the current wallpaper";              command = [ "autocomplete" "wallpaper" ];   enabled = true;  dangerous = false; }
      { name = "Variant";      icon = "colors";             description = "Change the current scheme variant";         command = [ "autocomplete" "variant" ];     enabled = true;  dangerous = false; }
      { name = "Transparency"; icon = "opacity";            description = "Change shell transparency";                 command = [ "autocomplete" "transparency" ]; enabled = true; dangerous = false; }
      { name = "Random";       icon = "casino";             description = "Switch to a random wallpaper";              command = [ "caelestia" "wallpaper" "-r" ]; enabled = true;  dangerous = false; }
      { name = "Light";        icon = "light_mode";         description = "Change the scheme to light mode";           command = [ "setMode" "light" ];            enabled = true;  dangerous = false; }
      { name = "Dark";         icon = "dark_mode";          description = "Change the scheme to dark mode";            command = [ "setMode" "dark" ];             enabled = true;  dangerous = false; }
      { name = "Shutdown";     icon = "power_settings_new"; description = "Shutdown the system";                       command = [ "systemctl" "poweroff" ];       enabled = true;  dangerous = true;  }
      { name = "Reboot";       icon = "cached";             description = "Reboot the system";                         command = [ "systemctl" "reboot" ];         enabled = true;  dangerous = true;  }
      { name = "Logout";       icon = "exit_to_app";        description = "Log out of the current session";            command = [ "pkill" "-KILL" "-u" var.username ]; enabled = true; dangerous = true; }
      { name = "Lock";         icon = "lock";               description = "Lock the current session";                  command = [ "loginctl" "lock-session" ];    enabled = true;  dangerous = false; }
      { name = "Sleep";        icon = "bedtime";            description = "Suspend then hibernate";                    command = [ "systemctl" "suspend-then-hibernate" ]; enabled = true; dangerous = false; }
      { name = "Settings";     icon = "settings";           description = "Configure the shell";                       command = [ "caelestia" "shell" "controlCenter" "open" ]; enabled = true; dangerous = false; }
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
    favouriteApps = [];
    hiddenApps = [];
  };
}
