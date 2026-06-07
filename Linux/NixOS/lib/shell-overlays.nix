{ inputs }:

let
  caelestiaShellHyprLuaPatch = builtins.toFile "caelestia-shell-hyprlua-dispatch.patch" ''
    diff --git a/services/Hypr.qml b/services/Hypr.qml
    index a5ef2cd..5662859 100644
    --- a/services/Hypr.qml
    +++ b/services/Hypr.qml
    @@ -41,8 +41,80 @@ Singleton {

         signal configReloaded

    +    function luaString(value: string): string {
    +        return JSON.stringify(value ?? "");
    +    }
    +
    +    function luaWindow(value: string): string {
    +        const trimmed = (value ?? "").trim();
    +
    +        if (trimmed.startsWith("address:")) {
    +            return trimmed;
    +        }
    +
    +        if (trimmed.startsWith("0x")) {
    +            return `address:''${trimmed}`;
    +        }
    +
    +        return trimmed;
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
    +        if (dispatcher === "movetoworkspace") {
    +            const parts = args.split(",");
    +            const workspace = (parts[0] ?? "").trim();
    +            const window = luaWindow(parts.slice(1).join(",").trim());
    +
    +            if (window !== "") {
    +                Hyprland.dispatch(`hl.dsp.window.move({ workspace = ''${luaString(workspace)}, follow = false, window = ''${luaString(window)} })`);
    +                return;
    +            }
    +
    +            Hyprland.dispatch(`hl.dsp.window.move({ workspace = ''${luaString(workspace)} })`);
    +            return;
    +        }
    +
    +        if (dispatcher === "togglefloating") {
    +            const window = luaWindow(args);
    +
    +            if (window !== "") {
    +                Hyprland.dispatch(`hl.dsp.window.float({ action = "toggle", window = ''${luaString(window)} })`);
    +                return;
    +            }
    +
    +            Hyprland.dispatch('hl.dsp.window.float({ action = "toggle" })');
    +            return;
    +        }
    +
    +        if (dispatcher === "killwindow") {
    +            const window = luaWindow(args);
    +
    +            if (window !== "") {
    +                Hyprland.dispatch(`hl.dsp.window.close(''${luaString(window)})`);
    +                return;
    +            }
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
