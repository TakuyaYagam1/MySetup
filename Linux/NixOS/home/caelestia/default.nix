{ config, pkgs, lib, var, ... }:

# Caelestia shell entry point.
#
# The shell.json contract is too rich to merge cleanly via NixOS module
# semantics (deeply nested attrsets, list overrides, vendor-defined keys).
# Instead each slice file (./bar.nix, ./launcher.nix, ...) writes into a
# single freeform `caelestiaShellSettings` option, which we materialise
# into JSON and seed via home.activation.
#
# The activation step only seeds ~/.config/caelestia/shell.json when it is
# missing. After that the shell UI or manual JSON edits own the live state.

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
        src="${shellJson}"

        $DRY_RUN_CMD ${pkgs.coreutils}/bin/mkdir -p "$HOME/.config/caelestia"

        if [ -L "$target" ]; then
          resolved="$(${pkgs.coreutils}/bin/readlink -f "$target" 2>/dev/null || true)"
          case "$resolved" in
            /nix/store/*)
              $DRY_RUN_CMD ${pkgs.coreutils}/bin/rm -f "$target"
              ;;
          esac
        fi

        if [ ! -e "$target" ]; then
          $DRY_RUN_CMD ${pkgs.coreutils}/bin/install -m 644 "$src" "$target"
        fi

        if [ -z "''${DRY_RUN_CMD:-}" ] && [ -e "$target" ]; then
          tmp="$target.tmp.$$"
          if ${pkgs.jq}/bin/jq 'if .bar.excludedScreens? then .bar.excludedScreens |= map(select(. != "")) else . end' "$target" > "$tmp"; then
            ${pkgs.coreutils}/bin/mv "$tmp" "$target"
          else
            ${pkgs.coreutils}/bin/rm -f "$tmp"
          fi
        fi
      '';

    home.packages = [ pkgs.quickshell ];
  };
}
