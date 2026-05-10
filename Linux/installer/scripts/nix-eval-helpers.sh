#!/usr/bin/env bash
# Shared helpers used by the installer Makefile to invoke `nix eval` against
# the flake. NIXOS_DIR and the preset override are passed through environment
# variables consumed inside the Nix expression via `builtins.getEnv`, so the
# values never get interpolated into the expression source.

set -euo pipefail

if [[ -z "${NIXOS_DIR:-}" ]]; then
    echo "nix-eval-helpers: NIXOS_DIR must be set" >&2
    exit 64
fi
if [[ -z "${NIX_CACHE_SUBSTITUTERS:-}" || -z "${NIX_CACHE_KEYS:-}" ]]; then
    echo "nix-eval-helpers: NIX_CACHE_SUBSTITUTERS and NIX_CACHE_KEYS must be set" >&2
    exit 64
fi

_NIX_CACHE_ARGS=(
    --option extra-substituters "${NIX_CACHE_SUBSTITUTERS}"
    --option extra-trusted-public-keys "${NIX_CACHE_KEYS}"
)

_nix_eval() {
    nix eval "${_NIX_CACHE_ARGS[@]}" --no-write-lock-file --impure --raw --expr "$1" >/dev/null
}

# eval_hm_simple <attribute>
# Evaluates a Home Manager configuration that imports only the shared shells
# module (no mysetup specialArgs). Useful for sanity-checking activation
# scripts and the runtime entrypoint files.
eval_hm_simple() {
    local attr="$1"
    _nix_eval "
        let
          nixosDir = builtins.getEnv \"MYSETUP_NIXOS_DIR\";
          flake = builtins.getFlake (\"path:\" + nixosDir);
          system = \"x86_64-linux\";
          pkgs = import flake.inputs.nixpkgs { inherit system; config.allowUnfree = true; };
          hm = flake.inputs.home-manager.lib.homeManagerConfiguration {
            inherit pkgs;
            modules = [
              ({ ... }: {
                home.username = \"user\";
                home.homeDirectory = \"/tmp/mysetup-home\";
                home.stateVersion = \"25.11\";
              })
              (nixosDir + \"/home/shells/default.nix\")
            ];
            extraSpecialArgs = { };
          };
        in ${attr}
    "
}

# eval_hm_full <preset|""> <attribute>
# Evaluates the full home/home.nix module with mysetup, mysetupLib and the
# stable/bleeding pkgs sets wired through extraSpecialArgs. When <preset> is
# non-empty, the host-vars defaults are overridden so the chosen preset is
# selected; otherwise the host-vars defaults are used as-is.
eval_hm_full() {
    local preset="$1"
    local attr="$2"

    MYSETUP_PRESET_OVERRIDE="${preset}" _nix_eval "
        let
          nixosDir = builtins.getEnv \"MYSETUP_NIXOS_DIR\";
          presetOverride = builtins.getEnv \"MYSETUP_PRESET_OVERRIDE\";
          flake = builtins.getFlake (\"path:\" + nixosDir);
          system = \"x86_64-linux\";
          pkgs = import flake.inputs.nixpkgs { inherit system; config.allowUnfree = true; };
          lib = pkgs.lib;
          mysetupLib = import (nixosDir + \"/lib/mysetup.nix\") { inherit lib; };
          pkgsStable = import flake.inputs.nixpkgs-stable {
            localSystem = system;
            config.allowUnfree = true;
            config.permittedInsecurePackages = [ \"python3.12-pypdf2-3.0.1\" ];
          };
          pkgsBleeding = import flake.inputs.nixpkgs-bleeding {
            localSystem = system;
            config.allowUnfree = true;
          };
          varsModule = import (nixosDir + \"/hosts/NixOS/variables.nix\") { inherit lib; };
          baseMysetup = varsModule.config.mysetup;
          mysetup =
            if presetOverride == \"\"
            then baseMysetup
            else baseMysetup // {
              packages = baseMysetup.packages // { preset = presetOverride; };
            };
          hm = flake.inputs.home-manager.lib.homeManagerConfiguration {
            inherit pkgs;
            modules = [ (nixosDir + \"/home/home.nix\") ];
            extraSpecialArgs = {
              inputs = flake.inputs;
              inherit mysetupLib mysetup;
              pkgs-stable = pkgsStable;
              pkgs-bleeding = pkgsBleeding;
            };
          };
        in ${attr}
    "
}

# Export so subshells inherit and the Nix expression can read it via getEnv.
export MYSETUP_NIXOS_DIR="${NIXOS_DIR}"
