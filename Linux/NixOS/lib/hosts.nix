{
  flakeModules,
  home-manager,
  inputs,
  mysetupLib,
  nixpkgs,
  pkgs-bleeding,
  pkgs-stable,
  system,
  zapret-discord-youtube,
}:

let
  hostVars = import ../hosts/NixOS/host-vars.nix;
  hostname = if hostVars ? host then hostVars.host.hostname else hostVars.hostname;
in
{
  nixosConfigurations.${hostname} = nixpkgs.lib.nixosSystem {
    inherit system;

    specialArgs = {
      inherit
        inputs
        mysetupLib
        pkgs-bleeding
        pkgs-stable
        ;
    };

    modules = [
      ../hosts/NixOS

      flakeModules.overlaysModule

      inputs.lanzaboote.nixosModules.lanzaboote
      zapret-discord-youtube.nixosModules.default
      inputs.nix-snapd.nixosModules.default
      inputs.stylix.nixosModules.stylix
      inputs.sops-nix.nixosModules.sops

      home-manager.nixosModules.home-manager
      flakeModules.homeManagerModule
    ];
  };
}
