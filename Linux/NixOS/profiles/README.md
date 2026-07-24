# Profile Layers

These files are import layers for the single host graph. Runtime package selection
still comes from `mysetup.packages.preset` and `mysetupLib.presets`.

`features.nix` aggregates conditional feature modules that are gated by
`mysetup.features.*` toggles (CTF tools, OmniRouter, Portainer, observability).
That includes service modules and feature-scoped package modules. Add new
feature-gated modules here.
