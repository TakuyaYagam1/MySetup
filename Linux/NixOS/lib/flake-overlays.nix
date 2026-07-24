{
  inputs,
  system,
}:

let
  shellOverlays = import ./shell-overlays.nix { inherit inputs; };

  flakePackagesOverlay =
    _final: prev:
    let
      shellPackages = shellOverlays.shellPackagesFor { inherit prev system; };
      flakePackages = shellPackages // {
        claude-code = inputs.claude-code.packages.${system}.default;
        codex = inputs.codex.packages.${system}.default;
        neovim = inputs.neovim-nightly-overlay.packages.${system}.default;
        zen-browser = inputs.zen-browser.packages.${system}.default;
        burpsuitepro = prev.callPackage ../pkgs/burpsuitepro.nix { };
        firefox-legacy = prev.callPackage ../pkgs/firefox-legacy.nix { };
      };
    in
    flakePackages
    // {
      mysetup = (prev.mysetup or { }) // flakePackages;
    };
in
{
  inherit flakePackagesOverlay;
  inherit (shellOverlays) valkeyNoCheckOverlay;

  omnirouterFromMySetupOverlay = _final: prev: {
    omnirouter =
      inputs.mysetup.packages.${system}.omnirouter or (prev.callPackage ../pkgs/omnirouter.nix { });
  };

  pipxTestCompatibilityOverlay = _final: prev: {
    pipx = prev.pipx.overridePythonAttrs (old: {
      disabledTests = (old.disabledTests or [ ]) ++ [
        "test_fix_package_name"
        "test_parse_specifier_for_metadata"
      ];
      disabledTestPaths = (old.disabledTestPaths or [ ]) ++ [
        "tests/test_inject.py"
      ];
    });
  };
}
