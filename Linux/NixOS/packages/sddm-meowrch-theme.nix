{ pkgs, ... }:

pkgs.stdenv.mkDerivation {
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
    Email=
    Theme-Id=meowrch-sddm-theme
    Theme-API=2.0
    QtVersion=6
    EOF
  '';
}
