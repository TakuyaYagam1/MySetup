_:

{
  imports = [
    ../system/boot/grub.nix
    ../system/boot/plymouth.nix
    ../system/boot/secure.nix
    ../system/locale.nix
    ../system/networking.nix
    ../system/security.nix
    ../system/nvidia-drivers.nix
    ../system/power.nix
    ../system/hardware.nix
    ../system/settings.nix
    ../system/kernel.nix
    ../system/limits.nix
    ../services/desktop-session.nix
    ../programs/fish.nix
    ../programs/system-tools.nix
    ../system/packages.nix
    ../users/user.nix
  ];
}
