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
  version = "3.8.12";

  src = fetchFromGitHub {
    owner = "diegosouzapw";
    repo = "OmniRoute";
    rev = "v3.8.12";
    hash = "sha256-mmOc7j0d9PwHTqkSg/815dMJ/9QS15WYwJFRaPKkPNk=";
  };

  npmDepsHash = "sha256-EUbpXmSc635TQeRH5fpRF0rW4qvXf50kSK3QPom5Zfw=";

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
    # CPU binaries are bundled; skip optional CUDA downloads from NuGet.
    ONNXRUNTIME_NODE_INSTALL = "skip";
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
    rm -f $out/share/omnirouter/.env

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

    cat > $out/bin/omniroute <<EOF
    #!/bin/sh
    cd $out/share/omnirouter
    export DATA_DIR="\''${DATA_DIR:-\''${HOME:-/var/lib/omnirouter}}"
    export NODE_ENV=production
    export OMNIROUTE_NO_UPDATE_NOTIFIER="\''${OMNIROUTE_NO_UPDATE_NOTIFIER:-1}"
    exec ${lib.getExe nodejs} bin/omniroute.mjs "\$@"
    EOF

    chmod +x $out/bin/omniroute

    runHook postInstall
  '';

  meta = with lib; {
    description = "OmniRouter: A versatile LLM router";
    homepage = "https://github.com/diegosouzapw/OmniRoute";
    license = licenses.mit;
  };
}
