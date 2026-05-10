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
  writeText,
}:

let
  nodejs = nodejs_22;
  buildNpmPackage' = buildNpmPackage.override { inherit nodejs; };

  patchLayout = writeText "patch-layout.py" ''
    import re, pathlib
    p = pathlib.Path("src/app/layout.tsx")
    t = p.read_text()
    t = t.replace("import { Inter } from \"next/font/google\";", "")
    t = re.sub(
      r"const inter = Inter\(\{[^}]+\}\);",
      "const inter = { variable: \"--font-inter\", className: \"\" };",
      t,
      flags=re.DOTALL,
    )
    p.write_text(t)
  '';
in
buildNpmPackage' rec {
  pname = "omnirouter";
  version = "3.6.9";

  src = fetchFromGitHub {
    owner = "diegosouzapw";
    repo = "OmniRoute";
    rev = "v3.6.9";
    hash = "sha256-3Pqg9RP1u2RyL2rmbmnecwWDAUTJcOn2JooC6C9n0GU=";
  };

  npmDepsHash = "sha256-yzu/blkCoQ6bEkJPzNb8BNI2TG79KHs5jIOf7HmdKos=";

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

  # next/font/google fetches Inter from Google Fonts at build time,
  # which fails in the Nix sandbox (no network). Replace with a plain object.
  postPatch = ''
    python3 ${patchLayout}
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
