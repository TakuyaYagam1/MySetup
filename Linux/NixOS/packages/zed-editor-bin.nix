{
  stdenv,
  fetchurl,
  patchelf,
  makeWrapper,
  libbsd,
  libX11,
  libXau,
  libxcb,
  libXdmcp,
  libxkbcommon,
  zlib,
  alsa-lib,
  wayland,
  vulkan-loader,
  buildFHSEnv,
  lib,
}:
let
  version = "0.232.2";

  src = fetchurl {
    url = "https://github.com/zed-industries/zed/releases/download/v${version}/zed-linux-x86_64.tar.gz";
    hash = "sha256-wpu3nOOOnMvkDZVtmgJVnA42iRb/JyuJK9WF6IlBX5M=";
  };

  nixDeps = [
    libbsd
    libX11
    libXau
    libxcb
    libXdmcp
    libxkbcommon
    zlib
    alsa-lib
    wayland
    vulkan-loader
  ];

  libPath = lib.makeLibraryPath nixDeps;

  executableName = "zeditor";

  fhs =
    {
      zed-editor,
      additionalPkgs ? pkgs: [ ],
    }:
    buildFHSEnv {
      name = executableName;

      targetPkgs =
        pkgs:
        (with pkgs; [
          glibc
        ])
        ++ additionalPkgs pkgs;

      extraInstallCommands = ''
        ln -s "${zed-editor}/share" "$out/"
      '';

      runScript = "${zed-editor}/bin/${executableName}";

      passthru = {
        inherit executableName;
        inherit (zed-editor) pname version;
      };

      meta =
        zed-editor.meta
        // {
          description = ''
            Wrapped variant of ${zed-editor.pname} which launches in a FHS compatible environment.
            Should allow for easy usage of extensions without nix-specific modifications.
          '';
        };
    };
in
stdenv.mkDerivation (finalAttrs: {
  pname = "zed-editor-bin";
  inherit version src;

  nativeBuildInputs = [ patchelf makeWrapper ];
  buildInputs = nixDeps;

  phases = [ "unpackPhase" "installPhase" ];

  unpackPhase = ''
    tar xzf "$src"
  '';

  installPhase = ''
    appdir="$(find . -maxdepth 1 -type d -name '*.app' -print -quit)"

    mkdir -p $out/{bin,libexec,share}

    cp "$appdir/bin/zed" $out/bin/
    cp "$appdir/libexec/zed-editor" $out/libexec/
    cp -R "$appdir/share"/* $out/share/

    patchelf \
      --set-interpreter "$(cat $NIX_CC/nix-support/dynamic-linker)" \
      --set-rpath "${lib.makeLibraryPath ([ stdenv.cc.cc ] ++ nixDeps)}" \
      "$out/bin/zed"

    patchelf \
      --set-interpreter "$(cat $NIX_CC/nix-support/dynamic-linker)" \
      --set-rpath "${lib.makeLibraryPath ([ stdenv.cc.cc ] ++ nixDeps)}" \
      "$out/libexec/zed-editor"

    wrapProgram $out/bin/zed \
      --prefix LD_LIBRARY_PATH : ${libPath}

    wrapProgram $out/libexec/zed-editor \
      --prefix LD_LIBRARY_PATH : ${libPath}

    ln -s zed $out/bin/zeditor
  '';

  passthru = {
    fhs = fhs { zed-editor = finalAttrs.finalPackage; };
    fhsWithPackages =
      f:
      fhs {
        zed-editor = finalAttrs.finalPackage;
        additionalPkgs = f;
      };
  };

  meta = with lib; {
    description = "Prebuilt Zed Editor package for x86_64-linux";
    homepage = "https://zed.dev";
    changelog = "https://github.com/zed-industries/zed/releases/tag/v${version}";
    mainProgram = executableName;
    license = licenses.gpl3Only;
    platforms = [ "x86_64-linux" ];
  };
})
