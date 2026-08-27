{
  config,
  lib,
  wahrwelt,
  pkgs,
  ...
}:

{
  _module.args.end4Lib = import ./lib.nix {
    inherit config lib pkgs;
    wahrweltConfig = wahrwelt;
  };

  imports = [
    ../migrations/v1_to_v2/end4-app-seed.nix
    ./seed
    ./packages.nix
    ./environment.nix
    ./quickshell.nix
    ./patches
  ];
}
