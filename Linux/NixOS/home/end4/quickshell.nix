{ end4Lib, pkgs, ... }:

let
  mysetupPkgs = pkgs.mysetup or { };
  qsPackage = mysetupPkgs.quickshell or pkgs.quickshell;
in
{
  home.packages = [
    (pkgs.writeShellScriptBin "qs-end4" ''
      runtime_env="$HOME/.config/hypr/scripts/end4-runtime-env.sh"
      if [ -r "$runtime_env" ]; then
        . "$runtime_env"
      else
        ${end4Lib.runtimeEnv.quickshellExports}
      fi
      exec ${qsPackage}/bin/qs "$@"
    '')
  ];
}
