{ lib }:

let
  inherit (import ./preset-names.nix) presetNames defaultPreset;

  orderedPresets = presetNames;

  indexOf =
    value:
    let
      matches = lib.imap0 (index: preset: if preset == value then index else null) orderedPresets;
      indexes = builtins.filter (index: index != null) matches;
    in
    if indexes == [ ] then -1 else builtins.head indexes;

  fromConfig = cfg: cfg.packages.preset or defaultPreset;
  atLeast =
    minimum: cfg:
    let
      presetIndex = indexOf (fromConfig cfg);
      minimumIndex = indexOf minimum;
    in
    presetIndex >= minimumIndex && minimumIndex >= 0;
in
{
  inherit orderedPresets fromConfig atLeast;

  minimalOrMore = atLeast "minimal";
  desktopOrMore = atLeast "desktop";
  developerOrMore = atLeast "developer";
  personal = cfg: fromConfig cfg == "personal";
  oneOf = presets: cfg: lib.elem (fromConfig cfg) presets;
}
