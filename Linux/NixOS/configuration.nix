{ config, pkgs, lib, inputs, ... }:

{
  imports = [
    ./hardware-configuration.nix
    
    # System
    ./system/boot/grub.nix
    ./system/boot/plymouth.nix
    # ./system/boot/secure.nix
    ./system/locale.nix
    ./system/networking.nix
    ./system/security.nix
    # ./system/nvidia-drivers.nix
    ./system/power.nix
    ./system/hardware.nix
    ./system/settings.nix
    
    # Services
    ./services/sddm.nix
    ./services/databases.nix
    ./services/observability.nix
    ./services/virtualization.nix
    ./services/system-services.nix
    # ./services/zapret.nix
    
    # Programs
    ./programs/hyprland.nix
    ./programs/thunar.nix
    ./programs/gaming.nix
    ./programs/fish.nix
    ./programs/development.nix
    ./programs/system-tools.nix
    
    # Packages & Users
    ./packages/system-packages.nix
    ./packages/dev-tools.nix
    ./packages/fonts.nix
    # ./packages/ctf-tools.nix
    ./users/user.nix
  ];

  boot.kernelPackages = lib.mkForce pkgs.linuxPackages_6_18;

  environment.sessionVariables = {
    NIXOS_OZONE_WL = "1";
    ELECTRON_OZONE_PLATFORM_HINT = "wayland";
  };

  system.stateVersion = "25.11";
}
