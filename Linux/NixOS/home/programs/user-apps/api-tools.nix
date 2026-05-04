{ lib, pkgs, pkgs-stable, var, ... }:

let
  preset = var.packagePreset or "personal";
  enabled = lib.elem preset [ "developer" "personal" ];
in
{
  config = lib.mkIf enabled {
    home.packages = (with pkgs; [
    # API clients
    yaak
    insomnia

    # Database GUIs / TUIs
    dbeaver-bin
    sqlit-tui
    pkgs-stable.pgbadger

    # Terminal multiplexers / advanced terminals
    warp-terminal
    termius
    ]) ++ lib.optionals (!(var.features.russiaMode or false)) [
      pkgs.jetbrains.datagrip
    ];
  };
}
