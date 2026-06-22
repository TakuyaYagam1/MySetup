// Package defaults centralises tunable constants that several packages share
// (cache substitutes, mkpasswd rounds, staging tmp pattern). Keeping them in
// one place lets reviewers audit security-sensitive defaults without scanning
// the whole codebase, and makes future overrides via flags or env trivial.
package defaults

// ShaCryptRounds is the iteration count passed to mkpasswd for hashed-password.nix.
// 656000 matches the nixpkgs default used by NixOS 24.05+ and is intentionally
// expensive; do not lower without a security review.
const ShaCryptRounds = 656000

// StagingTempPattern is the os.MkdirTemp pattern for the apply staging tree.
// Includes the project name so leftover dirs after a kill -9 are easy to spot.
const StagingTempPattern = "mysetup-nixos-*"

// ExtraSubstituters lists the binary cache hosts that nixos-rebuild trusts in
// addition to cache.nixos.org. Order matches the precedence: faster mirrors
// first, then specialised ones.
var ExtraSubstituters = []string{
	"https://nix-community.cachix.org",
	"https://hyprland.cachix.org",
	"https://quickshell.cachix.org",
	"https://numtide.cachix.org",
}

// ExtraTrustedPublicKeys is the matching set of public keys for ExtraSubstituters.
// Indices need not align: nix matches keys to substitutes by hostname inside
// the key string itself.
var ExtraTrustedPublicKeys = []string{
	"nix-community.cachix.org-1:mB9FSh9qf2dCimDSUo8Zy7bkq5CX+/rkCWyvRCYg3Fs=",
	"hyprland.cachix.org-1:a7pgxzMz7+chwVL3/pzj6jIBMioiJM7ypFP8PwtkuGc=",
	"quickshell.cachix.org-1:tjWMR3PQd01gN6YtjSRUdHHHUgrSLFIgwqrCQjFXVOU=",
	"numtide.cachix.org-1:2ps1kLBUWjxIneOy1Ik6cQjb41X0iXVXeHigGmycPPE=",
}
