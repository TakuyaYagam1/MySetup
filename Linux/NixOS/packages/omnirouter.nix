{ lib, buildNpmPackage, fetchFromGitHub, python3, pkg-config, libsecret, libx11, stdenv, nodejs_22, python311, vips, gnumake, gcc, perl }:

let
  nodejs = nodejs_22;
  buildNpmPackage' = buildNpmPackage.override { inherit nodejs; };
in
buildNpmPackage' rec {
  pname = "omnirouter";
  version = "3.5.3";

  src = fetchFromGitHub {
    owner = "diegosouzapw";
    repo = "OmniRoute";
    rev = "v3.5.3";
    hash = "sha256-I34fgrjYhm+pmIXd/efG/YMsZLN2lHupATc05Vq/8Q8=";
  };

  npmDepsHash = "sha256-4B1hHblMXI5gp9RUE4FrvQe0XUOV85UiAUd+uLHXDUQ=";

  # Need node-gyp and python to build native module like sharp
  nativeBuildInputs = [
    python311
    python3
    pkg-config
    nodejs
    gnumake
    gcc
    perl
  ];

  buildInputs = [
    libsecret
    vips # Sharp depends on libvips
  ] ++ lib.optionals stdenv.isLinux [
    libx11
  ];

  env = {
    NEXT_TELEMETRY_DISABLED = "1";
    npm_config_arch = stdenv.hostPlatform.parsed.cpu.name;
    SHARP_IGNORE_GLOBAL_LIBVIPS = "0"; 
  };

  # Skip tests because they might fail in sandbox
  doCheck = false;

  # next/font/google tries to fetch Inter from Google Fonts at build time,
  # which fails in the Nix sandbox (no network). Replace with a CSS variable fallback.
  postPatch = ''
    sed -i 's|import { Inter } from "next/font/google";||' src/app/layout.tsx
    perl -i -0pe 's|const inter = Inter\(\{[^}]+\}\);|const inter = { variable: "--font-inter" };|s' src/app/layout.tsx
  '';

  buildPhase = ''
    export NODE_ENV=production
    export CYPRESS_INSTALL_BINARY=0
    export HUSKY_SKIP_INSTALL=1
    npm run build
  '';

  installPhase = ''
    runHook preInstall
    
    mkdir -p $out/share/omnirouter
    cp -a . $out/share/omnirouter/
    
    mkdir -p $out/bin
    cat > $out/bin/omnirouter <<EOF
    #!/bin/sh
    cd $out/share/omnirouter
    export NODE_ENV=production
    exec ${lib.getExe nodejs} scripts/run-next.mjs start "\$@"
    EOF
    
    chmod +x $out/bin/omnirouter
    
    runHook postInstall
  '';

  meta = with lib; {
    description = "OmniRouter: A versatile LLM router";
    homepage = "https://github.com/diegosouzapw/OmniRoute";
    license = licenses.mit;
  };
}
