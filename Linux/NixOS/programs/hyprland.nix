{
  config,
  mysetupLib,
  ...
}:

{
  config = mysetupLib.mkIfPresetOrMore "desktop" config.mysetup {
    programs.hyprland = {
      enable = true;
      xwayland.enable = true;
    };
  };
}
