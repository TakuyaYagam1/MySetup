{ pkgs, ... }:

let
  meowrchSddmTheme = pkgs.stdenv.mkDerivation {
    pname = "meowrch-sddm-theme";
    version = "1.0";
    src = ../themes/sddm-theme;

    dontBuild = true;
    installPhase = ''
      mkdir -p $out/share/sddm/themes/meowrch-sddm-theme
      cp -r $src/* $out/share/sddm/themes/meowrch-sddm-theme/
      chmod -R +w $out/share/sddm/themes/meowrch-sddm-theme
      cat > $out/share/sddm/themes/meowrch-sddm-theme/metadata.desktop <<EOF
[SddmGreeterTheme]
Name=Meowrch SDDM Theme
Type=sddm-theme
Version=1.0
Author=DIMFLIX
License=MIT
Screenshot=screenshot.png
MainScript=Main.qml
ConfigFile=theme.conf
TranslationsDirectory=translations
Email=skr1ms13666@gmail.com
Theme-Id=meowrch-sddm-theme
Theme-API=2.0
QtVersion=6
EOF
    '';
  };
in
{
  environment.etc."sddm/faces/takuya.face.icon".source = ../themes/sddm-theme/icons/logo.png;

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
}
