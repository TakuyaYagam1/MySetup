{ config, ... }:

# SYSTEM-LEVEL sops-nix. Secrets live in ./secrets.yaml (encrypted) and are
# decrypted at activation time into /run/secrets/<name>, using the host key
# from /etc/ssh/ssh_host_ed25519_key.
#
# For USER secrets (~/.ssh/config, ~/.netrc, ssh keys) use
# home/secrets/default.nix - the HM-level module with a separate user age key.
#
# Bootstrap steps:
#   1. nix-shell -p sops ssh-to-age age
#   2. ssh-to-age -private-key -i /etc/ssh/ssh_host_ed25519_key > /tmp/hostkey.txt
#      (or: sudo ssh-to-age < /etc/ssh/ssh_host_ed25519_key.pub)
#   3. Put the public age key into .sops.yaml (see hosts/NixOS/secrets/.sops.yaml)
#   4. sops hosts/NixOS/secrets/secrets.yaml  -> add entries
#   5. Enable this module by importing it from hosts/NixOS/default.nix
#      and uncommenting the `sops.secrets.*` entries below.

{
  sops = {
    defaultSopsFile = ./secrets.yaml;
    defaultSopsFormat = "yaml";

    age.sshKeyPaths = [ "/etc/ssh/ssh_host_ed25519_key" ];

    # Basic example - secret available at /run/secrets/example, readable by user:
    # secrets.example = {
    #   owner = config.var.username;
    #   group = "users";
    #   mode = "0400";
    # };
    #
    # Place a decrypted secret at a custom path (outside /run/secrets):
    # secrets.wg-private = {
    #   path = "/etc/wireguard/private.key";
    #   mode = "0400";
    # };
    #
    # Restart a service when the secret rotates (sops-nix tracks the hash):
    # secrets.pgadmin-password = {
    #   owner = "pgadmin";
    #   restartUnits = [ "pgadmin.service" ];
    # };
  };
}
