let
  repoRoot = builtins.getEnv "WAHRWELT_NOCTALIA_TEST_REPO_ROOT";
  homeManagerWithoutBuiltinRef = builtins.getEnv "WAHRWELT_NOCTALIA_TEST_HM_WITHOUT_BUILTIN";
  homeManagerWithBuiltinRef = builtins.getEnv "WAHRWELT_NOCTALIA_TEST_HM_WITH_BUILTIN";
  source = builtins.getFlake ("path:" + repoRoot);
  homeManagerWithoutBuiltin = builtins.getFlake homeManagerWithoutBuiltinRef;
  homeManagerWithBuiltin = builtins.getFlake homeManagerWithBuiltinRef;
  system = "x86_64-linux";
  shellOverlays = import (repoRoot + "/Linux/NixOS/lib/shell-overlays.nix") {
    inputs = source.inputs;
  };
  pkgs = import source.inputs.nixpkgs {
    inherit system;
    overlays = [
      shellOverlays.shellPackagesOverlay
      shellOverlays.valkeyNoCheckOverlay
    ];
    config.allowUnfree = true;
  };
  inherit (pkgs) lib;
  wahrweltLib = import (repoRoot + "/Linux/NixOS/lib/mysetup.nix") {
    inherit lib;
  };
  baseWahrwelt = import (repoRoot + "/Linux/NixOS/hosts/NixOS/host-vars.nix");

  providerFor =
    providerHomeManager:
    (import (repoRoot + "/flake.nix")).outputs (
      source.inputs
      // {
        self = source;
        home-manager = providerHomeManager;
      }
    );

  configurationFor =
    providerHomeManager: consumerHomeManager: version:
    let
      provider = providerFor providerHomeManager;
      wahrwelt = baseWahrwelt // {
        user = {
          username = "alice";
          fullName = "Alice";
          homeDirectory = "/home/alice";
        };
        noctalia = baseWahrwelt.noctalia // {
          inherit version;
        };
      };
    in
    consumerHomeManager.lib.homeManagerConfiguration {
      inherit pkgs;
      modules = [ provider.lib.homeManagerModules.shells ];
      extraSpecialArgs = {
        inputs = source.inputs;
        inherit wahrwelt wahrweltLib;
        mysetup = wahrwelt;
        mysetupLib = wahrweltLib;
      };
    };

  checkOwner =
    providerHomeManager: consumerHomeManager: version:
    let
      configuration = configurationFor providerHomeManager consumerHomeManager version;
      programs = configuration.config.programs;
      activation = configuration.activationPackage.drvPath;
    in
    if version == "v4" then
      assert builtins.hasAttr "noctalia-shell" programs;
      assert programs.noctalia-shell.enable;
      assert !builtins.hasAttr "noctalia" programs;
      {
        inherit activation;
        owner = "programs.noctalia-shell";
      }
    else
      assert builtins.hasAttr "noctalia" programs;
      assert programs.noctalia.enable;
      assert !builtins.hasAttr "noctalia-shell" programs;
      {
        inherit activation;
        owner = "programs.noctalia";
      };
in
assert repoRoot != "";
assert homeManagerWithoutBuiltinRef != "";
assert homeManagerWithBuiltinRef != "";
{
  providerWithoutBuiltinConsumerWithBuiltin = {
    v4 = checkOwner homeManagerWithoutBuiltin homeManagerWithBuiltin "v4";
    v5 = checkOwner homeManagerWithoutBuiltin homeManagerWithBuiltin "v5";
  };
  providerWithBuiltinConsumerWithoutBuiltin = {
    v4 = checkOwner homeManagerWithBuiltin homeManagerWithoutBuiltin "v4";
    v5 = checkOwner homeManagerWithBuiltin homeManagerWithoutBuiltin "v5";
  };
}
