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
  wahrweltPkgs = pkgs.wahrwelt or (pkgs.mysetup or { });
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
          --replace-fail "Name=ChatGPT" "Name=Codex App" \
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
      cliPackage = wahrweltPkgs.codex;
    };

    home.activation.wahrweltCodexChromeBridge = lib.hm.dag.entryAfter [ "writeBoundary" ] ''
      runtimeAlias="$HOME/.codex/plugins/cache/openai-bundled/chrome/.codex-linux-runtime"
      runtimeTarget="$HOME/.codex/plugins/linux-runtime-cache/openai-bundled/chrome/latest"

      if [ -L "$runtimeAlias" ]; then
        if [ "$(readlink "$runtimeAlias")" != "$runtimeTarget" ]; then
          echo "Wahrwelt Codex Chrome bridge collision: $runtimeAlias is an unmanaged symlink" >&2
          exit 1
        fi
      elif [ -e "$runtimeAlias" ]; then
        echo "Wahrwelt Codex Chrome bridge collision: $runtimeAlias already exists" >&2
        exit 1
      else
        $DRY_RUN_CMD mkdir -p "$(dirname "$runtimeAlias")"
        $DRY_RUN_CMD ln -s "$runtimeTarget" "$runtimeAlias"
      fi
    '';
  };
}
