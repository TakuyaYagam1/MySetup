{
  config,
  mysetupLib,
  pkgs,
  ...
}:

{
  config = mysetupLib.mkIfPresetOrMore "desktop" config.mysetup {
    programs.thunar = {
      enable = true;
      plugins = with pkgs; [
        thunar-archive-plugin
        thunar-volman
      ];
    };
  };
}
