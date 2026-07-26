# Profile Layers

These files are import layers for the single host graph. Runtime package selection
still comes from `wahrwelt.packages.preset` and `wahrweltLib.presets`.

`features.nix` aggregates conditional feature modules that are gated by
`wahrwelt.features.*` toggles (CTF tools, OmniRouter, Portainer, observability).
That includes service modules and feature-scoped package modules. Add new
feature-gated modules here.
