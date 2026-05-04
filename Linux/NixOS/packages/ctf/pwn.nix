{ config, lib, pkgs-stable, ... }:

# Binary exploitation: debugging, fuzzing, exploit development, exploitation frameworks.

{
  config = lib.mkIf (config.var.features.ctfTools or false) {
  environment.systemPackages = with pkgs-stable; [
    aflplusplus
    armitage
    atomic-operator
    bed
    certi
    checksec
    delve
    exploitdb
    gdb
    gef
    havoc
    hb-honeypot
    honggfuzz
    linux-exploit-suggester
    metasploit
    msfpc
    payloadsallthethings
    pwncat
    pwntools
    radamsa
    ropgadget
    routersploit
    shellnoob
    sigma-cli
    sploitscan
    spike
    unix-privesc-check
    vulnix
  ];
  };
}
