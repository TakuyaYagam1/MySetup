{
  config,
  wahrweltLib,
  ...
}:

{
  config = wahrweltLib.mkIfPresetOrMore "desktop" config.wahrwelt {
    services.flatpak.enable = true;
    services.snap.enable = true;
  };
}
