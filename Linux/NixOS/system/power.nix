{ pkgs, ... }:

{
  services.power-profiles-daemon.enable = true;

  services.upower = {
    enable = true;
    criticalPowerAction = "Hibernate";
    percentageLow = 20;
    percentageCritical = 10;
    percentageAction = 5;
  };

  services.udev.extraRules = ''
    ACTION=="add", SUBSYSTEM=="backlight", RUN+="${pkgs.coreutils}/bin/chgrp video /sys/class/backlight/%k/brightness"
    ACTION=="add", SUBSYSTEM=="backlight", RUN+="${pkgs.coreutils}/bin/chmod g+w /sys/class/backlight/%k/brightness"
  '';

  powerManagement = {
    enable = true;
    powertop.enable = false;
  };

  hardware.acpilight.enable = true;
}
