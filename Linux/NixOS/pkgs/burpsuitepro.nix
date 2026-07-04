{
  lib,
  buildFHSEnv,
  fetchurl,
  jdk21,
  makeDesktopItem,
  unzip,
}:

# To update: change `version`, then run `nix build` once to get the correct hash,
# and replace `burpHash` with the hash printed in the error.
let
  version = "2026.3.3";
  productName = "pro";

  burpHash = "sha256-LX7QwWuHu9ReU58CSU6EOkDSJAHSSQWSFnOJcJZw+CA=";

  burpSrc = fetchurl {
    name = "burpsuite_pro_v${version}.jar";
    urls = [
      "https://portswigger.net/burp/releases/download?product=${productName}&version=${version}&type=Jar"
      "https://web.archive.org/web/https://portswigger.net/burp/releases/download?product=${productName}&version=${version}&type=Jar"
    ];
    hash = burpHash;
  };

  loaderSrc = fetchurl {
    name = "loader.jar";
    url = "https://raw.githubusercontent.com/xiv3r/Burpsuite-Professional/main/loader.jar";
    hash = "sha256-3N8orPNgVUpamNePQDyWzOpQC+JLJ9ArAg4UKCBjfAo=";
  };

  desktopItem = makeDesktopItem {
    name = "burpsuitepro";
    exec = "burpsuitepro";
    icon = "burpsuitepro";
    desktopName = "Burp Suite Professional";
    comment = "Web application security testing platform";
    categories = [
      "Development"
      "Security"
      "System"
    ];
  };
in
buildFHSEnv {
  pname = "burpsuitepro";
  inherit version;

  runScript = "${jdk21}/bin/java --add-opens=java.desktop/javax.swing=ALL-UNNAMED --add-opens=java.base/java.lang=ALL-UNNAMED --add-opens=java.base/jdk.internal.org.objectweb.asm=ALL-UNNAMED --add-opens=java.base/jdk.internal.org.objectweb.asm.tree=ALL-UNNAMED --add-opens=java.base/jdk.internal.org.objectweb.asm.Opcodes=ALL-UNNAMED -javaagent:${loaderSrc} -noverify -jar ${burpSrc}";

  targetPkgs =
    pkgs: with pkgs; [
      alsa-lib
      at-spi2-core
      cairo
      cups
      dbus
      expat
      glib
      gtk3
      gtk3-x11
      jython
      libcanberra-gtk3
      libdrm
      udev
      libxkbcommon
      mesa
      nspr
      nss
      pango
      libx11
      libxcb
      libxcomposite
      libxdamage
      libxext
      libxfixes
      libxrandr
    ];

  extraInstallCommands = ''
    mkdir -p $out/share/pixmaps $out/share/applications

    ${lib.getBin unzip}/bin/unzip -p ${burpSrc} resources/Media/icon64${productName}.png \
      > $out/share/pixmaps/burpsuitepro.png 2>/dev/null || true

    cp -r ${desktopItem}/share/applications $out/share
  '';

  meta = {
    description = "Web application security testing platform";
    homepage = "https://portswigger.net/burp/pro";
    changelog = "https://portswigger.net/burp/releases/professional-community-${
      lib.replaceStrings [ "." ] [ "-" ] version
    }";
    sourceProvenance = with lib.sourceTypes; [ binaryBytecode ];
    license = lib.licenses.unfree;
    mainProgram = "burpsuitepro";
    platforms = [ "x86_64-linux" ];
  };
}
