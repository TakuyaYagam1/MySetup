# Mutable state seeding for the end4 shell.
# Runs at home-manager activation: jq-patches user JSON configs (config.nix),
# and bootstraps app dotfiles (apps.nix). Idempotent.
_:

{
  imports = [
    ./config.nix
    ./apps.nix
  ];
}
