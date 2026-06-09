{ inputs, pkgs }:

let
  inherit (pkgs.stdenv.hostPlatform) system;
in
inputs.noctalia-shell.packages.${system}.default.overrideAttrs (oldAttrs: {
  postPatch = (oldAttrs.postPatch or "") + ''
    if [ -f Modules/Tooltip/Tooltip.qml ]; then
      # Tooltip: clamp width in grid mode so multi-monitor tooltips stay on-screen.
      grep -q '^  property int maxWidth: 340$' Modules/Tooltip/Tooltip.qml
      sed -i '/^  property int maxWidth: 340$/a\  readonly property int effectiveMaxWidth: isGridMode ? Math.max(1, screenWidth - (margin * 2)) : maxWidth' Modules/Tooltip/Tooltip.qml
      substituteInPlace Modules/Tooltip/Tooltip.qml \
        --replace-fail 'Math.ceil(Math.min(contentWidth + ((padding + extraPad) * 2), maxWidth))' 'Math.ceil(Math.min(contentWidth + ((padding + extraPad) * 2), effectiveMaxWidth))'
    fi

    if [ -f Modules/LockScreen/LockScreenPanel.qml ]; then
      # LockScreen: derive panel/button dimensions from Style.uiScaleRatio so HiDPI
      # displays render the password field, session buttons, and panel at usable size.
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
    fi
  '';
})
