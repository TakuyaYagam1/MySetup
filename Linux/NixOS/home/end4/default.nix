{
  config,
  lib,
  mysetup,
  pkgs,
  ...
}:

{
  _module.args.end4Lib = import ./lib.nix {
    inherit config lib pkgs;
    mysetupConfig = mysetup;
  };

  imports = [
    ./seed
    ./packages.nix
    ./environment.nix
    ./quickshell.nix
    ./patches
  ];
}
