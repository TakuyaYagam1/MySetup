# Build-time patching of upstream end4 sources.
# Builds derivations that overlay Wahrwelt customisations on top of upstream
# Hyprland (hypr.nix) and Quickshell (quickshell.nix) configs, and links
# rendered dotfiles into XDG paths (xdg-files.nix).
_:

{
  imports = [
    ./quickshell.nix
    ./quickshell-pc.nix
    ./hypr.nix
    ./xdg-files.nix
  ];
}
