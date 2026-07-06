{
  config,
  homeLibs,
  pkgs,
  ...
}:

let
  bootThemeDir = "${config.home.homeDirectory}/.config/mysetup/boot-theme";
  readme = pkgs.writeText "mysetup-boot-theme-readme" ''
    MySetup boot theme overrides
    ============================

    Drop a PNG or JPG image in this directory to customize the GRUB, SDDM,
    and Plymouth boot logos. Changes need `mysetup apply` / `nixos-rebuild
    switch` to take effect; GRUB and Plymouth only become visible after a
    reboot (they render before Linux is running).

      logo.png / logo.jpg           -> used for grub, sddm, and plymouth
      grub.png / grub.jpg           -> overrides just the GRUB boot menu
      sddm.png / sddm.jpg           -> overrides just the SDDM login avatar
      plymouth.png / plymouth.jpg   -> overrides just the Plymouth splash

    A per-service file always wins over logo.png. Delete a file to fall
    back to the next one in that order, or delete them all to restore the
    built-in default.
  '';
in
{
  home.activation.mysetupSeedBootTheme = homeLibs.shellSeed.mkSeedActivation {
    dirs = [ bootThemeDir ];
    body = ''
      seed_if_missing "${bootThemeDir}/logo.png" "${../../themes/sddm-theme/icons/logo.png}"
      seed_if_missing "${bootThemeDir}/README.txt" "${readme}"
    '';
  };
}
