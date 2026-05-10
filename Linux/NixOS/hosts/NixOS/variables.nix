_:

{
  imports = [
    ../../modules/mysetup-options.nix
  ];

  config.mysetup = import ./host-vars.nix;
}
