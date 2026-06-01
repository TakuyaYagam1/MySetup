{
  mkMySetupHost,
  system,
}:

let
  hostVarsPath = ../hosts/NixOS/host-vars.nix;
  hostVars = import hostVarsPath;
  hostname = if hostVars ? host then hostVars.host.hostname else hostVars.hostname;
  hardwarePath = ../hosts/NixOS/hardware-configuration.nix;
  hashedPasswordPath = ../hosts/NixOS/hashed-password.nix;
  hasHardware = builtins.pathExists hardwarePath;
  ciHardwareFallback = _: {
    fileSystems."/" = {
      device = "none";
      fsType = "tmpfs";
    };
  };
in
{
  nixosConfigurations.${hostname} = mkMySetupHost {
    inherit system;
    inherit hostname;
    hostVars = hostVarsPath;
    hardware = if hasHardware then hardwarePath else null;
    hashedPassword = if builtins.pathExists hashedPasswordPath then hashedPasswordPath else null;
    secretsDir = ../hosts/NixOS/secrets;
    extraModules = if hasHardware then [ ] else [ ciHardwareFallback ];
  };
}
