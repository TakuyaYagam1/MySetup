{ lib }:

args:

let
  hasLegacyArgument = args ? hashedPassword;
  hasCurrentArgument = args ? hashedPasswordFile;
  legacyModule = if hasLegacyArgument then args.hashedPassword else null;
  passthrough = builtins.removeAttrs args [ "hashedPassword" ];
in
assert lib.assertMsg (!(hasLegacyArgument && hasCurrentArgument))
  "Wahrwelt v1_to_v2 compatibility rejects simultaneous hashedPassword and hashedPasswordFile arguments";
passthrough
// lib.optionalAttrs hasLegacyArgument {
  # One transitional build must still import the exact historical module so
  # an existing system can activate the service that externalizes it. The
  # v1_to_v2 system migration removes this module from the installed wrapper.
  extraModules = (args.extraModules or [ ]) ++ lib.optional (legacyModule != null) legacyModule;
}
