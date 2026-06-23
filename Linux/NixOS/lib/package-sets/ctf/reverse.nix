{ pkgs-stable }:

let
  reversePython = pkgs-stable.python3.withPackages (
    ps: with ps; [
      capstone
      keystone-engine
      lief
      pefile
      r2pipe
      unicorn
      z3-solver
    ]
  );
  reversePythonBin = pkgs-stable.writeShellScriptBin "rev-python" ''
    exec ${reversePython}/bin/python3 "$@"
  '';
in
with pkgs-stable;
[
  binaryen
  binutils
  binwalk
  capstone
  clamav
  cutter
  detect-it-easy
  frida-tools
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
  reversePythonBin
  rizin
  semgrep
  strace
  upx
  wabt
  yara
  z3
]
