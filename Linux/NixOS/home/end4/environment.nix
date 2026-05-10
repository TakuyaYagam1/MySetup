{ end4Lib, ... }:

{
  home.sessionVariables = end4Lib.runtimeEnv.sessionVariables;

  systemd.user.sessionVariables = end4Lib.runtimeEnv.sessionVariables;

  xdg.configFile."hypr/scripts/end4-runtime-env.sh" = {
    force = true;
    text = ''
      # shellcheck shell=bash
      ${end4Lib.runtimeEnv.quickshellExports}
    '';
  };
}
