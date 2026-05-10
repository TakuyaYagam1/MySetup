{ pkgs-stable }:

let
  importCategory = name: import (./ctf + "/${name}.nix") { inherit pkgs-stable; };
in
{
  ctfCloud = importCategory "cloud";
  ctfCrypto = importCategory "crypto";
  ctfForensics = importCategory "forensics";
  ctfHardware = importCategory "hardware";
  ctfMisc = importCategory "misc";
  ctfMobile = importCategory "mobile";
  ctfNetwork = importCategory "network";
  ctfOsint = importCategory "osint";
  ctfPwn = importCategory "pwn";
  ctfReverse = importCategory "reverse";
  ctfStego = importCategory "stego";
  ctfWeb = importCategory "web";
}
