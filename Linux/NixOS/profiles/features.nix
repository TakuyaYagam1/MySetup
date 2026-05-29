{
  config,
  lib,
  pkgs,
  pkgs-stable,
  inputs,
  ...
}:

let
  packageSets = import ../lib/package-sets.nix {
    inherit lib pkgs pkgs-stable inputs;
  };
in
{
  imports = [
    ../services/zapret.nix
    ../services/omnirouter.nix
    ../services/observability.nix
    ../packages/ghidra-mcp.nix
    ../packages/burp-mcp.nix
  ];

  config = lib.mkIf config.mysetup.features.ctfTools {
    environment.systemPackages = lib.flatten (lib.attrValues packageSets.ctf);
  };
}
