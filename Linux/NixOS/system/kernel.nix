{ lib, pkgs, ... }:

{
  # Pinned to the 6.12 LTS branch rather than linuxPackages_latest: the
  # out-of-tree amneziawg module (system-tools.nix) fails to build against
  # newer kernels (7.x removed the ipv6_stub symbol it depends on).
  boot.kernelPackages = lib.mkDefault pkgs.linuxPackages_6_12;
}
