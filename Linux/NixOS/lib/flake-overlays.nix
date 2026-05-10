{
  inputs,
  pkgs-bleeding,
  system,
}:

let
  flakePackagesOverlay =
    _final: prev:
    let
      flakePackages = {
        caelestia-cli = inputs.caelestia-cli.packages.${system}.default;
        caelestia-shell = inputs.caelestia-shell.packages.${system}.default;
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
