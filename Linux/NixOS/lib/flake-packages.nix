{
  layout,
  nixpkgs,
  system,
}:

let
  flakePkgs = import nixpkgs {
    localSystem = system;
    config = {
      allowUnfree = true;
      allowInsecurePredicate = _: true;
    };
  };

  nixosSource = layout.nixos;
  dotsSource = layout.dots;
  installerSource = layout.installer;

  wahrweltRuntimeSource = flakePkgs.runCommand "wahrwelt-runtime-source" { } ''
    mkdir -p "$out"
    cp -a ${nixosSource} "$out/NixOS"
    cp -a ${dotsSource} "$out/dots"
    cp -a ${installerSource} "$out/installer"
  '';

  wahrwelt = flakePkgs.buildGoModule {
    pname = "wahrwelt";
    version = "0.1.0";
    src = installerSource;
    subPackages = [ "cmd/wahrwelt" ];
    vendorHash = "sha256-owIDnnxJBzzo9Jdn+Avn0bRBXMQPnfYzxh8/5viBw+Y=";
    nativeBuildInputs = [ flakePkgs.makeWrapper ];
    ldflags = [
      "-s"
      "-w"
    ];
    postInstall = ''
      ln -s wahrwelt $out/bin/mysetup
      wrapProgram $out/bin/wahrwelt \
        --set WAHRWELT_REPO_ROOT ${wahrweltRuntimeSource}/NixOS \
        --set MYSETUP_REPO_ROOT ${wahrweltRuntimeSource}/NixOS \
        --set WAHRWELT_XKB_RULES_DIR ${flakePkgs.xkeyboard_config}/share/X11/xkb/rules \
        --set MYSETUP_XKB_RULES_DIR ${flakePkgs.xkeyboard_config}/share/X11/xkb/rules \
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
    claude-desktop = flakePkgs.callPackage ../pkgs/claude-desktop.nix { };
    omnirouter = flakePkgs.callPackage ../pkgs/omnirouter.nix { };
    inherit wahrwelt;
    mysetup = wahrwelt;
    default = wahrwelt;
  };

  checks = {
    inherit wahrwelt;
    mysetup = wahrwelt;
  };

  wahrweltApp = {
    type = "app";
    program = "${packages.wahrwelt}/bin/wahrwelt";
    meta.description = "Run the Wahrwelt NixOS installer";
  };

  mysetupApp = {
    type = "app";
    program = "${packages.mysetup}/bin/mysetup";
    meta.description = "Run the legacy MySetup-compatible NixOS installer entrypoint";
  };
in
{
  inherit checks packages;

  apps = {
    wahrwelt = wahrweltApp;
    mysetup = mysetupApp;
    default = wahrweltApp;
  };
}
