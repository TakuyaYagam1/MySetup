{ config, lib, pkgs, inputs, ... }:

{
  imports = [
    ./hardware-configuration.nix
    
    # System
    ./system/boot/grub.nix
    # ./system/boot/secure.nix
    ./system/locale.nix
    ./system/networking.nix
    ./system/security.nix
    # ./system/nvidia-drivers.nix
    ./system/power.nix
    ./system/hardware.nix
    ./system/nix.nix
    
    # Services
    ./services/display.nix
    ./services/databases.nix
    ./services/observability.nix
    ./services/virtualization.nix
    ./services/system-services.nix
    # ./services/zapret.nix
    
    # Programs
    ./programs/desktop.nix
    ./programs/gaming.nix
    ./programs/shell.nix
    ./programs/development.nix
    ./programs/system-tools.nix
    
    # Packages & Users
    ./packages/system-packages.nix
    ./packages/dev-tools.nix
    ./packages/fonts.nix
    # ./packages/ctf-tools.nix
    ./users/takuya.nix
  ];

  system.stateVersion = "25.11";
}
