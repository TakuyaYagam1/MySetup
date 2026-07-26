{
  config,
  wahrweltLib,
  ...
}:

{
  config = wahrweltLib.mkIfPresetOrMore "desktop" config.wahrwelt {
    programs.hyprland = {
      enable = true;
      xwayland.enable = true;
      withUWSM = true;
    };
  };
}
