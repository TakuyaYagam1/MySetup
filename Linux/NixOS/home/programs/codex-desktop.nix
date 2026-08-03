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
  codexChromeBridge = pkgs.writeShellApplication {
    name = "wahrwelt-codex-chrome-bridge";
    runtimeInputs = [ pkgs.coreutils ];
    text = ''
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
        mkdir -p "$(dirname "$runtimeAlias")"
        ln -s "$runtimeTarget" "$runtimeAlias"
      fi
    '';
  };
  codexChromeBridgeWatcher = pkgs.writeShellApplication {
    name = "wahrwelt-codex-chrome-bridge-watcher";
    runtimeInputs = [ pkgs.coreutils pkgs.inotify-tools ];
    text = ''
      watchRoot="$HOME/.codex/plugins/cache/openai-bundled"
      watchChrome="$watchRoot/chrome"
      mkdir -p "$watchRoot"
      ${codexChromeBridge}/bin/wahrwelt-codex-chrome-bridge

      while true; do
        if [ ! -d "$watchChrome" ]; then
          ${pkgs.inotify-tools}/bin/inotifywait --quiet --event moved_to --event create "$watchRoot"
          ${codexChromeBridge}/bin/wahrwelt-codex-chrome-bridge
          continue
        fi

        while IFS='|' read -r watchedPath changedName; do
          changedPath="$watchedPath$changedName"
          if [ "$changedPath" = "$watchChrome/.codex-linux-runtime" ]; then
            continue
          fi
          if [ "$changedPath" = "$watchChrome" ] || [[ "$changedPath" == "$watchChrome/"* ]]; then
            ${codexChromeBridge}/bin/wahrwelt-codex-chrome-bridge
          fi
        done < <(
          ${pkgs.inotify-tools}/bin/inotifywait --monitor --quiet --format '%w|%f' \
            --event moved_to --event create "$watchRoot" "$watchChrome"
        )
      done
    '';
  };
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
      $DRY_RUN_CMD ${codexChromeBridge}/bin/wahrwelt-codex-chrome-bridge
    '';

    systemd.user.services.wahrwelt-codex-chrome-bridge = {
      Unit.Description = "Repair the Codex Chrome native-messaging bridge";
      Service = {
        Type = "simple";
        ExecStart = "${codexChromeBridgeWatcher}/bin/wahrwelt-codex-chrome-bridge-watcher";
        Restart = "always";
        RestartSec = "5s";
      };
      Install.WantedBy = [ "default.target" ];
    };
  };
}
