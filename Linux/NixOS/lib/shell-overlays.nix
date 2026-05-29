{ inputs }:

let
  caelestiaShellHyprLuaPatch = builtins.toFile "caelestia-shell-hyprlua-dispatch.patch" ''
    diff --git a/services/Hypr.qml b/services/Hypr.qml
    index a5ef2cd..5662859 100644
    --- a/services/Hypr.qml
    +++ b/services/Hypr.qml
    @@ -41,8 +41,31 @@ Singleton {

         signal configReloaded

    +    function luaString(value: string): string {
    +        return JSON.stringify(value ?? "");
    +    }
    +
         function dispatch(request: string): void {
    -        Hyprland.dispatch(request);
    +        const match = request.match(/^(\S+)(?:\s+(.*))?$/);
    +        if (!match) {
    +            Hyprland.dispatch(request);
    +            return;
    +        }
    +
    +        const dispatcher = match[1];
    +        const args = match[2] ?? "";
    +
    +        if (dispatcher === "workspace") {
    +            Hyprland.dispatch(`hl.dsp.focus({ workspace = ''${luaString(args)} })`);
    +            return;
    +        }
    +
    +        if (dispatcher === "togglespecialworkspace") {
    +            Hyprland.dispatch(`hl.dsp.workspace.toggle_special(''${luaString(args)})`);
    +            return;
    +        }
    +
    +        Hyprland.dispatch(request);
         }

        function cycleSpecialWorkspace(direction: string): void {
  '';

  hyprReloadReplacement = builtins.toFile "caelestia-shell-hypr-reload.qml" ''
    const bindScript = 'hl.unbind("Caps_Lock"); '
        + 'hl.unbind("Num_Lock"); '
        + 'hl.bind("Caps_Lock", hl.dsp.global("caelestia:refreshDevices"), { locked = true, non_consuming = true }); '
        + 'hl.bind("Caps_Lock", hl.dsp.global("caelestia:refreshDevices"), { locked = true, non_consuming = true, release = true }); '
        + 'hl.bind("Num_Lock", hl.dsp.global("caelestia:refreshDevices"), { locked = true, non_consuming = true }); '
        + 'hl.bind("Num_Lock", hl.dsp.global("caelestia:refreshDevices"), { locked = true, non_consuming = true, release = true })';

    Quickshell.execDetached([
        "hyprctl",
        "eval",
        bindScript
    ]);
  '';

  hyprRefreshHelperReplacement = builtins.toFile "caelestia-shell-refresh-helper.qml" ''
    signal configReloaded

    property int refreshDevicesRetryCount: 0

    function refreshDevicesWithRetry(): void {
        extras.refreshDevices();
        refreshDevicesRetryCount = 0;
        refreshDevicesRetry.restart();
    }
  '';

  hyprRefreshHandlerReplacement = builtins.toFile "caelestia-shell-refresh-handler.qml" ''
        function refreshDevices(): void {
            root.refreshDevicesWithRetry();
        }
  '';

  hyprRefreshHandlerPattern = builtins.toFile "caelestia-shell-refresh-handler-pattern.qml" (
    "        function refreshDevices(): void {\n"
    + "            extras.refreshDevices();\n"
    + "        }\n"
  );

  hyprRefreshShortcutReplacement = builtins.toFile "caelestia-shell-refresh-shortcut.qml" ''
        onPressed: root.refreshDevicesWithRetry()
        onReleased: root.refreshDevicesWithRetry()
  '';

  hyprRefreshShortcutPattern = builtins.toFile "caelestia-shell-refresh-shortcut-pattern.qml" (
    "        onPressed: extras.refreshDevices()\n"
    + "        onReleased: extras.refreshDevices()\n"
  );

  hyprRefreshTimerReplacement = builtins.toFile "caelestia-shell-refresh-timer.qml" ''
    Timer {
        id: refreshDevicesRetry
        interval: 80
        repeat: true
        onTriggered: {
            extras.refreshDevices();
            root.refreshDevicesRetryCount += 1;

            if (root.refreshDevicesRetryCount >= 3) {
                stop();
                root.refreshDevicesRetryCount = 0;
            }
        }
    }

    FileView {
  '';

  patchCaelestiaShell =
    package:
    package.overrideAttrs (oldAttrs: {
      patches = (oldAttrs.patches or [ ]) ++ [ caelestiaShellHyprLuaPatch ];
      postPatch = (oldAttrs.postPatch or "") + ''
        substituteInPlace services/Hypr.qml \
          --replace-fail \
            'extras.batchMessage(["keyword bindlni ,Caps_Lock,global,caelestia:refreshDevices", "keyword bindlni ,Num_Lock,global,caelestia:refreshDevices"]);' \
            "$(cat ${hyprReloadReplacement})"

        substituteInPlace services/Hypr.qml \
          --replace-fail \
            'signal configReloaded' \
            "$(cat ${hyprRefreshHelperReplacement})"

        substituteInPlace services/Hypr.qml \
          --replace-fail \
            "$(cat ${hyprRefreshHandlerPattern})" \
            "$(cat ${hyprRefreshHandlerReplacement})"

        substituteInPlace services/Hypr.qml \
          --replace-fail \
            "$(cat ${hyprRefreshShortcutPattern})" \
            "$(cat ${hyprRefreshShortcutReplacement})"

        substituteInPlace services/Hypr.qml \
          --replace-fail \
            '    FileView {' \
            "$(cat ${hyprRefreshTimerReplacement})"

        substituteInPlace modules/bar/popouts/Content.qml \
          --replace-fail \
            'sourceComponent: trayMenuComp' \
            'sourceComponent: trayMenu.modelData?.menu ? trayMenuComp : null'

        substituteInPlace modules/bar/popouts/Content.qml \
          --replace-fail \
            'if (root.popouts.hasCurrent && trayMenu.shouldBeActive) {' \
            'if (root.popouts.hasCurrent && trayMenu.shouldBeActive && trayMenu.modelData?.menu) {'

      '';
    });

  shellPackagesFor =
    {
      prev,
      system ? prev.stdenv.hostPlatform.system,
    }:
    {
      caelestia-cli = inputs.caelestia-cli.packages.${system}.default;
      caelestia-shell = patchCaelestiaShell inputs.caelestia-shell.packages.${system}.with-cli;
      noctalia-shell = inputs.noctalia-shell.packages.${system}.default;
      quickshell = inputs.quickshell.packages.${system}.default.override {
        inherit (prev) libxcb;
      };
    };

  shellPackagesOverlay =
    _final: prev:
    let
      shellPackages = shellPackagesFor { inherit prev; };
    in
    shellPackages
    // {
      mysetup = (prev.mysetup or { }) // shellPackages;
    };

  valkeyNoCheckOverlay = _final: prev: {
    valkey = prev.valkey.overrideAttrs (_: {
      # Noctalia pulls Valkey through its graph; upstream checks are flaky on this pinned nixpkgs.
      doCheck = false;
    });
  };
in
{
  inherit
    patchCaelestiaShell
    shellPackagesFor
    shellPackagesOverlay
    valkeyNoCheckOverlay
    ;
}
