{
  inputs,
  lib,
  pkgs,
  wahrwelt,
  wahrweltLib,
  ...
}:

let
  system = pkgs.stdenv.hostPlatform.system;
  codexDesktopBase = inputs.codex-desktop-linux.packages.${system}.codex-desktop;
  codexDesktopIcon = "${codexDesktopBase}/share/icons/hicolor/256x256/apps/codex-desktop.png";
  codexDesktopPackage = pkgs.symlinkJoin {
    name = "${codexDesktopBase.name}-wahrwelt";
    paths = [ codexDesktopBase ];
    postBuild = ''
      desktopFile="$out/share/applications/codex-desktop.desktop"
      if [ -e "$desktopFile" ]; then
        target="$(readlink -f "$desktopFile")"
        rm -f "$desktopFile"
        substitute "$target" "$desktopFile" \
          --replace-fail "Name=ChatGPT Community" "Name=Codex App" \
          --replace-fail "Icon=codex-desktop" "Icon=${codexDesktopIcon}" \
          --replace-fail "${codexDesktopBase}/bin/codex-desktop" "$out/bin/codex-desktop"
      fi
    '';
    meta = codexDesktopBase.meta or { };
  };
in
{
  config = lib.mkIf (wahrweltLib.presets.developerOrMore wahrwelt) {
    programs.codexDesktopLinux = {
      enable = true;
      package = codexDesktopPackage;
    };
  };
}
