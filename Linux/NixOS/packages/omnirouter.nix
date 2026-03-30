{ lib, buildNpmPackage, fetchFromGitHub, python3, pkg-config, libsecret, libx11, stdenv, nodejs, python311, vips, gnumake, gcc }:

buildNpmPackage rec {
  pname = "omnirouter";
  version = "main";

  src = fetchFromGitHub {
    owner = "diegosouzapw";
    repo = "OmniRoute";
    rev = "main"; 
    hash = "sha256-vRaSLrtfM80Fh8bVqoBQify4dK9ZVmJOFLcCJNIqmy0=";
  };

  npmDepsHash = "sha256-8NWOjucobYgeOQ0g3CQEGNO007gbjJp/s3YI5ipWZIQ=";

  # Need node-gyp and python to build native module like sharp
  nativeBuildInputs = [
    python311
    python3
    pkg-config
    nodejs
    gnumake
    gcc
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

  preBuild = ''
    export CYPRESS_INSTALL_BINARY=0
    export HUSKY_SKIP_INSTALL=1
    npm install --legacy-peer-deps
    npm install node-gyp --save-dev
  '';

  buildPhase = ''
    export NODE_ENV=production
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
