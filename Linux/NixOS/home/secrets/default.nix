{
  config,
  pkgs,
  inputs,
  ...
}:

let
  home = config.home.homeDirectory;
in
{
  imports = [ inputs.sops-nix.homeManagerModules.sops ];

  sops = {
    age.keyFile = "${home}/.config/sops/age/keys.txt";
    defaultSopsFile = ./secrets.yaml;
    defaultSopsFormat = "yaml";
  };

  # Local secret editing helpers; bootstrap examples live in Linux/README.md.
  home.packages = with pkgs; [
    sops
    age
  ];
}
