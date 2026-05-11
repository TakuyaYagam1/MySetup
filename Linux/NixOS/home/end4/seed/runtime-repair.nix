{ lib, pkgs, ... }:

{
  home.activation.end4PruneStaleProfile = lib.hm.dag.entryBefore [ "checkLinkTargets" ] ''
    end4_dir="$HOME/.config/hypr/end4"

    if [ -d "$end4_dir" ] && [ ! -L "$end4_dir" ] && [ ! -f "$end4_dir/hyprland.conf" ]; then
      backup="$end4_dir.stale.$(${pkgs.coreutils}/bin/date +%Y%m%d%H%M%S)"
      if [ -e "$backup" ]; then
        backup="$backup.$$"
      fi
      $DRY_RUN_CMD mv -- "$end4_dir" "$backup"
    fi
  '';

  home.activation.end4PruneRedundantSymlinks = lib.hm.dag.entryAfter [ "writeBoundary" ] ''
    hypr_dir="$HOME/.config/hypr"

    for name in monitors.conf workspaces.conf; do
      target="$hypr_dir/$name"
      if [ -L "$target" ]; then
        resolved="$(${pkgs.coreutils}/bin/readlink -f "$target" 2>/dev/null || true)"
        case "$resolved" in
          "$hypr_dir/end4/"*)
            $DRY_RUN_CMD ${pkgs.coreutils}/bin/rm -f "$target"
            ;;
        esac
      fi
    done
  '';
}
