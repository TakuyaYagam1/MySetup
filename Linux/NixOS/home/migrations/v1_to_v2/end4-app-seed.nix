{
  lib,
  pkgs,
  ...
}:

let
  linkGuard = pkgs.writeShellApplication {
    name = "wahrwelt-v1-to-v2-end4-link-guard";
    runtimeInputs = [ pkgs.python3 ];
    text = ''
      exec python3 ${./link-guard.py} "$@"
    '';
  };
in
{
  # v1 managed these mutable app paths as Home Manager store links, then later
  # releases converted them to ordinary writable paths. Preserve an ordinary
  # path of the expected type as already migrated. Remove only links from the
  # exact old generation layout when that generation contains both End4 markers.
  home.activation.wahrweltV1ToV2End4AppSeed = lib.hm.dag.entryAfter [ "writeBoundary" ] ''
    migrate_v1_end4_hm_link() {
      local target="$1"
      local relative="$2"
      local token recovery

      token="$("${linkGuard}/bin/wahrwelt-v1-to-v2-end4-link-guard" \
        check "$target" "$relative" "" "" "$HOME")" || return
      if [ -n "''${DRY_RUN_CMD:-}" ]; then
        $DRY_RUN_CMD "${linkGuard}/bin/wahrwelt-v1-to-v2-end4-link-guard" \
          quarantine "$target" "$relative" "" "" "$HOME" "$token"
        return
      fi
      recovery="$("${linkGuard}/bin/wahrwelt-v1-to-v2-end4-link-guard" \
        quarantine "$target" "$relative" "" "" "$HOME" "$token")" || return
      if [ -n "$recovery" ]; then
        printf 'Wahrwelt v1_to_v2 End4 link recovery retained at %s\n' "$recovery"
      fi
    }

    migrate_v1_end4_hm_link "$HOME/.config/kitty" ".config/kitty" || exit $?
    migrate_v1_end4_hm_link "$HOME/.config/fuzzel" ".config/fuzzel" || exit $?
    migrate_v1_end4_hm_link "$HOME/.config/kdeglobals" ".config/kdeglobals" || exit $?
    migrate_v1_end4_hm_link "$HOME/.local/share/konsole/Profile 1.profile" \
      ".local/share/konsole/Profile 1.profile" || exit $?
  '';
}
