{
  mkMySetupHost,
  system,
}:

let
  hostVarsPath = ../hosts/NixOS/host-vars.nix;
  hostVars = import hostVarsPath;
  hostname = if hostVars ? host then hostVars.host.hostname else hostVars.hostname;
  hardware = ../hosts/NixOS/hardware-configuration.nix;
  hashedPassword = ../hosts/NixOS/hashed-password.nix;
in
if builtins.pathExists hardware then
  {
    nixosConfigurations.${hostname} = mkMySetupHost {
      inherit system;
      inherit hostname;
      hostVars = hostVarsPath;
      inherit hardware;
      hashedPassword = if builtins.pathExists hashedPassword then hashedPassword else null;
      secretsDir = ../hosts/NixOS/secrets;
    };
  }
else
  { }
