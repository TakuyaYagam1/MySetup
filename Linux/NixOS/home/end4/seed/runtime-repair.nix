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

  home.activation.end4RepairRuntime = lib.hm.dag.entryAfter [ "writeBoundary" ] ''
    hypr_dir="$HOME/.config/hypr"
    end4_dir="$hypr_dir/end4"
    active_shell_file="$HOME/.local/state/mysetup/active-shell"
    active_shell=""

    $DRY_RUN_CMD mkdir -p "$hypr_dir"

    if [ -f "$active_shell_file" ]; then
      active_shell="$(${pkgs.coreutils}/bin/tr -d '[:space:]' < "$active_shell_file" 2>/dev/null || true)"
    fi

    if [ "$active_shell" = "end4" ]; then
      $DRY_RUN_CMD rm -f "$hypr_dir/monitors.conf" "$hypr_dir/workspaces.conf"
      $DRY_RUN_CMD ln -sfn "$end4_dir/monitors.conf" "$hypr_dir/monitors.conf"
      $DRY_RUN_CMD ln -sfn "$end4_dir/workspaces.conf" "$hypr_dir/workspaces.conf"
    fi
  '';
}
