{
  config,
  lib,
  pkgs,
  ...
}:

{
  config = lib.mkIf ((config.mysetup.hardware.gpu or "amd") == "nvidia") {
    services.xserver.videoDrivers = [ "nvidia" ];

    hardware.nvidia = {
      modesetting.enable = true;
      powerManagement.enable = false;
      powerManagement.finegrained = false;
      open = false;
      nvidiaSettings = true;
      package = config.boot.kernelPackages.nvidiaPackages.stable;
    };

    hardware.graphics.extraPackages = with pkgs; [
      egl-wayland
      libva
      libva-vdpau-driver
    ];

    boot = {
      kernelParams = [
        "nvidia-drm.modeset=1"
        "nvidia.NVreg_PreserveVideoMemoryAllocations=1"
      ];

      initrd.kernelModules = [
        "nvidia"
        "nvidia_modeset"
        "nvidia_uvm"
        "nvidia_drm"
      ];

      blacklistedKernelModules = [ "nouveau" ];

      extraModprobeConfig = ''
        options nvidia_drm modeset=1
        options nvidia_drm fbdev=1
      '';
    };

    environment.sessionVariables = {
      GBM_BACKEND = "nvidia-drm";
      __GLX_VENDOR_LIBRARY_NAME = "nvidia";
      WLR_NO_HARDWARE_CURSORS = "1";
      LIBVA_DRIVER_NAME = "nvidia";
      NVD_BACKEND = "direct";
      MOZ_ENABLE_WAYLAND = "1";
    };

    environment.systemPackages = with pkgs; [
      libva-utils
      mesa-demos
      vulkan-tools
    ];
  };
}
