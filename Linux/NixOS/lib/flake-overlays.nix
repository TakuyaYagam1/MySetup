{
  inputs,
  pkgs-bleeding,
  system,
}:

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
    });

  flakePackagesOverlay =
    _final: prev:
    let
      flakePackages = {
        caelestia-cli = inputs.caelestia-cli.packages.${system}.default;
        caelestia-shell = patchCaelestiaShell inputs.caelestia-shell.packages.${system}.with-cli;
        claude-code = inputs.claude-code.packages.${system}.default;
        codex = inputs.codex.packages.${system}.default;
        neovim = inputs.neovim-nightly-overlay.packages.${system}.default;
        quickshell = inputs.quickshell.packages.${system}.default.override {
          inherit (prev) libxcb;
        };
        inherit (inputs.templ.packages.${system}) templ;
        zen-browser = inputs.zen-browser.packages.${system}.default;
      };
    in
    flakePackages
    // {
      mysetup = (prev.mysetup or { }) // flakePackages;
    };
in
{
  inherit flakePackagesOverlay;

  qtBleedingOverlay = _final: _prev: {
    inherit (pkgs-bleeding) qt6;
    inherit (pkgs-bleeding) qt6Packages;
    inherit (pkgs-bleeding) kdePackages;
  };

  valkeyNoCheckOverlay = _final: prev: {
    valkey = prev.valkey.overrideAttrs (_: {
      # Noctalia pulls Valkey through its graph; upstream checks are flaky on this pinned nixpkgs.
      doCheck = false;
    });
  };
}
