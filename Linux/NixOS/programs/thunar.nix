{
  config,
  mysetupLib,
  pkgs,
  ...
}:

let
  user = config.mysetup.user.username;
  xdgDataDirs = builtins.concatStringsSep ":" [
    "/run/current-system/sw/share"
    "/etc/profiles/per-user/${user}/share"
    "/home/${user}/.nix-profile/share"
    "/home/${user}/.local/share/flatpak/exports/share"
    "/var/lib/flatpak/exports/share"
    "/usr/local/share"
    "/usr/share"
  ];
in
{
  config = mysetupLib.mkIfPresetOrMore "desktop" config.mysetup {
    programs.thunar = {
      enable = true;
      plugins = with pkgs; [
        thunar-archive-plugin
        thunar-volman
      ];
    };

    environment.etc."environment.d/10-mysetup-xdg.conf".text = ''
      XDG_DATA_DIRS=${xdgDataDirs}
    '';

    systemd.user.extraConfig = ''
      DefaultEnvironment="XDG_DATA_DIRS=${xdgDataDirs}"
    '';

    environment.systemPackages = with pkgs; [
      gdk-pixbuf
      librsvg
    ];
  };
}
