{ end4Lib, lib, ... }:

{
  # Keep end4 runtime commands available in every session without merging a
  # large shell-only package set into home-manager-path.
  home.sessionPath = lib.mkAfter [
    (lib.makeBinPath end4Lib.runtime.binPackages)
  ];
}
