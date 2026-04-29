{ pkgs, ... }:

{
  home.packages = (with pkgs; [
    # Editors / IDEs
    vscode
    code-cursor
    jetbrains.goland
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
  ]);
}
