<!--
Thanks for the PR! Please fill in the sections below.

PR title MUST follow Conventional Commits:
  <type>(<optional scope>): <description>     # lowercase, no markdown
  types: feat | fix | chore | refactor | docs | test

Examples:
  feat(installer): add doctor command for nix store check
  fix(hypr/keybinds): correct workspace switcher chord
  docs(readme): clarify locale region prompt
-->

## What

<!-- Brief summary of the change. One or two sentences. -->

## Why

<!--
Motivation, context, related issue (Fixes #123 / Refs #456),
upstream change being followed, constraint being addressed.
-->

## Testing

<!--
Describe what you actually ran, on what hardware/VM, and what you observed.
"Tested" with no details is not enough.
-->

- [ ] `make -C Linux all` passes locally (required for any change under `Linux/**`)
- [ ] Manually tested on real hardware or VM - specify:
- [ ] Breaking change documented above (or N/A)

## Checklist

- [ ] Commit messages follow Conventional Commits, lowercase, no AI attribution
- [ ] No secrets, tokens, or personal absolute paths in the diff
- [ ] Relevant README updated if user-facing behavior changed
- [ ] New dependencies justified (see CONTRIBUTING.md "Libraries first")
