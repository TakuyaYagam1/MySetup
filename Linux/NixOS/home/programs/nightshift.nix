{
  lib,
  mysetup,
  mysetupLib,
  pkgs,
  ...
}:

let
  desktopOrMore = mysetupLib.presets.desktopOrMore mysetup;
  nightshift-toggle = pkgs.writeShellScriptBin "nightshift-toggle" ''
    if ${pkgs.procps}/bin/pgrep -x hyprsunset >/dev/null; then
      ${pkgs.procps}/bin/pkill -x hyprsunset
      ${pkgs.libnotify}/bin/notify-send "Night Shift Disabled" "Blue light filter disabled."
    else
      ${pkgs.hyprsunset}/bin/hyprsunset -t 4500 >/dev/null 2>&1 &
      ${pkgs.libnotify}/bin/notify-send "Night Shift Enabled" "Blue light filter set to 4500K."
    fi
  '';
in
{
  config = lib.mkIf desktopOrMore {
    home.packages = [
      pkgs.hyprsunset
      nightshift-toggle
    ];

    programs.fish.shellAliases.nightshift = "nightshift-toggle";

    xdg.desktopEntries.nightshift-toggle = {
      name = "Night Shift Toggle";
      exec = "nightshift-toggle";
      icon = "weather-clear-night";
      type = "Application";
      categories = [ "Utility" ];
      terminal = false;
    };
  };
}
