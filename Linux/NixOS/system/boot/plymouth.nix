{ pkgs, lib, ... }:

let
  plymouthTheme = pkgs.stdenv.mkDerivation {
    pname = "meowrch-plymouth-theme";
    version = "1.0";

    src = lib.cleanSource ../../themes/plymouth-theme;

    dontBuild = true;

    installPhase = ''
      dest=$out/share/plymouth/themes/meowrch
      mkdir -p $dest/assets

      cp -r assets/* $dest/assets/
      cp meowrch.plymouth $dest/
      cp meowrch.script $dest/

      cp ${../../themes/plymouth-theme/assets/logo.png} $dest/assets/logo.png

      substituteInPlace $dest/meowrch.plymouth \
        --replace "/usr/share/plymouth/themes/meowrch/assets" "$dest/assets" \
        --replace "/usr/share/plymouth/themes/meowrch/meowrch.script" "$dest/meowrch.script"
    '';
  };
in
{
  boot.plymouth = {
    enable = true;
    theme = "meowrch";
    themePackages = [ plymouthTheme ];
  };

  boot.initrd.systemd.enable = true;

  boot.kernelParams = [ "quiet" "splash" "udev.log_level=3" ];
}
