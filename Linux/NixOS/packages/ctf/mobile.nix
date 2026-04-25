{ pkgs-stable, ... }:

# Mobile reverse engineering: APK/IPA analysis, Java/Dalvik bytecode tooling.

{
  environment.systemPackages = with pkgs-stable; [
    apktool
    bytecode-viewer
    dex2jar
    jadx
    quark-engine
  ];
}
