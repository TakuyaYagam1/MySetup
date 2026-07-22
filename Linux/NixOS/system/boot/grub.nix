{
  config,
  mysetupLib,
  pkgs,
  ...
}:

let
  grubLogo = mysetupLib.bootTheme.resolveLogo {
    homeDirectory = config.mysetup.user.homeDirectory;
    service = "grub";
    default = ../../themes/grub-theme/logo.png;
  };

  meowrchGrubTheme = pkgs.stdenv.mkDerivation {
    pname = "meowrch-grub-theme";
    version = "1.0";
    src = ../../themes/grub-theme;

    nativeBuildInputs = [ pkgs.imagemagick ];

    dontBuild = true;
    installPhase = ''
      mkdir -p $out
      cp -r ./* $out/
      convert ${grubLogo} -background none -resize 260x260^ -gravity center -extent 260x260 $out/logo.png
    '';
  };
in
{
  boot = {
    kernelParams = [
      "acpi_osi=Linux"
      "acpi_backlight=native"
    ];

    tmp.cleanOnBoot = true;

    loader = {
      efi = {
        canTouchEfiVariables = true;
        efiSysMountPoint = "/boot";
      };

      grub = {
        enable = true;
        efiSupport = true;
        device = "nodev";
        useOSProber = true;
        configurationLimit = 10;
        gfxmodeEfi = "1920x1080,auto";
        theme = meowrchGrubTheme;
      };
    };
  };
}
