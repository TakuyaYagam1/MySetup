# NixOS Configuration

NixOS + Hyprland configuration managed by the `mysetup` Go TUI/CLI installer.
Hyprland itself is configured through Lua for Hyprland 0.55+; this setup no
longer maintains a `hyprland.conf` fallback.

The old `install.sh` and `dots/install.fish` entry points are gone. System
settings, host-local generated files, and user dotfiles are applied through the
flake app.

## Quick Start

Already booted into NixOS and the host has `/etc/nixos/hardware-configuration.nix`?
The single command below opens the TUI installer, lets you pick a preset, and
applies everything:

```bash
nix run "path:$PWD"
```

Run this from the cloned repository root. The detailed forms (GitHub-direct,
pinned commit/branch, refresh) are documented in the next section.

## Install / Reconfigure

Run from the repository root:

```bash
nix run "path:$PWD"
```

Use the repository root flake for local runs. It exposes the installer as the
default app and keeps the public flake inputs limited to the reusable shell
stack.

Or run directly from GitHub without cloning first. This opens the interactive
TUI and uses the stable `main` branch by default:

```bash
nix run --refresh 'github:TakuyaYagam1/MySetup'
```

This is the normal thin apply path. Nix fetches the repository source into
`/nix/store`, the wrapped installer points `MYSETUP_REPO_ROOT` at that immutable
source, then the installer writes a small `/etc/nixos` wrapper that tracks
`github:TakuyaYagam1/MySetup/main?dir=Linux/NixOS` by default.

To test the latest fixes before they are merged to `main`, run the TUI from the
`dev` branch and select `General -> MySetup channel -> development` before
applying:

```bash
nix run --refresh 'github:TakuyaYagam1/MySetup/dev?dir=Linux/NixOS#mysetup' -- tui
```

The TUI `General` section includes `MySetup channel`. Keep `stable` to track the
`main` branch after install, or select `development` to generate a wrapper that
tracks `github:TakuyaYagam1/MySetup/dev?dir=Linux/NixOS`.

Nix caches the resolved source for ~1 hour (`tarball-ttl` default). If you just
pushed a commit and want the new HEAD right now, keep `--refresh` in the command.
For a cached stable TUI launch, `--refresh` is optional:

```bash
nix run 'github:TakuyaYagam1/MySetup'
```

For reproducible installs, pin a commit, branch, or tag:

```bash
# Specific commit (full SHA):
nix run 'github:TakuyaYagam1/MySetup/<commit>'

# Specific branch (e.g. dev):
nix run 'github:TakuyaYagam1/MySetup/dev'
```

Fresh install through `/mnt` is not supported in v1. Run this on an already
booted NixOS system with an existing `/etc/nixos/hardware-configuration.nix`.
On first adoption from a stock NixOS/KDE install, MySetup replaces the active
`/etc/nixos/configuration.nix` with a clean host-local override file. The old
tree is kept in the `/etc/nixos.bak.<timestamp>.<pid>.<n>` backup made before
writing `/etc/nixos`. VPN/proxy tools such as Amnezia are only a network
workaround before running MySetup; they are not required in the bootstrap
`configuration.nix`.

Nix may ask whether to allow the flake app to use additional substituters. That
is expected for this config; the installer and checks use these binary caches:

- `https://nix-community.cachix.org`
- `https://hyprland.cachix.org`
- `https://quickshell.cachix.org`
- `https://numtide.cachix.org`

The first full build can still be large because this config includes multiple
Wayland shells, Qt/QML packages, desktop apps, and optional CTF/development
stacks.

MySetup's low-RAM bootstrap path uses conservative build settings so 8 GB RAM
machines do not get killed by OOM during rebuilds: `max-jobs = 1`, `cores = 2`,
zram swap is enabled, and the existing `/var/lib/swapfile` remains a disk
fallback. On Btrfs, NixOS creates the managed swapfile with
`btrfs filesystem mkswapfile`, so manual CoW workarounds are not needed for that
declarative swapfile.

For the very first remote installer launch, pass the same limits before the app
separator because the installer cannot control the resources used to build
itself. This is a non-interactive apply path and will not open the TUI:

```bash
nix run --refresh --option max-jobs 1 --option cores 2 \
  'github:TakuyaYagam1/MySetup?dir=Linux/NixOS#mysetup' -- \
  apply --lock-mode managed
```

Installed machines with more RAM should override the low-RAM values in
`host-vars.nix`. For a 16-thread machine with about 30 GB RAM, use a bounded pair
such as `maxJobs = 4; cores = 4;`. If the active system is still using the old
low-RAM Nix daemon config, run the first switch with explicit options:

```bash
sudo nixos-rebuild switch --flake /etc/nixos#NixOS --option max-jobs 4 --option cores 4
```

## State And Secrets

The installer writes machine-local state to:

```text
/etc/nixos/mysetup/state.json
```

The in-progress TUI draft is stored at:

```text
$XDG_STATE_HOME/mysetup/draft.json
```

Plain passwords are never stored in state or draft JSON.

Managed secret files:

```text
/etc/nixos/hashed-password.nix
```

Optional sops-nix files are split by scope:

- `/etc/nixos/secrets/secrets.yaml`: system secrets, decrypted through the host
  SSH key.
- `Linux/NixOS/home/secrets/default.nix`: user secrets from
  `home/secrets/secrets.yaml`, decrypted through the user's age key.

System secret bootstrap:

```bash
nix-shell -p sops ssh-to-age age
ssh-to-age -private-key -i /etc/ssh/ssh_host_ed25519_key > /tmp/hostkey.txt
# or: sudo ssh-to-age < /etc/ssh/ssh_host_ed25519_key.pub
sops /etc/nixos/secrets/secrets.yaml
```

Put the public age key into `/etc/nixos/secrets/.sops.yaml`. In the default thin
layout, `mkMySetupHost` automatically wires `/etc/nixos/secrets/secrets.yaml`
into sops-nix when that file exists. The legacy full layout still supports
`hosts/NixOS/secrets/sops.nix`.

User secret bootstrap:

```bash
age-keygen -o ~/.config/sops/age/keys.txt
age-keygen -y ~/.config/sops/age/keys.txt
sops home/secrets/secrets.yaml
systemctl --user enable --now sops-nix.service
```

Put the generated public key into the relevant `.sops.yaml` recipient list. For
services that depend on decrypted user secrets, add an explicit user unit
ordering such as `After=sops-nix.service`.

On repeat TUI runs, the `Passwords` section checks those paths and shows
`already exists` when a value is present. Leaving both password and confirmation
blank preserves the existing value. Enter a new value only when changing it.

For non-interactive CLI apply, pass secrets through files so they do not leak
through shell history:

```bash
nix run "path:$PWD" -- apply \
  --user-password-file /path/to/user-password
```

Secret files must be regular files, non-symlinks, and not group/world readable.

## What The Installer Handles

- Host/user settings: hostname, username, full name, home directory.
- MySetup channel: stable/main branch or development/dev branch for the
  generated thin wrapper.
- Git identity: `user.name` and `user.email` for Home Manager Git config.
- Locale/region fields: timezone, locale, console keymap, weather location.
- Display: monitor name, mode, position, scale, Hypr keyboard layouts/toggle.
- Package preset: `personal`, `developer`, `desktop`, or `minimal`.
- Feature flags: GPU type, Secure Boot, CTF tools, OmniRouter.
- Services: Zapret enable flag, Zapret config preset.
- Passwords: Linux user password hash.
- Dots: Hyprland Lua config, scripts chmod, wallpapers, Zen Browser Catppuccin
  chrome, optional Sine profile, Neovim, v2rayN `sing-box`.

Package presets are intentionally coarse:

- `personal`: full private workstation profile, including desktop apps, dev
  stacks, proxy/VPN tools, Wine, games, containers, virtualization, and optional
  CTF tooling.
- `developer`: desktop apps plus developer/API/container tooling, without the
  personal gaming/proxy/Wine layer.
- `desktop`: browser, chat, office, media, shell/Wayland essentials, and common
  desktop utilities.
- `minimal`: shell/Wayland essentials and core CLI/system tools only.

Implementation note: `hosts/NixOS/default.nix` imports the full local module
group, and individual modules gate behavior with `mysetup.packages.preset`.
This keeps one host graph while letting `minimal`, `desktop`, `developer`, and
`personal` evaluate through the same code path.

## Hyprland Lua Runtime

The active Hyprland config is Lua-only and assumes Hyprland 0.55 or newer:

- `~/.config/hypr/hyprland.lua` is the stable entrypoint owned by Home Manager.
- That file loads `$XDG_STATE_HOME/mysetup/hypr-runtime/hyprland.lua`, which is
  rewritten by the shell runtime when switching profiles.
- Shared MySetup modules live under `Linux/dots/hypr/hyprland/*.lua`,
  `variables.lua`, `scheme/default.lua`, and `lib/mysetup.lua`.
- Shell-specific binds and launchers live under
  `Linux/dots/hypr/{caelestia,noctalia,end4}/*.lua`.
- Common runtime fragments are `shell-common-keybinds.lua`,
  `shell-workspace-keybinds.lua`, `shell-keybinds.lua`,
  `shell-launcher.lua`, and `shell-profile.lua`.

`hyprlock.conf` and `hypridle.conf` intentionally stay in hyprlang because the
Hyprland companion tools have not moved to Lua config here. Hyprland's old
`hyprland.conf`, `shell-keybinds.conf`, `shell-launcher.conf`, and
`shell-profile.conf` are no longer active entrypoints.

Lua bind helpers call `hl.dsp.*` directly. This matters on Hyprland 0.55:
legacy commands such as `hyprctl dispatch movewindow l` are parsed as Lua and
will not behave like old hyprlang dispatchers.

## Runtime Shells

Shell selection is runtime-managed after the system is applied. It is no longer
a TUI install-time choice.

Use:

```text
Super+Shift+W
```

The QuickShell selector opens on the focused monitor and switches between:

- `caelestia-shell`
- `noctalia`
- `end4` / Illogical Impulse

The full per-shell keybind reference (common + caelestia + noctalia + end4)
lives in the [GitHub Wiki](https://github.com/TakuyaYagam1/MySetup/wiki) or
[`Linux/keybinds.md`](keybinds.md).

Runtime state:

```text
$XDG_STATE_HOME/mysetup/active-shell
```

Runtime log:

```text
$XDG_RUNTIME_DIR/mysetup-shell.log
```

Manual switch commands:

```bash
~/.config/hypr/scripts/start-shell.sh caelestia
~/.config/hypr/scripts/start-shell.sh noctalia
~/.config/hypr/scripts/start-shell.sh end4
```

The shell stack is intentionally split:

- `caelestia` and `noctalia` use the installer-managed `Linux/dots/hypr`
  Lua entrypoint with shared `shell-common-keybinds.lua` and
  `shell-workspace-keybinds.lua` fragments.
- `end4` uses a dedicated Home Manager profile based on
  `end-4/dots-hyprland`, with patched Lua Hyprland and QuickShell paths.
- `end4` keeps mutable runtime settings under `~/.config/illogical-impulse`,
  so shell-side JSON changes can persist without a rebuild.

Ownership contract:

- Installer owns first-apply user dotfile sync, unmanaged backups, executable
  bits for Hypr scripts, and immediate runtime bootstrap.
- Home Manager owns stable Hypr entrypoints, managed Hypr scripts, shell
  selector assets, and shell profile metadata after rebuild.
- Runtime shell scripts own mutable files under `$XDG_STATE_HOME/mysetup` and
  may rewrite active-shell fragments during profile switches.
- User/vendor state remains mutable under shell-specific config/cache paths,
  especially Caelestia/Noctalia JSON and end4 Illogical Impulse settings.

## Boot Theme

Customize the GRUB, SDDM, and Plymouth boot logos with your own image.

Drop your own logo into:

```text
~/.config/mysetup/boot-theme/
```

First apply seeds that directory with an example `logo.png` (the current
default theme logo) and a `README.txt` explaining the naming below - it never
overwrites a file that's already there.

Naming, highest priority first:

- `grub.png` / `grub.jpg` - GRUB boot menu logo only
- `sddm.png` / `sddm.jpg` - SDDM login avatar only
- `plymouth.png` / `plymouth.jpg` - Plymouth boot splash only
- `logo.png` / `logo.jpg` - shared fallback used by all three

A per-service file always wins over `logo.png`. Delete a file to fall back to
the next one in that order, or delete them all to restore the built-in
default. GRUB's image is auto-resized to 320x320 (its theme layout is
hand-calibrated for that size); SDDM and Plymouth accept any aspect ratio.

Takes effect on the next `mysetup apply` / `nixos-rebuild switch`. SDDM shows
the new avatar at the next login screen; GRUB and Plymouth render before Linux
even starts, so you need an actual reboot to see them - `switch` alone
prepares the files but can't repaint a bootloader or splash screen that's
already run.

## Apply Flow

The installer applies changes defensively:

1. Creates a temporary staging wrapper flake for `/etc/nixos`.
2. Writes generated `host-vars.nix`, `configuration.nix`, and `home.nix`
   templates when they do not already exist.
3. Preserves host-local `hardware-configuration.nix`, `hashed-password.nix`,
   `private/`, and `secrets/`. Existing MySetup thin installs also keep
   `flake.lock`, `configuration.nix`, and `home.nix`; generated wrapper
   `flake.nix` files may be regenerated to pick up the selected lock mode.
   Stock NixOS or legacy non-thin configs are replaced with the generated thin
   wrapper and clean override templates.
4. Copies or generates `hashed-password.nix` for the staging build.
5. Runs `nix flake update --flake <staging>` for the default thin wrapper, so
   the installed host owns the important external input revisions in
   `/etc/nixos/flake.lock`. Use `--lock-mode managed` to keep the compatibility
   behavior of updating only `mysetup` and using MySetup's transitive lock.
6. Runs `nixos-rebuild dry-build` against the staging flake before touching
   `/etc/nixos`.
7. Backs up `/etc/nixos` to a unique `/etc/nixos.bak.<timestamp>.<pid>.<n>`.
8. Syncs the thin staging tree to `/etc/nixos` without deleting legacy mirror
   files.
9. Applies selected user dotfiles and reloads Hypr when a session is running.
10. Asks before `nixos-rebuild switch` in TUI mode.
11. Writes `/etc/nixos/mysetup/state.json` only after switch succeeds.

Use `--layout full` to keep the old full-mirror behavior for debugging or
migration fallback. Use `--lock-mode managed` with the thin layout when you want
the host to track only the MySetup commit while reusing MySetup's tested
transitive dependency lock.

Rollback is intentionally scoped to `/etc/nixos`. If user dotfile sync fails
after partial writes under `~/.config`, run `mysetup doctor` and re-apply or
clean those user-level files separately.

`/etc/nixos` is the canonical activation target. Building a full system
toplevel directly from the cloned checkout can fail until host-local files are
present; use the installer or build from `/etc/nixos` for real activation
checks.

The default installed layout is intentionally small:

```text
/etc/nixos/
├── flake.nix
├── flake.lock
├── host-vars.nix
├── hardware-configuration.nix
├── configuration.nix      # system-level overrides
├── home.nix               # Home Manager overrides
├── hashed-password.nix    # generated when password is reset
├── private/
│   └── default.nix         # local-only Nix module imports
├── secrets/               # optional system sops-nix secrets
└── mysetup/state.json     # written after successful activation
```

Add NixOS packages, services, and system overrides to `configuration.nix`.
Add user packages and Home Manager overrides to `home.nix`.
Use `private/` for explicit local imports that must not live in the public
repository. Fresh installs import `./private` from `configuration.nix`; add
local modules to `private/default.nix`, which includes commented examples for
`ida-pro.nix`, `ida-mcp.nix`, and `ida-plugins.nix`.

## Commands

```bash
# Local checkout: open the interactive TUI.
nix run "path:$PWD?dir=Linux/NixOS#mysetup"
nix run "path:$PWD?dir=Linux/NixOS#mysetup" -- tui

# Remote stable/main: open the interactive TUI.
nix run --refresh 'github:TakuyaYagam1/MySetup'

# Remote latest/dev: open the interactive TUI, then select the development channel.
nix run --refresh 'github:TakuyaYagam1/MySetup/dev?dir=Linux/NixOS#mysetup' -- tui

# Read-only inspection commands.
nix run "path:$PWD?dir=Linux/NixOS#mysetup" -- doctor
nix run "path:$PWD?dir=Linux/NixOS#mysetup" -- print-state

# Non-interactive apply commands. These use saved state and do not open the TUI.
nix run "path:$PWD?dir=Linux/NixOS#mysetup" -- apply --no-switch
nix run "path:$PWD?dir=Linux/NixOS#mysetup" -- apply --source-channel development --no-switch
nix run "path:$PWD?dir=Linux/NixOS#mysetup" -- apply --lock-mode managed --no-switch
nix run "path:$PWD?dir=Linux/NixOS#mysetup" -- apply --layout full --no-switch

# Managed cleanup.
nix run "path:$PWD?dir=Linux/NixOS#mysetup" -- cleanup
```

`apply --no-switch` is a validation mode: it stages the build and stops after
`nixos-rebuild dry-build`, before writing `/etc/nixos`, user dotfiles,
activation, or activated state.

Use `--dry-run` to preview filesystem actions where supported. The installer logs
commands but does not execute external command actions through the shared runner,
and it avoids host-local side effects such as password hashing or writing
`/etc/nixos`:

```bash
nix run "path:$PWD?dir=Linux/NixOS#mysetup" -- apply --dry-run --no-switch
```

Useful checks after changing installer or shell integration:

| Make target | Run it when |
| --- | --- |
| `make -C Linux/installer check` | Before pushing or applying - full local CI: lint, fmt-check, hypr-bind-check, shell-check, tests, nix evals. |
| `make -C Linux/installer shell-check` | After editing Hypr scripts, JSON, or Python patch sources under `Linux/dots/hypr/`. |
| `make -C Linux/installer nix-hm-eval` | After touching `home/`, end4 runtime-env, or shell-profile imports - evaluates the runtime shell module and all-on home-manager imports including end4. |
| `make -C Linux/installer nix-installed-mirror-build` | After flake changes that affect an already-installed system - builds `mysetup` from an `/etc/nixos`-style temporary mirror. |
| `make all` (run from `Linux/`) | Aggregate: delegates to installer Makefile + `statix` + `deadnix` + json-lint. |

## Configuration Structure

```text
Linux/NixOS/
├── flake.nix
├── flake.lock
├── modules/
│   ├── mysetup-options.nix        # `mysetup.*` NixOS options
│   └── mysetup-stack.nix          # reusable workstation stack
├── hosts/NixOS/
│   ├── default.nix
│   ├── host-vars.nix
│   ├── secrets/                   # optional sops-nix system secrets
│   ├── hashed-password.nix        # generated locally when password is reset
│   └── hardware-configuration.nix # host-local, preserved from /etc/nixos
├── lib/                           # flake glue: layout, hosts, packages,
│   │                              # overlays, modules, presets, ports,
│   │                              # mysetupLib helpers
│   └── package-sets/              # per-preset package set definitions
├── profiles/                      # base / desktop / developer / features
│                                  # import layers (composed in hosts/NixOS)
├── home/                          # home-manager root + shell profiles
│   ├── home.nix
│   ├── theming.nix
│   ├── avatar.jpg
│   ├── lib/                       # dotfile sync + shell selector helpers
│   ├── caelestia/                 # caelestia-shell profile
│   ├── noctalia/                  # noctalia profile
│   ├── end4/                      # end-4 Illogical Impulse profile
│   ├── programs/                  # btop, cava, fastfetch, fish, foot, git,
│   │                              # packages (preset-gated home pkgs),
│   │                              # starship, thunar, vesktop, uwsm, …
│   ├── secrets/                   # optional sops-nix user secrets
│   └── shells/
│       └── quickshell/mysetup-shell-selector/  # Super+Shift+W picker
├── pkgs/                          # pure derivations
│                                  # (omnirouter, sddm-meowrch-theme)
├── programs/                      # system-wide program modules
│                                  # (dev-tools, fish, hyprland, thunar, …)
├── services/                      # databases, observability, sddm,
│                                  # virtualization, zapret, omnirouter, …
├── system/                        # fonts, hardware, kernel, locale,
│   │                              # networking, nvidia, packages, settings
│   └── boot/                      # grub, plymouth, secure boot
├── themes/                        # active theme switch + grub/plymouth/sddm
├── users/                         # user + android-sdk modules
└── Wallpapers/

Linux/dots/
├── hypr/
│   ├── hyprland.lua
│   ├── variables.lua
│   ├── caelestia/
│   ├── noctalia/
│   ├── end4/
│   ├── hyprland/                  # shared Hyprland Lua modules
│   ├── lib/                       # Lua helpers for paths and bind wrappers
│   ├── scheme/                    # color scheme outputs
│   ├── scripts/                   # bash + fish helpers (sourced by Hypr)
│   ├── shell-common-keybinds.lua
│   └── shell-workspace-keybinds.lua
├── nvim/                          # LazyVim-based Neovim config
└── zen/
    └── chrome/                    # Zen Browser Catppuccin chrome

Linux/installer/                   # Go TUI/CLI (`mysetup`)
├── cmd/mysetup/                   # binary entrypoint
├── internal/                      # app, apply, cleanup, config, defaults,
│                                  # doctor, dots, paths, rollback, run,
│                                  # secrets, shellruntime, tui, zenutil
├── scripts/                       # nix-eval helpers used by Makefile checks
├── Makefile                       # local CI: lint, fmt, test, nix checks
├── go.mod
└── go.sum

Linux/Makefile                     # aggregate make targets for the Linux tree
                                   # (delegates to installer/Makefile + nix)
```

## Layer Model: NixOS vs Home-Manager

Many programs (Thunar, fish, Hyprland, …) intentionally appear in **two**
files. They are not duplicates - they configure two different layers:

| Layer | Scope | Owns | Example |
| --- | --- | --- | --- |
| NixOS module (`programs/<x>.nix`, `services/<x>.nix`, `system/<x>.nix`) | system-wide, applied by root via `nixos-rebuild` | package install in `/etc`, polkit/dbus/systemd, mime database, kernel/udev | `programs/thunar.nix`: `programs.thunar.enable`, `environment.systemPackages = [ tumbler ffmpegthumbnailer ]` |
| Home-Manager module (`home/programs/<x>.nix`) | per-user, applied by your user via `home-manager activate` | `~/.config` dotfiles, xfconf/gsettings, GTK theme, user-only `home.activation` hooks | `home/programs/thunar.nix`: `xfconf.settings.thunar.*`, `xdg.configFile."Thunar/uca.xml"`, restart of user thumbnail daemons |

These two scopes run in **different processes with different privileges** and
cannot be merged into one file. The same split exists in every serious NixOS
configuration. If a program only has one file, it is because it does not need
the other half:

- **NixOS-only** (no home-manager half): `programs/gaming.nix`,
  `programs/xdg-portal.nix`, `programs/system-tools.nix`,
  `programs/development.nix`. These provide system features that have no
  per-user dotfile surface (gamemode, steam, portals, build tooling).
- **Home-Manager-only** (no NixOS half): `home/programs/btop.nix`,
  `home/programs/starship.nix`, `home/programs/cava.nix`,
  `home/programs/fastfetch.nix`. These are user-scope CLI tools with config
  files in `~/.config/`; no system service or polkit/dbus rule is required.
- **Asymmetric pair**: `programs/fish.nix` (system-wide shell enable +
  `users.defaultUserShell`) plus `home/programs/fish.nix` (aliases,
  abbreviations, functions, integrations like `direnv` / `zoxide`).

Preset gating (`mysetupLib.mkIfPresetOrMore "desktop" config.mysetup`) decides
**whether** a NixOS module is active per host preset
(`minimal -> desktop -> developer -> personal`). Home-manager packages use the
same preset helpers via `home/programs/packages.nix`.

Compositional note: there is no separate `profiles/personal.nix` file. The
`personal` preset is just `developer` + `desktop` + everything gated on
`presets.personal` (e.g. games in `home/programs/packages.nix`).
`hosts/NixOS/default.nix` imports `profiles/{base,desktop,developer,features}.nix`
directly, and each module checks `mysetup.packages.preset` at evaluation time.

## Recovery

The installer keeps unique `/etc/nixos.bak.<timestamp>.<pid>.<n>` backups
before replacing `/etc/nixos` (matches Apply Flow step 5).

To recover manually, pick the most recent backup and roll it forward:

```bash
ls -dt /etc/nixos.bak.* | head -1                # most recent backup
sudo rsync -a --delete /etc/nixos.bak.<timestamp>.<pid>.<n>/ /etc/nixos/
sudo nixos-rebuild switch --flake /etc/nixos#NixOS
```

Run Doctor for checks and recovery hints:

```bash
nix run "path:$PWD?dir=Linux/NixOS#mysetup" -- doctor
```

For shell-switch issues, inspect:

```bash
cat "${XDG_RUNTIME_DIR:-/tmp}/mysetup-shell.log"
```

## Troubleshooting

Quick triage for the most common breakage paths. If none of these apply,
run `mysetup doctor` and check the relevant log.

- **Shell-swap (Super+Shift+W) does nothing or freezes.** Check
  `$XDG_STATE_HOME/mysetup/active-shell` (should be one of `caelestia`,
  `noctalia`, `end4`). Tail `$XDG_RUNTIME_DIR/mysetup-shell.log` while you
  press the binding. Most failures are stale lockfiles under
  `$XDG_STATE_HOME/mysetup/hypr-runtime/` - remove that directory and retry.
- **Hyprland keybinds are listed but do nothing.** Run `hyprctl configerrors`
  first. On Hyprland 0.55+, dispatch commands must use the Lua dispatcher API;
  MySetup binds should go through `lib/mysetup.lua` helpers or direct
  `hl.dsp.*` calls, not old `hyprctl dispatch movewindow l` style strings.
- **Thunar shows generic icons instead of image previews.** Check that
  `tumbler` is running (`pgrep -f tumbler-1/tumblerd`), that
  `~/.cache/thumbnails/normal/` is writable, and that `XDG_DATA_DIRS`
  contains `/run/current-system/sw/share` and `~/.nix-profile/share`. If end4
  is active, this is most likely upstream `env = XDG_DATA_DIRS,...` leaking
  through - re-apply and re-login.
- **`apply --no-switch` fails at dry-build.** Read the Nix error tail; the
  installer aborts before touching `/etc/nixos`. Re-run
  `make -C Linux/installer nix-hm-eval` to localise the failure to a single
  module before retrying apply.
- **Build is huge / slow on first apply.** Expected: the config pulls in
  multiple Wayland shells, Qt/QML, desktop apps, optional CTF stacks. Make
  sure the listed binary caches are accepted (see Install / Reconfigure) -
  without them everything compiles from source.
- **First `switch` fails after writing `/etc/nixos` with dbus/systemd activation
  errors.** Reboot into the new generation boundary, then run
  `sudo nixos-rebuild switch --flake /etc/nixos#NixOS`. The dry-build already
  passed; this usually means the live system could not restart part of the
  desktop/session stack cleanly.
- **`sudo nixos-rebuild switch` works locally but `mysetup apply` fails.**
  The installer wraps switch with stricter checks (dry-build, password
  hash, dots mirror). Run `nix run "path:$PWD?dir=Linux/NixOS#mysetup" --
  doctor` to see which precondition is missing.

## Maintenance

System update:

```bash
cd /etc/nixos
sudo nix flake update
sudo nixos-rebuild switch --flake .#NixOS
```

Default thin installs use an independent host lock: `/etc/nixos/flake.lock`
owns nixpkgs, Home Manager, Stylix, Quickshell, shell flakes, and the selected
MySetup source revision. The `stable` channel uses the `main` branch; the
`development` channel uses `dev`. Managed thin installs update only the
`mysetup` input and reuse the transitive lock shipped by MySetup.

Existing generated thin wrappers migrate to the independent lock shape on the
next `mysetup apply`; a plain `nix flake update` only changes `flake.lock`, not
the wrapper `flake.nix` structure.

Garbage collection:

```bash
sudo nix-collect-garbage -d
sudo nixos-rebuild boot --flake /etc/nixos#NixOS
```
