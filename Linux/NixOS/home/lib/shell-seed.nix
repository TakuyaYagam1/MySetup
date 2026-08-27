{
  lib,
  dotfilesLib,
}:

{
  # Build a home-manager activation hook for seeding mutable JSON config files.
  # `dirs` are created through the no-follow mutable seed helper before `body` runs.
  # `seed_json_object`, provided by dotfilesLib.mutableJsonShellHelpers.
  mkSeedActivation =
    {
      dirs ? [ ],
      after ? [ "writeBoundary" ],
      body,
    }:
    lib.hm.dag.entryAfter after ''
      ${dotfilesLib.mutableJsonShellHelpers}

      ${lib.concatMapStringsSep "\n" (d: ''ensure_real_directory "${d}" || exit $?'') dirs}

      ${body}
    '';
}
