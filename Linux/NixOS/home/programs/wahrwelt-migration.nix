{
  config,
  lib,
  pkgs,
  ...
}:

let
  home = config.home.homeDirectory;
  configHome = config.xdg.configHome;
  stateHome = config.xdg.stateHome;
  cacheHome = config.xdg.cacheHome;
in
{
  home.activation.migrateWahrweltUserPaths = lib.hm.dag.entryBefore [ "checkLinkTargets" ] ''
    move_tree() {
      old="$1"
      new="$2"

      if [ ! -e "$old" ] && [ ! -L "$old" ]; then
        return 0
      fi
      if [ -e "$new" ] || [ -L "$new" ]; then
        echo "Wahrwelt migration conflict: both $old and $new exist" >&2
        return 1
      fi

      $DRY_RUN_CMD ${pkgs.coreutils}/bin/mkdir -p "$(${pkgs.coreutils}/bin/dirname "$new")"
      $DRY_RUN_CMD ${pkgs.coreutils}/bin/mv -- "$old" "$new"
    }

    merge_cache() {
      old="$1"
      new="$2"

      if [ ! -e "$old" ] && [ ! -L "$old" ]; then
        return 0
      fi
      if [ ! -e "$new" ] && [ ! -L "$new" ]; then
        move_tree "$old" "$new"
        return
      fi
      if [ ! -d "$old" ] || [ -L "$old" ] || [ ! -d "$new" ] || [ -L "$new" ]; then
        echo "Wahrwelt migration conflict: cache paths must be directories: $old, $new" >&2
        return 1
      fi

      $DRY_RUN_CMD ${pkgs.rsync}/bin/rsync -a --ignore-existing "$old/" "$new/"
      $DRY_RUN_CMD ${pkgs.coreutils}/bin/rm -rf -- "$old"
    }

    move_tree "${configHome}/mysetup" "${configHome}/wahrwelt"
    move_tree "${configHome}/hypr/mysetup" "${configHome}/hypr/wahrwelt"
    move_tree "${stateHome}/mysetup" "${stateHome}/wahrwelt"
    merge_cache "${cacheHome}/mysetup" "${cacheHome}/wahrwelt"

    for old_link in \
      "${configHome}/hypr/lib/mysetup.lua" \
      "${configHome}/quickshell/mysetup-shell-selector"
    do
      if [ -L "$old_link" ]; then
        $DRY_RUN_CMD ${pkgs.coreutils}/bin/rm -f -- "$old_link"
      elif [ -e "$old_link" ]; then
        echo "Wahrwelt migration conflict: refusing to remove non-symlink $old_link" >&2
        exit 1
      fi
    done

    while IFS= read -r -d "" old_marker; do
      marker_dir="$(${pkgs.coreutils}/bin/dirname "$old_marker")"
      new_marker="$marker_dir/.wahrwelt-managed.json"
      if [ -e "$new_marker" ]; then
        if ${pkgs.gnugrep}/bin/grep -q '"manager": "mysetup"' "$old_marker" &&
          ${pkgs.gnugrep}/bin/grep -q '"manager": "wahrwelt"' "$new_marker"; then
          $DRY_RUN_CMD ${pkgs.coreutils}/bin/rm -f -- "$old_marker"
          continue
        fi
        echo "Wahrwelt migration conflict: incompatible markers $old_marker and $new_marker" >&2
        exit 1
      fi
      if [ -z "''${DRY_RUN_CMD:-}" ]; then
        ${pkgs.gnused}/bin/sed \
          -e 's/"manager": "mysetup"/"manager": "wahrwelt"/g' \
          "$old_marker" > "$new_marker"
        ${pkgs.coreutils}/bin/chmod --reference="$old_marker" "$new_marker"
        ${pkgs.coreutils}/bin/rm -f -- "$old_marker"
      else
        echo "Would migrate marker $old_marker -> $new_marker"
      fi
    done < <(${pkgs.findutils}/bin/find "${configHome}" -type f -name .mysetup-managed.json -print0 2>/dev/null)

    for text_file in \
      "${configHome}/wahrwelt/boot-theme/README.txt" \
      "${configHome}/hypr/wahrwelt/keybinds.lua" \
      "${stateHome}/wahrwelt/hypr-runtime/hyprland.lua"
    do
      if [ -f "$text_file" ]; then
        $DRY_RUN_CMD ${pkgs.gnused}/bin/sed -i \
          -e 's/MYSETUP/WAHRWELT/g' \
          -e 's/MySetup/Wahrwelt/g' \
          -e 's/mysetup/wahrwelt/g' \
          "$text_file"
      fi
    done

    for stale_backup in \
      "${configHome}/hypr/lib/mysetup.lua.backup" \
      "${configHome}/hypr/mysetup/hyprland.lua.backup"
    do
      if [ -e "$stale_backup" ]; then
        $DRY_RUN_CMD ${pkgs.coreutils}/bin/rm -f -- "$stale_backup"
      fi
    done

    legacy_runtime="/tmp/mysetup-runtime-$(${pkgs.coreutils}/bin/id -u)"
    if [ -d "$legacy_runtime" ]; then
      $DRY_RUN_CMD ${pkgs.coreutils}/bin/rm -rf -- "$legacy_runtime"
    fi

    legacy_cli="${home}/.local/bin/mysetup"
    if [ -L "$legacy_cli" ] && [ "$(${pkgs.coreutils}/bin/readlink "$legacy_cli")" = "wahrwelt" ]; then
      $DRY_RUN_CMD ${pkgs.coreutils}/bin/rm -f -- "$legacy_cli"
    fi
  '';
}
