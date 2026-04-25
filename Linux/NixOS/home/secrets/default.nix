{ config, pkgs, inputs, ... }:

# HM-level sops-nix (per-user age key, secrets decrypted under $HOME).
# Bootstrap:
#   age-keygen -o ~/.config/sops/age/keys.txt
#   age-keygen -y ~/.config/sops/age/keys.txt   # public key -> add to .sops.yaml as &user_main
#   sops home/secrets/secrets.yaml              # create encrypted store
#   # Uncomment ./secrets in home/home.nix and the sops.secrets.* entries below
#   systemctl --user enable --now sops-nix.service
# Hyprland fallback if not using the systemd unit: exec-once = systemctl --user start sops-nix

let
  home = config.home.homeDirectory;
in {
  imports = [ inputs.sops-nix.homeManagerModules.sops ];

  sops = {
    age.keyFile = "${home}/.config/sops/age/keys.txt";
    defaultSopsFile = ./secrets.yaml;
    defaultSopsFormat = "yaml";

    # Uncomment as real keys appear in secrets.yaml.
    # Each `path` is the target decryption location in the home directory.
    #
    # secrets = {
    #   ssh-config = {
    #     path = "${home}/.ssh/config";
    #     mode = "0600";
    #   };
    #   netrc = {
    #     path = "${home}/.netrc";
    #     mode = "0600";
    #   };
    #   github-key = {
    #     path = "${home}/.ssh/github";
    #     mode = "0400";
    #   };
    #   gitlab-key = {
    #     path = "${home}/.ssh/gitlab";
    #     mode = "0400";
    #   };
    # };
  };

  # Services that depend on decrypted secrets must wait for sops-nix.
  # Example (uncomment when adding units):
  #
  # systemd.user.services.mbsync.Unit.After = [ "sops-nix.service" ];

  # sops/age CLI for convenient local secret editing
  # (`sops home/secrets/secrets.yaml` - opens $EDITOR, encrypts on save).
  home.packages = with pkgs; [ sops age ];
}
