{
  end4Lib,
  inputs,
  lib,
  ...
}:

let
  dotfilesSource = inputs.end4-dotfiles;
  inherit (end4Lib) dotfilesLib;
in
{
  home.activation.end4SeedAppConfig =
    lib.hm.dag.entryAfter
      [
        "writeBoundary"
        "wahrweltV1ToV2End4AppSeed"
      ]
      ''
        config_dir="$HOME/.config"
        data_dir="$HOME/.local/share"
        kitty_target="$HOME/.config/kitty"
        fuzzel_target="$HOME/.config/fuzzel"
        applications_dir="$HOME/.local/share/applications"
        kdeglobals_target="$HOME/.config/kdeglobals"
        konsole_target="$HOME/.local/share/konsole"
        ${dotfilesLib.mutableJsonShellHelpers}
        ensure_real_directory "$config_dir" || exit $?
        ensure_real_directory "$data_dir" || exit $?
        ensure_real_directory "$applications_dir" || exit $?
        ensure_real_directory "$konsole_target" || exit $?

        # Existing user files are never replaced or chmodded.
        seed_if_missing "$kdeglobals_target" "${dotfilesSource}/dots/.config/kdeglobals" || exit $?

        # section: konsole - seed default profile if missing
        seed_if_missing "$konsole_target/Profile 1.profile" \
          "${dotfilesSource}/dots/.local/share/konsole/Profile 1.profile" || exit $?

        # Seed whole app directories only when absent. Existing directories are
        # user-owned and are left byte-for-byte untouched.
        seed_directory_if_missing "$kitty_target" "${dotfilesSource}/dots/.config/kitty" || exit $?
        seed_directory_if_missing "$fuzzel_target" "${dotfilesSource}/dots/.config/fuzzel" || exit $?

        # section: rfdump - copy system desktop entry with absolute icon path
        rfdump_desktop="$applications_dir/rfdump.desktop"
        rfdump_icon="/run/current-system/sw/share/pixmaps/rfdump.png"
        if [ -f "/run/current-system/sw/share/applications/rfdump.desktop" ] && [ -f "$rfdump_icon" ]; then
          seed_with_replaced_line_if_missing \
            "$rfdump_desktop" \
            "/run/current-system/sw/share/applications/rfdump.desktop" \
            "Icon=" \
            "Icon=$rfdump_icon" || exit $?
        fi
      '';
}
