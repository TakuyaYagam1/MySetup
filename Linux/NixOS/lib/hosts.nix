{
  mkWahrweltHost,
  system,
}:

let
  hostVarsPath = ../hosts/NixOS/host-vars.nix;
  hostVars = import hostVarsPath;
  hostname = if hostVars ? host then hostVars.host.hostname else hostVars.hostname;
  hardwarePath = ../hosts/NixOS/hardware-configuration.nix;
  passwordHashMarker = ../.wahrwelt-password-hash-enabled;
  hasHardware = builtins.pathExists hardwarePath;
  ciHardwareFallback = _: {
    fileSystems."/" = {
      device = "none";
      fsType = "tmpfs";
    };
  };
in
{
  nixosConfigurations.${hostname} = mkWahrweltHost {
    inherit system;
    inherit hostname;
    hostVars = hostVarsPath;
    hardware = if hasHardware then hardwarePath else null;
    hashedPasswordFile =
      if builtins.pathExists passwordHashMarker then "/etc/wahrwelt/hashed-password" else null;
    secretsDir = ../hosts/NixOS/secrets;
    extraModules = if hasHardware then [ ] else [ ciHardwareFallback ];
  };
}
