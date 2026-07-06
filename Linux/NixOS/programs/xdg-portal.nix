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
      # xdg-open (double-click file opening) resolves directly against
      # mimeapps.list instead of routing through OpenURI - the GTK portal
      # backend does not reliably activate DBusActivatable apps (e.g.
      # org.gnome.Loupe) for local files outside a GNOME session.
      xdgOpenUsePortal = false;

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
