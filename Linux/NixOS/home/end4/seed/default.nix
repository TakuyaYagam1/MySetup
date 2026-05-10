# Mutable state seeding for the end4 shell.
# Runs at home-manager activation: jq-patches user JSON configs (config.nix),
# bootstraps app dotfiles (apps.nix), and recovers writable trees from
# read-only Nix store symlinks (runtime-repair.nix). Idempotent.
_:

{
  imports = [
    ./runtime-repair.nix
    ./config.nix
    ./apps.nix
  ];
}
