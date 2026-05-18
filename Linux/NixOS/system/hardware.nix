{
  config,
  pkgs,
  lib,
  ...
}:

let
  gpu = config.mysetup.hardware.gpu or "amd";
in

{
  nixpkgs.config.allowUnfree = true;

  hardware = {
    graphics = {
      enable = true;
      enable32Bit = true;
      extraPackages = with pkgs; [
        intel-media-driver
        intel-vaapi-driver
        libvdpau-va-gl
        mesa
      ];
    };

    cpu.intel.updateMicrocode = lib.mkDefault config.hardware.enableRedistributableFirmware;
    cpu.amd.updateMicrocode = lib.mkDefault config.hardware.enableRedistributableFirmware;

    bluetooth = {
      enable = true;
      powerOnBoot = true;
    };
  };

  services.xserver.videoDrivers = lib.mkDefault [
    "amdgpu"
    "modesetting"
  ];

  boot.initrd.kernelModules =
    if gpu == "amd" then
      [ "amdgpu" ]
    else if gpu == "intel" then
      [ "i915" ]
    else
      [ ];

  services.blueman.enable = true;
  systemd.user.services.blueman-applet.serviceConfig.ExecStart = lib.mkForce [
    ""
    "${pkgs.blueman}/bin/blueman-applet"
  ];

  swapDevices = [
    {
      device = "/var/lib/swapfile";
      size = 32 * 1024;
    }
  ];
}
