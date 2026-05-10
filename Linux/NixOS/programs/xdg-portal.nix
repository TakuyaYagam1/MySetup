{
  config,
  mysetupLib,
  pkgs,
  ...
}:

{
  config = mysetupLib.mkIfPresetOrMore "desktop" config.mysetup {
    xdg.portal = {
      enable = true;
      xdgOpenUsePortal = true;

      extraPortals = with pkgs; [
        xdg-desktop-portal-gtk
        xdg-desktop-portal-hyprland
      ];

      config = {
        common.default = [ "gtk" ];
        hyprland.default = [
          "hyprland"
          "gtk"
        ];
      };
    };
  };
}
