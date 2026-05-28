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

  patchCaelestiaShell =
    package:
    package.overrideAttrs (oldAttrs: {
      patches = (oldAttrs.patches or [ ]) ++ [ caelestiaShellHyprLuaPatch ];
      postPatch = (oldAttrs.postPatch or "") + ''
        hypr_reload_replacement="$(cat <<'EOF'
        const bindScript = 'hl.unbind("Caps_Lock"); '
            + 'hl.unbind("Num_Lock"); '
            + 'hl.bind("Caps_Lock", hl.dsp.global("caelestia:refreshDevices"), { locked = true, non_consuming = true }); '
            + 'hl.bind("Num_Lock", hl.dsp.global("caelestia:refreshDevices"), { locked = true, non_consuming = true })';

        Quickshell.execDetached([
            "hyprctl",
            "eval",
            bindScript
        ]);
        EOF
        )"

        substituteInPlace services/Hypr.qml \
          --replace-fail \
            'extras.batchMessage(["keyword bindlni ,Caps_Lock,global,caelestia:refreshDevices", "keyword bindlni ,Num_Lock,global,caelestia:refreshDevices"]);' \
            "$hypr_reload_replacement"

        substituteInPlace modules/bar/popouts/Content.qml \
          --replace-fail \
            'sourceComponent: trayMenuComp' \
            'sourceComponent: trayMenu.modelData?.menu ? trayMenuComp : null'

        substituteInPlace modules/bar/popouts/Content.qml \
          --replace-fail \
            'if (root.popouts.hasCurrent && trayMenu.shouldBeActive) {' \
            'if (root.popouts.hasCurrent && trayMenu.shouldBeActive && trayMenu.modelData?.menu) {'

        substituteInPlace modules/bar/popouts/TrayMenu.qml \
          --replace-fail \
            'model: menuOpener.children' \
            'model: menuOpener.children.filter(child => !!child)'
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
