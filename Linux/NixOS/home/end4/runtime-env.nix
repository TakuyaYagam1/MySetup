{
  config,
  homeLibs,
  lib,
  runtime,
}:

let
  qtRuntimeLib = homeLibs.qtRuntime;
  qtPlatformTheme = homeLibs.qtDefaults.platformTheme;
  qtRuntime = runtime.qtPackages;
  runtimeBinPath = lib.makeBinPath runtime.binPackages;
  systemBinPath = lib.concatStringsSep ":" [
    "/run/wrappers/bin"
    "/run/current-system/sw/bin"
    "/etc/profiles/per-user/${config.home.username}/bin"
    "${config.home.homeDirectory}/.nix-profile/bin"
    "/nix/var/nix/profiles/default/bin"
  ];
  sessionBinPath = "${runtimeBinPath}:${systemBinPath}:$PATH";
  qtPluginPath = qtRuntimeLib.mkSearchPath [
    "lib/qt-6/plugins"
    "lib/qt6/plugins"
    "lib/plugins"
  ] qtRuntime;
  qmlImportPath = qtRuntimeLib.mkSearchPath [
    "lib/qt-6/qml"
    "lib/qt6/qml"
  ] qtRuntime;

  xdgDataDirBase = [
    (lib.makeSearchPath "share" qtRuntime)
    "${config.home.homeDirectory}/.nix-profile/share"
    "${config.home.homeDirectory}/.local/share"
    "${config.home.homeDirectory}/.local/share/flatpak/exports/share"
    "/etc/profiles/per-user/${config.home.username}/share"
    "/run/current-system/sw/share"
    "/var/lib/flatpak/exports/share"
    "/usr/local/share"
    "/usr/share"
  ];

  shellXdgDataDirs = lib.concatStringsSep ":" (xdgDataDirBase ++ [ "$XDG_DATA_DIRS" ]);

  sessionVariables = {
    ILLOGICAL_IMPULSE_DOTFILES_SOURCE = "${config.home.homeDirectory}/.config";
    ILLOGICAL_IMPULSE_VIRTUAL_ENV = "${config.home.homeDirectory}/.local/state/quickshell/.venv";
    qsConfig = "${config.home.homeDirectory}/.config/quickshell/ii";
  };

  commonQtVars = {
    QT_PLUGIN_PATH = qtPluginPath;
    QML2_IMPORT_PATH = qmlImportPath;
    QT_WAYLAND_DISABLE_WINDOWDECORATION = "1";
    QT_QPA_PLATFORMTHEME = qtPlatformTheme;
  };

  shellVariables = commonQtVars // {
    PATH = sessionBinPath;
    XDG_DATA_DIRS = shellXdgDataDirs;
  };

  hyprVariables = commonQtVars // {
    PATH = sessionBinPath;
    inherit (sessionVariables) qsConfig;
    inherit (sessionVariables) ILLOGICAL_IMPULSE_VIRTUAL_ENV;
  };

  renderShellExport = name: value: ''
    export ${name}="${toString value}"
  '';

  renderHyprEnv = name: value: "env = ${name},${toString value}";
in
{
  inherit
    hyprVariables
    qmlImportPath
    qtPluginPath
    qtRuntime
    runtime
    runtimeBinPath
    sessionVariables
    shellVariables
    shellXdgDataDirs
    ;

  quickshellExports = lib.concatStringsSep "\n" (lib.mapAttrsToList renderShellExport shellVariables);

  hyprEnv = ''
    ${lib.concatStringsSep "\n" (lib.mapAttrsToList renderHyprEnv hyprVariables)}
    $qsConfig = ${config.home.homeDirectory}/.config/quickshell/ii
  '';
}
