{
  config,
  homeLibs,
  pkgs,
  ...
}:

let
  bootThemeDir = "${config.home.homeDirectory}/.config/mysetup/boot-theme";
  seededMarker = "${bootThemeDir}/.seeded";
  readme = pkgs.writeText "mysetup-boot-theme-readme" ''
    Wahrwelt boot theme overrides
    ============================

    Drop a PNG or JPG image in this directory to customize the GRUB, SDDM,
    and Plymouth boot logos. Changes need `mysetup apply` / `nixos-rebuild
    switch` to take effect; GRUB and Plymouth only become visible after a
    reboot (they render before Linux is running).

      logo.png / logo.jpg           -> used for grub, sddm, and plymouth
      grub.png / grub.jpg           -> overrides just the GRUB boot menu
      sddm.png / sddm.jpg           -> overrides just the SDDM login avatar
      plymouth.png / plymouth.jpg   -> overrides just the Plymouth splash

    A per-service file always wins over logo.png. Once this directory
    exists, every one of grub/sddm/plymouth must resolve to a real file
    (its own override or logo.png) - covering some services and not
    others fails the build on purpose instead of silently guessing.
    Delete the whole directory to go back to the built-in default.
  '';
in
{
  home.activation.mysetupSeedBootTheme = homeLibs.shellSeed.mkSeedActivation {
    dirs = [ bootThemeDir ];
    body = ''
      if [ ! -e "${seededMarker}" ]; then
        seed_if_missing "${bootThemeDir}/logo.png" "${../../themes/sddm-theme/icons/logo.png}"
        seed_if_missing "${bootThemeDir}/README.txt" "${readme}"
        $DRY_RUN_CMD ${pkgs.coreutils}/bin/touch "${seededMarker}"
      fi
    '';
  };
}
