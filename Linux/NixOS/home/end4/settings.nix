{
  mysetupConfig,
  transparencyDefaults,
}:

let
  hypr = mysetupConfig.hypr or { };
  display = mysetupConfig.display or { };
  locale = mysetupConfig.locale or { };

  transparency = transparencyDefaults;

  monitor = rec {
    name = display.monitorName or "eDP-1";
    mode = display.monitorMode or "2560x1600@120";
    position = display.monitorPosition or "0x0";
    scale = display.monitorScale or "1";
    rule = "${name}, ${mode}, ${position}, ${scale}";
  };
in
{
  inherit monitor transparency;

  hyprSourceFiles = [
    "hyprland/env.conf"
    "custom/env.conf"
    "hyprland/variables.conf"
    "custom/variables.conf"
    "hyprland/execs.conf"
    "hyprland/general.conf"
    "hyprland/rules.conf"
    "hyprland/colors.conf"
    "hyprland/keybinds.conf"
    "custom/execs.conf"
    "custom/general.conf"
    "custom/rules.conf"
    "custom/keybinds.conf"
    "workspaces.conf"
    "monitors.conf"
    "hyprland/shellOverrides/main.conf"
  ];

  idle = {
    lockTimeout = 600;
    screenOffTimeout = 1800;
    hibernateTimeout = 3600;
  };

  keyboard = {
    layouts = hypr.keyboardLayouts or "us,ru";
    toggle = hypr.keyboardToggle or "grp:alt_shift_toggle";
  };

  window.opacity = toString (hypr.windowOpacity or "0.8");

  fallbackBaseConfig = {
    appearance.transparency = {
      automatic = false;
      backgroundTransparency = transparency.shell;
      contentTransparency = transparency.content;
      enable = true;
    };
    background.wallpaperPath = "@HOME@/Pictures/Wallpapers/7.jpg";
    bar = {
      utilButtons = {
        showColorPicker = true;
        showDarkModeToggle = true;
        showKeyboardToggle = true;
        showMicToggle = true;
        showPerformanceProfileToggle = true;
        showScreenRecord = true;
        showScreenSnip = true;
      };
      verbose = true;
      vertical = true;
      weather = {
        city = locale.weatherLocation or "Moscow";
        enable = true;
        enableGPS = false;
        fetchInterval = 10;
        useUSCS = false;
      };
    };
    regionSelector.targetRegions = {
      contentRegionOpacity = transparency.regionContent;
      opacity = transparency.shell;
    };
  };
}
