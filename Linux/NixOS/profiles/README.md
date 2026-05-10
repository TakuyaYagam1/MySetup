# Profile Layers

These files are import layers for the single host graph. Runtime package selection
still comes from `mysetup.packages.preset` and `mysetupLib.presets`.

`features.nix` aggregates conditional services that are gated by
`mysetup.features.*` toggles (CTF tools, OmniRouter, observability, Zapret).
Add a new conditional service module to `../services/` and import it here.
