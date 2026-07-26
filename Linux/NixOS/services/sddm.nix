{
  config,
  wahrweltLib,
  pkgs,
  ...
}:

let
  meowrchSddmTheme = pkgs.callPackage ../pkgs/sddm-meowrch-theme.nix { };
  cursorPackage = pkgs.bibata-cursors;
  cursorTheme = "Bibata-Modern-Classic";
  cursorSize = "24";
in
{
  config = wahrweltLib.mkIfPresetOrMore "desktop" config.wahrwelt {
    environment.etc."sddm/faces/${config.wahrwelt.user.username}.face.icon".source =
      wahrweltLib.bootTheme.resolveLogo
        {
          homeDirectory = config.wahrwelt.user.homeDirectory;
          service = "sddm";
          default = ../themes/sddm-theme/icons/logo.png;
        };

    services.gnome.gnome-keyring.enable = true;
    security.pam.services.sddm.enableGnomeKeyring = true;
    environment.systemPackages = [
      pkgs.libsecret
      cursorPackage
    ];

    systemd.services.display-manager.environment = {
      XCURSOR_THEME = cursorTheme;
      XCURSOR_SIZE = cursorSize;
      XCURSOR_PATH = "${cursorPackage}/share/icons:/run/current-system/sw/share/icons";
    };

    services.displayManager = {
      defaultSession = "hyprland-uwsm";

      sddm = {
        enable = true;
        wayland.enable = true;
        # Weston intermittently loses the hardware cursor on this AMD Wayland greeter.
        # KWin avoids that cursor-plane path while keeping SDDM on Wayland.
        wayland.compositor = "kwin";
        package = pkgs.kdePackages.sddm;
        theme = "meowrch-sddm-theme";
        settings.Theme = {
          ThemeDir = "${meowrchSddmTheme}/share/sddm/themes";
          Current = "meowrch-sddm-theme";
          CursorTheme = cursorTheme;
          CursorSize = cursorSize;
          FacesDir = "/etc/sddm/faces";
        };
        extraPackages = with pkgs; [
          meowrchSddmTheme
          cursorPackage
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
