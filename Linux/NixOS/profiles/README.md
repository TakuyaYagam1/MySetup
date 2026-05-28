# Profile Layers

These files are import layers for the single host graph. Runtime package selection
still comes from `mysetup.packages.preset` and `mysetupLib.presets`.

`features.nix` aggregates conditional feature modules that are gated by
`mysetup.features.*` toggles (CTF tools, OmniRouter, observability, Zapret).
That includes service modules and feature-scoped package modules such as MCP
helpers for CTF tooling. Add new feature-gated modules here.
