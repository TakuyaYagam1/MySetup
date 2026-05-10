{
  config,
  mysetupLib,
  ...
}:

{
  config = mysetupLib.mkIfPresetOrMore "desktop" config.mysetup {
    services.flatpak.enable = true;
    services.snap.enable = true;
  };
}
