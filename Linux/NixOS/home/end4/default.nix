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
    ./seed
    ./packages.nix
    ./environment.nix
    ./quickshell.nix
    ./patches
  ];
}
