# Security Policy

## Supported versions

| Version | Supported |
| --- | --- |
| `main` | ✅ Rolling, supported |
| Older commits | ⚠️ Best effort only |

This is a personal dotfiles repo with a single maintainer. There is no LTS branch.
Always reproduce on `main` before reporting.

## Reporting a vulnerability

**Do not open a public issue for security problems.**

Use [GitHub Security Advisories](https://github.com/TakuyaYagam1/MySetup/security/advisories/new)
to report privately. That is the preferred channel - it creates an encrypted, maintainer-only
thread and lets us coordinate a fix before disclosure.

If you cannot use the advisory form, email `skr1ms13666@gmail.com` with `[mysetup-security]`
in the subject line. PGP is not currently published.

Please include:

- A description of the issue and its impact.
- Affected paths or modules (e.g. `Linux/installer/...`, `Linux/NixOS/...`).
- Reproduction steps or a minimal PoC.
- Your assessment of severity, if you have one.
- Whether the issue is already public somewhere (CVE, mailing list, blog).

## Scope

**In scope:**

- The Go installer under `Linux/installer/**`.
- NixOS modules and home-manager configs under `Linux/NixOS/**`.
- Dotfiles and shell scripts shipped from this repo.
- The Windows installer and YASB/Komorebi configs under `Windows/**`.
- CI workflows under `.github/workflows/**`.

**Out of scope:**

- Vulnerabilities in upstream projects (Hyprland, Quickshell, caelestia-shell,
  noctalia, end-4 dots-hyprland, Komorebi, YASB, Zen Browser, etc.).
  Report those to the upstream maintainers directly.
- Vulnerabilities in NixOS itself or in any package fetched from `nixpkgs`.
- Issues that require an already-compromised machine to exploit.
- Issues that depend on user-supplied malicious config files outside the repo.

## Response time

Single maintainer, best effort:

- Acknowledgement within **7 days** of receiving the report.
- Initial assessment and severity classification within **14 days**.
- Fix or mitigation plan within **30 days** for high/critical issues. Lower severities
  may be deferred and tracked in the advisory.

If the issue is being actively exploited, mark it clearly in the report so it can be
prioritized.

## Disclosure

Once a fix is available and published on `main`, the advisory will be published and
credit given to the reporter unless they request anonymity.
