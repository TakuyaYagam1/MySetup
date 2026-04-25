# Switch active theme here. Just change the `import` below to load a
# different palette - everything that consumes `config.theme.*` and
# `config.lib.stylix.colors.*` will re-derive automatically on the next
# `nixos-rebuild switch`.
{ ... }:

{
  imports = [
    ./caelestia-dark.nix
  ];
}
