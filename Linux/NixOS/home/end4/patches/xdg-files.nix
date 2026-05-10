{
  end4Lib,
  inputs,
  lib,
  pkgs,
  ...
}:

let
  dotfilesSource = inputs.end4-dotfiles;
  inherit (end4Lib) dotfilesLib pythonEnv;

  end4KvantumThemes = pkgs.runCommand "end4-kvantum-themes" { } ''
    mkdir -p "$out/share/Kvantum"
    cp -r ${dotfilesSource}/dots/.config/Kvantum/Colloid "$out/share/Kvantum/"
    cp -r ${dotfilesSource}/dots/.config/Kvantum/MaterialAdw "$out/share/Kvantum/"
  '';
in
{
  # Stylix/Home Manager owns ~/.config/Kvantum globally. Add end4's bundled
  # themes through qt.kvantum.themes instead of replacing that whole directory.
  qt.kvantum.themes = lib.mkAfter [ end4KvantumThemes ];

  home.file = {
    ".local/state/quickshell/.venv/bin/activate" = {
      force = true;
      text = ''
        export VIRTUAL_ENV="${pythonEnv}"
        export PATH="${pythonEnv}/bin:$PATH"
        deactivate () {
          return 0
        }
      '';
    };

    ".local/state/quickshell/.venv/bin/python" = {
      force = true;
      source = "${pythonEnv}/bin/python";
    };

    ".local/state/quickshell/.venv/bin/python3" = {
      force = true;
      source = "${pythonEnv}/bin/python3";
    };

    ".local/state/quickshell/.venv/pyvenv.cfg" = {
      force = true;
      text = ''
        home = ${pythonEnv}/bin
        include-system-site-packages = false
        version = ${pkgs.python3.pythonVersion}
      '';
    };
  };

  xdg.configFile = {
    "chrome-flags.conf" = dotfilesLib.forcedSource "${dotfilesSource}/dots/.config/chrome-flags.conf";
    "code-flags.conf" = dotfilesLib.forcedSource "${dotfilesSource}/dots/.config/code-flags.conf";
    "darklyrc" = dotfilesLib.forcedSource "${dotfilesSource}/dots/.config/darklyrc";
    "dolphinrc" = dotfilesLib.forcedSource "${dotfilesSource}/dots/.config/dolphinrc";
    "kde-material-you-colors" =
      dotfilesLib.forcedSource "${dotfilesSource}/dots/.config/kde-material-you-colors";
    "matugen" = dotfilesLib.forcedSource "${dotfilesSource}/dots/.config/matugen";
    "mpv" = dotfilesLib.forcedSource "${dotfilesSource}/dots/.config/mpv";
    "wlogout" = dotfilesLib.forcedSource "${dotfilesSource}/dots/.config/wlogout";
    "xdg-desktop-portal" = dotfilesLib.forcedSource "${dotfilesSource}/dots/.config/xdg-desktop-portal";
  };

  xdg.dataFile = {
    "icons/hicolor/scalable/apps/illogical-impulse.svg" =
      dotfilesLib.forcedSource "${dotfilesSource}/dots/.local/share/icons/illogical-impulse.svg";
  };
}
