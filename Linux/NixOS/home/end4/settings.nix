{
  mysetupConfig,
  transparencyDefaults,
}:

let
  hypr = mysetupConfig.hypr or { };
  display = mysetupConfig.display or { };
  locale = mysetupConfig.locale or { };

  transparency = transparencyDefaults;

  renderMonitorRule =
    m:
    let
      isAuto = m.mode == "preferred" || m.mode == "auto" || m.mode == "";
    in
    if isAuto then
      ",${if m.mode == "" then "preferred" else m.mode},auto,${m.scale}"
    else
      "${m.name}, ${m.mode}, ${m.position}, ${m.scale}";

  primaryMonitor = {
    name = display.monitorName or "eDP-1";
    mode = display.monitorMode or "preferred";
    position = display.monitorPosition or "0x0";
    scale = display.monitorScale or "1";
  };
  extraMonitors = display.extraMonitors or [ ];

  monitor = rec {
    inherit (primaryMonitor)
      name
      mode
      position
      scale
      ;
    definitions = [ primaryMonitor ] ++ extraMonitors;
    rule = renderMonitorRule primaryMonitor;
    rules = [ rule ] ++ map renderMonitorRule extraMonitors;
  };
in
{
  inherit monitor transparency;

  idle = {
    lockTimeout = 600;
    screenOffTimeout = 1800;
    hibernateTimeout = 3600;
  };

  keyboard = {
    layouts = hypr.keyboardLayouts or "us,ru";
    toggle = hypr.keyboardToggle or "grp:alt_shift_toggle";
  };

  window.opacity = toString (hypr.windowOpacity or "0.85");

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
