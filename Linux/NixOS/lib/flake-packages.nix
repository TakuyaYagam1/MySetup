{
  layout,
  nixpkgs,
  system,
}:

let
  flakePkgs = import nixpkgs {
    localSystem = system;
    config.allowUnfree = true;
  };

  nixosSource = layout.nixos;
  dotsSource = layout.dots;
  installerSource = layout.installer;

  mysetupRuntimeSource = flakePkgs.runCommand "mysetup-runtime-source" { } ''
    mkdir -p "$out"
    cp -a ${nixosSource} "$out/NixOS"
    cp -a ${dotsSource} "$out/dots"
    cp -a ${installerSource} "$out/installer"
  '';

  mysetup = flakePkgs.buildGoModule {
    pname = "mysetup";
    version = "0.1.0";
    src = installerSource;
    subPackages = [ "cmd/mysetup" ];
    vendorHash = "sha256-3BLXjtDy2dsq7A12BmAkoOQbu/hYkhVm4GKCtqYglTo=";
    nativeBuildInputs = [ flakePkgs.makeWrapper ];
    ldflags = [
      "-s"
      "-w"
    ];
    postInstall = ''
      wrapProgram $out/bin/mysetup \
        --set MYSETUP_REPO_ROOT ${mysetupRuntimeSource}/NixOS \
        --prefix PATH : ${
          flakePkgs.lib.makeBinPath (
            with flakePkgs;
            [
              coreutils
              findutils
              gnused
              rsync
              mkpasswd
              nix
              nixos-rebuild
              git
              curl
              jq
              hyprland
              libarchive
              unzip
              sing-box
            ]
          )
        }
    '';
  };

  packages = {
    omnirouter = flakePkgs.callPackage ../packages/omnirouter.nix { };
    inherit mysetup;
    default = mysetup;
  };

  checks = {
    inherit mysetup;
  };

  mysetupApp = {
    type = "app";
    program = "${packages.mysetup}/bin/mysetup";
    meta.description = "Run the MySetup NixOS installer";
  };
in
{
  inherit checks packages;

  apps = {
    mysetup = mysetupApp;
    default = mysetupApp;
  };
}
