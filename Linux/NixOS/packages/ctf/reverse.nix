{ config, lib, pkgs-stable, ... }:

# Reverse engineering: disassembly, decompilation, deobfuscation, malware analysis.

{
  config = lib.mkIf (config.var.features.ctfTools or false) {
  environment.systemPackages = with pkgs-stable; [
    binutils
    binwalk
    capstone
    clamav
    cutter
    detect-it-easy
    ghidra-bin
    goresym
    hexyl
    hotpatch
    imhex
    ltrace
    lynis
    nasm
    passdetective
    patchelf
    radare2
    rizin
    semgrep
    strace
    upx
    yara
  ];
  };
}
