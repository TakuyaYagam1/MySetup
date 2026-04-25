{ config, pkgs, lib, var, ... }:

# Caelestia shell entry point.
#
# The shell.json contract is too rich to merge cleanly via NixOS module
# semantics (deeply nested attrsets, list overrides, vendor-defined keys).
# Instead each slice file (./bar.nix, ./launcher.nix, ...) writes into a
# single freeform `caelestiaShellSettings` option, which we materialise
# into JSON and seed via home.activation.
#
# The activation step is hash-gated: it only rewrites ~/.config/caelestia/shell.json
# when the source JSON actually changes. This keeps manual tweaks across rebuilds
# until the next config edit lands.

let
  shellJson = pkgs.writeText "caelestia-shell.json"
    (builtins.toJSON config.caelestiaShellSettings);
in
{
  imports = [
    ./appearance.nix
    ./background.nix
    ./bar.nix
    ./general.nix
    ./launcher.nix
    ./services.nix
    ./session.nix
    ./utilities.nix
  ];

  options.caelestiaShellSettings = lib.mkOption {
    type = lib.types.attrsOf lib.types.anything;
    default = { };
    description = ''
      Aggregated settings tree for ~/.config/caelestia/shell.json.
      Slice files contribute keys (bar, launcher, services, ...) which are
      merged via the standard NixOS module system, then serialised to JSON
      and seeded onto disk.
    '';
  };

  config = {
    programs.caelestia = {
      enable = true;

      systemd = {
        enable = false;
        target = "graphical-session.target";
        environment = [ ];
      };

      cli = {
        enable = true;
        settings.theme.enableGtk = false;
      };
    };

    home.activation.caelestiaSeedShellJson =
      lib.hm.dag.entryAfter [ "writeBoundary" ] ''
        target="$HOME/.config/caelestia/shell.json"
        hashFile="$HOME/.config/caelestia/.shell.json.nix-hash"
        src="${shellJson}"
        newHash="$(${pkgs.coreutils}/bin/sha256sum "$src" | ${pkgs.coreutils}/bin/cut -d' ' -f1)"

        $DRY_RUN_CMD ${pkgs.coreutils}/bin/mkdir -p "$HOME/.config/caelestia"

        if [ ! -f "$target" ] || [ ! -f "$hashFile" ] || \
           [ "$(${pkgs.coreutils}/bin/cat "$hashFile" 2>/dev/null)" != "$newHash" ]; then
          $DRY_RUN_CMD ${pkgs.coreutils}/bin/install -m 644 "$src" "$target"
          $DRY_RUN_CMD ${pkgs.bash}/bin/sh -c "printf '%s\n' '$newHash' > '$hashFile'"
        fi
      '';

    home.packages = [ pkgs.quickshell ];
  };
}
