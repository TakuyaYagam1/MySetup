{ pkgs, ... }:

{
  home.packages = with pkgs; [
    # Calculators
    libqalculate
    qalculate-gtk

    # System info / utilities
    bulky
    xneur
    app2unit

    # Terminal tools
    tmux
    zellij
  ];
}
