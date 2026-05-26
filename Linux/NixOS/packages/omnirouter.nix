{
  lib,
  buildNpmPackage,
  fetchFromGitHub,
  python3,
  pkg-config,
  libsecret,
  libx11,
  stdenv,
  nodejs_22,
  python311,
  vips,
  gnumake,
  gcc,
  perl,
}:

let
  nodejs = nodejs_22;
  buildNpmPackage' = buildNpmPackage.override { inherit nodejs; };
in
buildNpmPackage' rec {
  pname = "omnirouter";
  version = "3.8.3";

  src = fetchFromGitHub {
    owner = "diegosouzapw";
    repo = "OmniRoute";
    rev = "v3.8.3";
    hash = "sha256-4GM5fSKNwzeEFcOcDqfHWyPZsQWaINYf1hqs7o6wV2M=";
  };

  npmDepsHash = "sha256-LxM0499IyfL1p8ngEGDl0GsSTpCqG3EDt0RMhv6TqpQ=";

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
    vips
  ]
  ++ lib.optionals stdenv.isLinux [
    libx11
  ];

  env = {
    NEXT_TELEMETRY_DISABLED = "1";
    npm_config_arch = stdenv.hostPlatform.parsed.cpu.name;
    SHARP_IGNORE_GLOBAL_LIBVIPS = "0";
  };

  doCheck = false;

  postPatch = ''
    if [ -f src/app/layout.tsx ]; then
      ${perl}/bin/perl -i -0pe '
        s|import \{ Inter \} from "next/font/google";\n?||g;
        s|const inter = Inter\(\{[^}]*\}\);|const inter = { variable: "--font-inter", className: "" };|gs;
      ' src/app/layout.tsx
    fi
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
    export DATA_DIR="\''${DATA_DIR:-\''${HOME:-/var/lib/omnirouter}}"
    export APP_LOG_FILE_PATH="\''${APP_LOG_FILE_PATH:-\''${DATA_DIR}/logs/application/app.log}"
    export NODE_ENV=production
    export OMNIROUTE_NO_UPDATE_NOTIFIER="\''${OMNIROUTE_NO_UPDATE_NOTIFIER:-1}"
    exec ${lib.getExe nodejs} scripts/dev/run-next.mjs start "\$@"
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
