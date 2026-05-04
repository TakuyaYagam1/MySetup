{ config, lib, pkgs-stable, ... }:

# Mobile reverse engineering: APK/IPA analysis, Java/Dalvik bytecode tooling.

{
  config = lib.mkIf (config.var.features.ctfTools or false) {
  environment.systemPackages = with pkgs-stable; [
    apktool
    bytecode-viewer
    dex2jar
    jadx
    quark-engine
  ];
  };
}
