{ config, ... }:

{
  imports = [
    ./mysetup-options.nix
    ../profiles/base.nix
    ../profiles/desktop.nix
    ../profiles/developer.nix
    ../profiles/features.nix
  ];

  environment.sessionVariables = {
    NIXOS_OZONE_WL = "1";
    ELECTRON_OZONE_PLATFORM_HINT = "wayland";
  };

  system.stateVersion = config.mysetup.host.stateVersion;
}
