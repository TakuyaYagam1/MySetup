{ config, lib, ... }:

{
  imports = [
    ./variables.nix
    ../../profiles/base.nix
    ../../profiles/personal.nix
    ../../profiles/features.nix
  ]
  ++ lib.optional (builtins.pathExists ./hardware-configuration.nix) ./hardware-configuration.nix
  ++ lib.optional (builtins.pathExists ./hashed-password.nix) ./hashed-password.nix
  ++ lib.optional (builtins.pathExists ./secrets/secrets.yaml) ./secrets/sops.nix;

  environment.sessionVariables = {
    NIXOS_OZONE_WL = "1";
    ELECTRON_OZONE_PLATFORM_HINT = "wayland";
  };

  system.stateVersion = config.mysetup.host.stateVersion;
}
