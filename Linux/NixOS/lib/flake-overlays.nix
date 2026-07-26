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
      wahrweltPackages = (prev.wahrwelt or (prev.mysetup or { })) // flakePackages;
    in
    flakePackages
    // {
      wahrwelt = wahrweltPackages;
      mysetup = wahrweltPackages;
    };
in
rec {
  inherit flakePackagesOverlay;
  inherit (shellOverlays) valkeyNoCheckOverlay;

  omnirouterFromWahrweltOverlay = _final: prev: {
    omnirouter =
      inputs.wahrwelt.packages.${system}.omnirouter or (prev.callPackage ../pkgs/omnirouter.nix { });
  };

  omnirouterFromMySetupOverlay = omnirouterFromWahrweltOverlay;

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
