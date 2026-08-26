{ end4Lib, ... }:

{
  xdg.configFile."hypr/scripts/end4-runtime-env.sh" = {
    force = true;
    text = ''
      # shellcheck shell=bash
      ${end4Lib.runtimeEnv.quickshellExports}
    '';
  };
}
