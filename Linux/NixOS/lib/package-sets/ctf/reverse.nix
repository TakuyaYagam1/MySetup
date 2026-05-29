{ pkgs-stable }:

let
  reversePython = pkgs-stable.python3.withPackages (
    ps: with ps; [
      # Temporarily disabled on 26.05: angr is 9.2.193, but nixpkgs still
      # provides some required angr components as 9.2.154 (pyvex/cle/archinfo).
      # Wait for the Python package set to become consistent before re-enabling.
      # angr
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
