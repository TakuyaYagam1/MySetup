{ pkgs, pkgs-stable, ... }:

{
  home.packages = with pkgs; [
    # API clients
    yaak
    insomnia

    # Database GUIs / TUIs
    dbeaver-bin
    jetbrains.datagrip
    sqlit-tui
    pkgs-stable.pgbadger

    # Terminal multiplexers / advanced terminals
    warp-terminal
    termius
  ];
}
