{
  config,
  mysetupLib,
  pkgs,
  ...
}:

let
  meowrchSddmTheme = pkgs.callPackage ../packages/sddm-meowrch-theme.nix { };
in
{
  config = mysetupLib.mkIfPresetOrMore "desktop" config.mysetup {
    environment.etc."sddm/faces/${config.mysetup.user.username}.face.icon".source =
      ../themes/sddm-theme/icons/logo.png;

    services.gnome.gnome-keyring.enable = true;
    security.pam.services.sddm.enableGnomeKeyring = true;
    environment.systemPackages = [ pkgs.libsecret ];

    services.displayManager = {
      defaultSession = "hyprland";

      sddm = {
        enable = true;
        wayland.enable = true;
        package = pkgs.kdePackages.sddm;
        theme = "meowrch-sddm-theme";
        settings.Theme = {
          ThemeDir = "${meowrchSddmTheme}/share/sddm/themes";
          Current = "meowrch-sddm-theme";
          FacesDir = "/etc/sddm/faces";
        };
        extraPackages = with pkgs; [
          meowrchSddmTheme
          kdePackages.qtmultimedia
          gst_all_1.gstreamer
          gst_all_1.gst-plugins-base
          gst_all_1.gst-plugins-good
          gst_all_1.gst-plugins-bad
          gst_all_1.gst-plugins-ugly
          gst_all_1.gst-libav
          kdePackages.qtsvg
          kdePackages.qt5compat
          kdePackages.kirigami-addons
          kdePackages.qqc2-desktop-style
        ];
      };
    };
  };
}
