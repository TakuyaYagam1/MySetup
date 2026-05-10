{
  pkgs,
  lib,
  dotfilesLib,
}:

{
  # Build a home-manager activation hook for seeding mutable JSON config files.
  # `dirs` are mkdir -p'd before `body` runs. `body` typically calls
  # `seed_json_object`, provided by dotfilesLib.mutableJsonShellHelpers.
  mkSeedActivation =
    {
      dirs ? [ ],
      after ? [ "writeBoundary" ],
      body,
    }:
    lib.hm.dag.entryAfter after ''
      ${dotfilesLib.mutableJsonShellHelpers}

      ${lib.concatMapStringsSep "\n" (d: ''$DRY_RUN_CMD ${pkgs.coreutils}/bin/mkdir -p "${d}"'') dirs}

      ${body}
    '';
}
