{ config, pkgs, lib, ... }:

let
  gpu = config.var.hardware.gpu or "amd";
in

{
  nixpkgs.config.allowUnfree = true;

  hardware.graphics = {
    enable = true;
    enable32Bit = true;
    extraPackages = with pkgs; [
      intel-media-driver
      intel-vaapi-driver
      libvdpau-va-gl
      mesa
    ];
  };

  services.xserver.videoDrivers = lib.mkDefault [ "amdgpu" "modesetting" ];

  boot.initrd.kernelModules =
    if gpu == "amd" then [ "amdgpu" ]
    else if gpu == "intel" then [ "i915" ]
    else [ ];

  hardware.cpu.intel.updateMicrocode = lib.mkDefault config.hardware.enableRedistributableFirmware;
  hardware.cpu.amd.updateMicrocode = lib.mkDefault config.hardware.enableRedistributableFirmware;

  hardware.bluetooth = {
    enable = true;
    powerOnBoot = true;
  };

  services.blueman.enable = true;

  swapDevices = [{
    device = "/var/lib/swapfile";
    size = 32 * 1024;
  }];
}
