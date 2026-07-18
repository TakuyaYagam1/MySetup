# Firefox 102 ESR - the final release before Firefox 103 enabled Total Cookie
# Protection (dFPI storage partitioning) by default, so third-party localStorage
# is not isolated per top-level site here. For testing storage-partitioning /
# cross-tab exfil behavior against pre-TCP browsers, not for daily browsing.
{
  lib,
  stdenv,
  buildFHSEnv,
  fetchurl,
  writeShellScript,
}:

let
  version = "102.15.1esr";

  src = fetchurl {
    url = "https://archive.mozilla.org/pub/firefox/releases/${version}/linux-x86_64/en-US/firefox-${version}.tar.bz2";
    hash = "sha256-Pk5pz6gS13bYKRYgjHzlzs3C56bGxrR1Pz3f+cmOXnM=";
  };

  unwrapped = stdenv.mkDerivation {
    pname = "firefox-legacy-unwrapped";
    inherit version src;
    dontStrip = true;
    dontPatchELF = true;
    dontFixup = true;
    installPhase = ''
      mkdir -p $out
      cp -r . $out
    '';
  };

  launcher = writeShellScript "firefox-legacy-launcher" ''
    profile_dir="$HOME/.mozilla/firefox-legacy-102esr"
    mkdir -p "$profile_dir"
    exec ${unwrapped}/firefox -no-remote -profile "$profile_dir" "$@"
  '';
in
buildFHSEnv {
  pname = "firefox-legacy";
  inherit version;

  runScript = "${launcher}";

  targetPkgs =
    pkgs: with pkgs; [
      alsa-lib
      atk
      at-spi2-core
      cairo
      cups
      dbus
      dbus-glib
      fontconfig
      freetype
      gdk-pixbuf
      glib
      gtk3
      libGL
      libpulseaudio
      libxkbcommon
      mesa
      nspr
      nss
      pango
      libX11
      libXcomposite
      libXcursor
      libXdamage
      libXext
      libXfixes
      libXi
      libXrandr
      libXrender
      libXt
      libXtst
      pciutils
      udev
    ];

  meta = {
    description = "Firefox 102 ESR - last release before default-on Total Cookie Protection, for storage-partitioning test coverage";
    homepage = "https://www.mozilla.org/firefox/";
    changelog = "https://www.firefox.com/en-US/firefox/102.15.1esr/releasenotes/";
    license = lib.licenses.unfree;
    mainProgram = "firefox-legacy";
    platforms = [ "x86_64-linux" ];
  };
}
