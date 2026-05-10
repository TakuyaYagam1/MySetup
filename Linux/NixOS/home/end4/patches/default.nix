# Build-time patching of upstream end4 sources.
# Builds derivations that overlay MySetup customisations on top of upstream
# Hyprland (hypr.nix) and Quickshell (quickshell.nix) configs, and links
# rendered dotfiles into XDG paths (xdg-files.nix).
_:

{
  imports = [
    ./quickshell.nix
    ./hypr.nix
    ./xdg-files.nix
  ];
}
