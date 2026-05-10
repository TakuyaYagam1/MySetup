_:

# SYSTEM-LEVEL sops-nix. Secrets live in ./secrets.yaml (encrypted) and are
# decrypted at activation time into /run/secrets/<name>, using the host key
# from /etc/ssh/ssh_host_ed25519_key.
# User-level secrets are handled by home/secrets/default.nix.

{
  sops = {
    defaultSopsFile = ./secrets.yaml;
    defaultSopsFormat = "yaml";

    age.sshKeyPaths = [ "/etc/ssh/ssh_host_ed25519_key" ];

    secrets."pgadmin-password" = { };
  };
}
