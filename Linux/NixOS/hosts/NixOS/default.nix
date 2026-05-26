{ lib, ... }:

{
  imports = [
    ../../modules/mysetup-stack.nix
  ]
  ++ lib.optional (builtins.pathExists ./hardware-configuration.nix) ./hardware-configuration.nix
  ++ lib.optional (builtins.pathExists ./hashed-password.nix) ./hashed-password.nix
  ++ lib.optional (builtins.pathExists ./secrets/secrets.yaml) ./secrets/sops.nix;

  mysetup = import ./host-vars.nix;
}
