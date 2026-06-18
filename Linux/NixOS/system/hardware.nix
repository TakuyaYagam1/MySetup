{
  config,
  pkgs,
  lib,
  ...
}:

let
  gpu = config.mysetup.hardware.gpu or "amd";
  nixCfg = config.mysetup.nix;
  commonGraphicsPackages = with pkgs; [
    libvdpau-va-gl
    mesa
    vulkan-loader
    vulkan-validation-layers
  ];
  amdGraphicsPackages = with pkgs; [
    libva-vdpau-driver
    rocmPackages.clr.icd
  ];
  intelGraphicsPackages = with pkgs; [
    intel-media-driver
    intel-vaapi-driver
  ];
in

{
  nixpkgs.config.allowUnfree = true;

  hardware = {
    graphics = {
      enable = true;
      enable32Bit = true;
      extraPackages =
        commonGraphicsPackages
        ++ lib.optionals (gpu == "amd") amdGraphicsPackages
        ++ lib.optionals (gpu == "intel") intelGraphicsPackages;
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

  zramSwap = {
    enable = nixCfg.zram.enable;
    algorithm = "zstd";
    memoryPercent = nixCfg.zram.memoryPercent;
    priority = 100;
  };

  swapDevices = lib.optionals (nixCfg.swapSizeMiB != null) [
    {
      device = "/var/lib/swapfile";
      size = nixCfg.swapSizeMiB;
      priority = 10;
    }
  ];
}
