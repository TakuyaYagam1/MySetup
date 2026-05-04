{ lib, pkgs, var, ... }:

let
  preset = var.packagePreset or "personal";
  enabled = lib.elem preset [ "developer" "personal" ];
in
{
  config = lib.mkIf enabled {
    home.packages = (with pkgs; [
    # Editors / IDEs
    vscode
    code-cursor
    qtcreator

    # AI assistants
    antigravity
    gemini-cli
    claude-code
    codex
    opencode
    opencode-claude-auth
    opencode-desktop

    # Mobile / cross-platform
    flutter
    android-studio-full
    android-tools
    scrcpy
    ]) ++ lib.optionals (!(var.features.russiaMode or false)) [
      pkgs.jetbrains.goland
    ];
  };
}
