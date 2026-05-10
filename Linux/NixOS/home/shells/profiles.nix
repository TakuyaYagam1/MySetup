let
  layout = import ../../lib/layout.nix {
    nixosRoot = ../..;
  };
  shellRuntimeManifest = builtins.fromJSON (
    builtins.readFile (layout.installer + "/internal/shellruntime/manifest.json")
  );

  ordered = shellRuntimeManifest.profiles;
in
{
  inherit (shellRuntimeManifest) defaultProfile;
  inherit ordered;

  byId = builtins.listToAttrs (
    map (profile: {
      name = profile.id;
      value = profile;
    }) ordered
  );
}
