{ pkgs, ... }:

let
  meowrchGrubTheme = pkgs.stdenv.mkDerivation {
    pname = "meowrch-grub-theme";
    version = "1.0";
    src = ../../themes/grub-theme;
    
    dontBuild = true;
    installPhase = ''
      mkdir -p $out
      cp -r ./* $out/
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
        gfxmodeEfi = "2560x1600";
        theme = meowrchGrubTheme;
      };
    };
  };

}
