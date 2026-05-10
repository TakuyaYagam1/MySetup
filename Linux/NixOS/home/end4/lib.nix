{
  config ? null,
  lib,
  mysetupConfig ? import ../../hosts/NixOS/host-vars.nix,
  pkgs,
}:

let
  homeLibs = import ../lib { inherit lib pkgs; };
  dotfilesLib = homeLibs.dotfiles;
  settings = import ./settings.nix {
    inherit mysetupConfig;
    transparencyDefaults = homeLibs.transparency.perShell.end4;
  };
  runtime = (import ../../lib/package-sets.nix { inherit lib pkgs; }).runtime.end4;
  runtimeEnv =
    if config == null then
      null
    else
      import ./runtime-env.nix {
        inherit
          config
          homeLibs
          lib
          runtime
          ;
      };
  pythonEnv = import ./python-env.nix { inherit pkgs; };
in
{
  inherit
    dotfilesLib
    homeLibs
    pythonEnv
    runtime
    runtimeEnv
    settings
    ;
}
