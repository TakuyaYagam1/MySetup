{ config, lib, ... }:

{
  config = lib.mkMerge [
    {
      home.sessionVariables = {
        QT_STYLE_OVERRIDE = lib.mkForce "";
        ILLOGICAL_IMPULSE_DOTFILES_SOURCE = "${config.home.homeDirectory}/.config";
        ILLOGICAL_IMPULSE_VIRTUAL_ENV = "${config.home.homeDirectory}/.local/state/quickshell/.venv";
        qsConfig = "${config.home.homeDirectory}/.config/quickshell/ii";
      };
    }
    {
      systemd.user.sessionVariables = config.home.sessionVariables;
    }
  ];
}
