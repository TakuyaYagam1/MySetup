{ config, inputs, lib, pkgs, ... }:

let
  system = pkgs.stdenv.hostPlatform.system;
  noctaliaShellPackage = inputs.noctalia-shell.packages.${system}.default.overrideAttrs (oldAttrs: {
    postPatch = (oldAttrs.postPatch or "") + ''
      grep -q '^  property int maxWidth: 340$' Modules/Tooltip/Tooltip.qml
      sed -i '/^  property int maxWidth: 340$/a\  readonly property int effectiveMaxWidth: isGridMode ? Math.max(1, screenWidth - (margin * 2)) : maxWidth' Modules/Tooltip/Tooltip.qml
      substituteInPlace Modules/Tooltip/Tooltip.qml \
        --replace-fail 'Math.ceil(Math.min(contentWidth + ((padding + extraPad) * 2), maxWidth))' 'Math.ceil(Math.min(contentWidth + ((padding + extraPad) * 2), effectiveMaxWidth))'

      grep -q '^  readonly property bool weatherReady: Settings.data.location.weatherEnabled && (LocationService.data.weather !== null)$' Modules/LockScreen/LockScreenPanel.qml
      substituteInPlace Modules/LockScreen/LockScreenPanel.qml \
        --replace-fail '  readonly property bool weatherReady: Settings.data.location.weatherEnabled && (LocationService.data.weather !== null)' '  readonly property bool weatherReady: Settings.data.location.weatherEnabled && (LocationService.data.weather !== null)
  readonly property real lockPanelScale: Math.max(1, Style.uiScaleRatio)
  readonly property int lockPanelBaseWidth: Settings.data.general.showHibernateOnLockScreen ? 860 : 810
  readonly property int lockPanelWidth: Math.min(Math.max(1, root.width - Math.round(48 * lockPanelScale)), Math.round(lockPanelBaseWidth * Math.min(lockPanelScale, 1.12)))
  readonly property int lockTopRowHeight: Math.round(78 * lockPanelScale)
  readonly property int lockPasswordHeight: Math.round((Settings.data.general.compactLockScreen ? 42 : 48) * lockPanelScale)
  readonly property int lockSessionButtonHeight: Math.round((Settings.data.general.compactLockScreen ? 36 : 48) * lockPanelScale)' \
        --replace-fail '      let calcHeight = Settings.data.general.compactLockScreen ? 120 : 220;' '      let calcHeight = Settings.data.general.compactLockScreen ? Math.round(120 * root.lockPanelScale) : Math.round(248 * root.lockPanelScale);' \
        --replace-fail '    width: Settings.data.general.showHibernateOnLockScreen ? 860 : 810' '    width: root.lockPanelWidth' \
        --replace-fail '        Layout.preferredHeight: 65' '        Layout.preferredHeight: root.lockTopRowHeight' \
        --replace-fail '          Layout.preferredHeight: 48' '          Layout.preferredHeight: root.lockPasswordHeight' \
        --replace-fail '        Layout.preferredHeight: Settings.data.general.compactLockScreen ? 36 : 48' '        Layout.preferredHeight: root.lockSessionButtonHeight'
    '';
  });
  settingsJson = ./config/settings.json;
  colorsJson = ./config/colors.json;
  pluginsJson = ./config/plugins.json;
  colorSchemesDir = ./config/colorschemes;
  wallpapersJson = pkgs.writeText "noctalia-wallpapers.json" (builtins.toJSON {
    wallpapers = { };
    defaultWallpaper = "@HOME@/Pictures/Wallpapers/1.jpg";
    usedRandomWallpapers = { };
  });

  colorSchemeFiles =
    if builtins.pathExists colorSchemesDir
    then lib.filesystem.listFilesRecursive colorSchemesDir
    else [ ];

  seedColorSchemeFiles = lib.concatMapStringsSep "\n" (file:
    let
      relPath = lib.removePrefix "${toString colorSchemesDir}/" (toString file);
      relDir = builtins.dirOf relPath;
      storeFile = builtins.path {
        path = file;
        name = lib.replaceStrings [ " " ] [ "-" ] (builtins.baseNameOf file);
      };
      targetDirExpr =
        if relDir == "."
        then "$targetDir/colorschemes"
        else "$targetDir/colorschemes/${relDir}";
      targetExpr = "$targetDir/colorschemes/${relPath}";
    in
    ''
      $DRY_RUN_CMD ${pkgs.coreutils}/bin/mkdir -p "${targetDirExpr}"
      if [ ! -e "${targetExpr}" ]; then
        $DRY_RUN_CMD ${pkgs.coreutils}/bin/install -m 644 "${storeFile}" "${targetExpr}"
      fi
    '') colorSchemeFiles;
in
{
  programs.noctalia-shell = {
    enable = true;
    package = noctaliaShellPackage;
    systemd.enable = false;
    settings = lib.mkForce { };
    colors = lib.mkForce { };
  };

  # The upstream Stylix target writes this as a Nix store symlink, but Noctalia
  # uses it as mutable runtime state for selected wallpapers.
  home.file.".cache/noctalia/wallpapers.json".enable = lib.mkForce false;

  home.activation.noctaliaSeedConfig =
    lib.hm.dag.entryAfter [ "writeBoundary" ] ''
      targetDir="$HOME/.config/noctalia"
      cacheDir="$HOME/.cache/noctalia"
      wallpaperCache="$cacheDir/wallpapers.json"
      $DRY_RUN_CMD ${pkgs.coreutils}/bin/mkdir -p "$targetDir"
      $DRY_RUN_CMD ${pkgs.coreutils}/bin/mkdir -p "$cacheDir"

      if [ -L "$wallpaperCache" ]; then
        resolved="$(${pkgs.coreutils}/bin/readlink -f "$wallpaperCache" 2>/dev/null || true)"
        case "$resolved" in
          /nix/store/*)
            $DRY_RUN_CMD ${pkgs.coreutils}/bin/rm -f "$wallpaperCache"
            ;;
        esac
      fi

      if [ ! -e "$wallpaperCache" ]; then
        $DRY_RUN_CMD ${pkgs.coreutils}/bin/install -m 644 "${wallpapersJson}" "$wallpaperCache"
        $DRY_RUN_CMD ${pkgs.gnused}/bin/sed -i "s|@HOME@|${config.home.homeDirectory}|g" "$wallpaperCache"
      fi

      if [ -L "$targetDir/settings.json" ]; then
        resolved="$(${pkgs.coreutils}/bin/readlink -f "$targetDir/settings.json" 2>/dev/null || true)"
        case "$resolved" in
          /nix/store/*)
            $DRY_RUN_CMD ${pkgs.coreutils}/bin/rm -f "$targetDir/settings.json"
            ;;
        esac
      fi

      if [ ! -e "$targetDir/settings.json" ]; then
        $DRY_RUN_CMD ${pkgs.coreutils}/bin/install -m 644 "${settingsJson}" "$targetDir/settings.json"
        $DRY_RUN_CMD ${pkgs.gnused}/bin/sed -i "s|@HOME@|${config.home.homeDirectory}|g" "$targetDir/settings.json"
      fi

      if [ -L "$targetDir/colors.json" ]; then
        resolved="$(${pkgs.coreutils}/bin/readlink -f "$targetDir/colors.json" 2>/dev/null || true)"
        case "$resolved" in
          /nix/store/*)
            $DRY_RUN_CMD ${pkgs.coreutils}/bin/rm -f "$targetDir/colors.json"
            ;;
        esac
      fi

      if [ ! -e "$targetDir/colors.json" ]; then
        $DRY_RUN_CMD ${pkgs.coreutils}/bin/install -m 644 "${colorsJson}" "$targetDir/colors.json"
      fi

      if [ -L "$targetDir/plugins.json" ]; then
        resolved="$(${pkgs.coreutils}/bin/readlink -f "$targetDir/plugins.json" 2>/dev/null || true)"
        case "$resolved" in
          /nix/store/*)
            $DRY_RUN_CMD ${pkgs.coreutils}/bin/rm -f "$targetDir/plugins.json"
            ;;
        esac
      fi

      if [ ! -e "$targetDir/plugins.json" ]; then
        $DRY_RUN_CMD ${pkgs.coreutils}/bin/install -m 644 "${pluginsJson}" "$targetDir/plugins.json"
      fi

      ${seedColorSchemeFiles}
    '';
}
