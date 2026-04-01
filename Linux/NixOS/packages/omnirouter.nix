{ lib, buildNpmPackage, fetchFromGitHub, python3, pkg-config, libsecret, libx11, stdenv, nodejs, python311, vips, gnumake, gcc }:

buildNpmPackage rec {
  pname = "omnirouter";
  version = "3.4.0";

  src = fetchFromGitHub {
    owner = "diegosouzapw";
    repo = "OmniRoute";
    rev = "v3.4.0"; 
    hash = "sha256-urgUKcr9pgK+nR5qLom8OFu2WM+BaOgguFy6J0wI3yQ=";
  };

  npmDepsHash = "sha256-e1yNVD+/5l6Vc0KqE5/LjInVid//PdkO51RUrh4EdXI=";

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
