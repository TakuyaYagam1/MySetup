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
    inherit
      lib
      pkgs
      pkgs-stable
      inputs
      ;
  };
in
{
  imports = [
    ../services/omnirouter.nix
    ../services/portainer.nix
    ../services/observability.nix
  ];

  config = lib.mkMerge [
    {
      assertions = [
        {
          assertion = !config.wahrwelt.features.firefoxLegacy || config.wahrwelt.features.ctfTools;
          message = "wahrwelt.features.firefoxLegacy requires the explicit wahrwelt.features.ctfTools lab feature";
        }
      ];
    }
    (lib.mkIf config.wahrwelt.features.ctfTools {
      environment.systemPackages = lib.flatten (lib.attrValues packageSets.ctf);
      programs.wireshark.enable = true;
    })
  ];
}
