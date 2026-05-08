{ config, lib, ... }:

{
  home.sessionVariables = {
    QT_STYLE_OVERRIDE = lib.mkForce "";
    ILLOGICAL_IMPULSE_DOTFILES_SOURCE = "${config.home.homeDirectory}/.config";
    ILLOGICAL_IMPULSE_VIRTUAL_ENV = "${config.home.homeDirectory}/.local/state/quickshell/.venv";
    qsConfig = "${config.home.homeDirectory}/.config/quickshell/ii";
  };

  # HM's Qt module already manages most systemd session env on its own.
  # Mirror only end4-specific runtime vars here so we do not reintroduce
  # option conflicts by copying the whole home.sessionVariables attrset.
  systemd.user.sessionVariables = {
    QT_STYLE_OVERRIDE = lib.mkForce "";
    ILLOGICAL_IMPULSE_DOTFILES_SOURCE = "${config.home.homeDirectory}/.config";
    ILLOGICAL_IMPULSE_VIRTUAL_ENV = "${config.home.homeDirectory}/.local/state/quickshell/.venv";
    qsConfig = "${config.home.homeDirectory}/.config/quickshell/ii";
  };
}
