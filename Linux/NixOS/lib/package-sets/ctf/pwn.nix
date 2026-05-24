{ pkgs-stable }:

let
  pwnPython = pkgs-stable.python3.withPackages (
    ps: with ps; [
      capstone
      pwntools
      pyelftools
      requests
      ropper
      unicorn
      z3-solver
    ]
  );
  pwnPythonBin = pkgs-stable.writeShellScriptBin "pwn-python" ''
    exec ${pwnPython}/bin/python3 "$@"
  '';
  gccMultilibBin = pkgs-stable.writeShellScriptBin "gcc-multilib" ''
    exec ${pkgs-stable.gcc_multi}/bin/gcc "$@"
  '';
  gxxMultilibBin = pkgs-stable.writeShellScriptBin "g++-multilib" ''
    exec ${pkgs-stable.gcc_multi}/bin/g++ "$@"
  '';
in
with pkgs-stable;
[
  aflplusplus
  armitage
  atomic-operator
  bed
  certi
  checksec
  delve
  exploitdb
  gccMultilibBin
  gdb
  gef
  gxxMultilibBin
  havoc
  hb-honeypot
  honggfuzz
  linux-exploit-suggester
  metasploit
  msfpc
  one_gadget
  patchelf
  payloadsallthethings
  pwnPythonBin
  pwncat
  pwninit
  pwntools
  python3Packages.ropper
  radamsa
  ropgadget
  routersploit
  rr
  shellnoob
  sigma-cli
  sploitscan
  spike
  unix-privesc-check
  vulnix
]
