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
nix run "path:$PWD?dir=Linux/NixOS#mysetup"
```

Run this from the cloned repository root. The detailed forms (GitHub-direct,
pinned commit/branch, refresh) are documented in the next section.

## Install / Reconfigure

Run from the repository root:

```bash
nix run "path:$PWD?dir=Linux/NixOS#mysetup"
```

Use the repository-root `path:...?dir=Linux/NixOS` form for local runs so the
flake source still includes sibling `Linux/dots` and `Linux/installer`.

Or run directly from GitHub without cloning first:

```bash
nix run 'github:TakuyaYagam1/MySetup?dir=Linux/NixOS#mysetup'
```

This is still the normal full apply path. Nix fetches the repository source into
`/nix/store`, the wrapped installer points `MYSETUP_REPO_ROOT` at that immutable
source, then the installer stages `NixOS`, `dots`, and `installer` into `/tmp`
before applying them to `/etc/nixos`.

Nix caches the resolved source for ~1 hour (`tarball-ttl` default). If you just
pushed a commit and want the new HEAD right now, force a re-fetch:

```bash
nix run --refresh 'github:TakuyaYagam1/MySetup?dir=Linux/NixOS#mysetup'
```

For reproducible installs, pin a commit, branch, or tag:

```bash
# Specific commit (full SHA):
nix run 'github:TakuyaYagam1/MySetup/<commit>?dir=Linux/NixOS#mysetup'

# Specific branch (e.g. develop):
nix run 'github:TakuyaYagam1/MySetup/develop?dir=Linux/NixOS#mysetup'
```

Fresh install through `/mnt` is not supported in v1. Run this on an already
booted NixOS system with an existing `/etc/nixos/hardware-configuration.nix`.

Nix may ask whether to allow the flake app to use additional substituters. That
is expected for this config; the installer and checks use these binary caches:

- `https://nix-community.cachix.org`
- `https://hyprland.cachix.org`
- `https://quickshell.cachix.org`
- `https://numtide.cachix.org`

The first full build can still be large because this config includes multiple
Wayland shells, Qt/QML packages, desktop apps, and optional CTF/development
stacks.

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
/etc/nixos/hosts/NixOS/hashed-password.nix
```

Optional sops-nix modules are split by scope:

- `Linux/NixOS/hosts/NixOS/secrets/sops.nix`: system secrets from
  `hosts/NixOS/secrets/secrets.yaml`, decrypted through the host SSH key.
- `Linux/NixOS/home/secrets/default.nix`: user secrets from
  `home/secrets/secrets.yaml`, decrypted through the user's age key.

System secret bootstrap:

```bash
nix-shell -p sops ssh-to-age age
ssh-to-age -private-key -i /etc/ssh/ssh_host_ed25519_key > /tmp/hostkey.txt
# or: sudo ssh-to-age < /etc/ssh/ssh_host_ed25519_key.pub
sops hosts/NixOS/secrets/secrets.yaml
```

Put the public age key into `hosts/NixOS/secrets/.sops.yaml`, then import
`hosts/NixOS/secrets/sops.nix` from `hosts/NixOS/default.nix` when system-level
sops secrets are needed.

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
nix run "path:$PWD?dir=Linux/NixOS#mysetup" -- apply \
  --user-password-file /path/to/user-password
```

Secret files must be regular files, non-symlinks, and not group/world readable.

## What The Installer Handles

- Host/user settings: hostname, username, full name, home directory.
- Git identity: `user.name` and `user.email` for Home Manager Git config.
- Region: timezone, locale, console keymap, weather location, Russia mode
  (`mysetup.features.russiaMode` - when `true`, drops JetBrains products
  such as `jetbrains.datagrip` / `jetbrains.goland` from the home package set).
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
- `noctalia-shell`
- `end4` / Illogical Impulse

The full per-shell keybind reference (common + caelestia + noctalia + end4)
lives in the [GitHub Wiki](https://github.com/TakuyaYagam1/MySetup/wiki).

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

## Apply Flow

The installer applies changes defensively:

1. Creates a temporary staging copy of `Linux/NixOS`, `Linux/dots`, and
   `Linux/installer`.
2. Preserves host-local `hardware-configuration.nix`.
3. Copies or generates `hosts/NixOS/hashed-password.nix` for the staging build.
4. Runs `nixos-rebuild dry-build` against the staging flake before touching
   `/etc/nixos`.
5. Backs up `/etc/nixos` to a unique `/etc/nixos.bak.<timestamp>.<pid>.<n>`.
6. Syncs the staging tree to `/etc/nixos`, including `flake.lock` so switch uses
   the same lock graph as dry-build.
7. Mirrors `Linux/dots/` from the staged repository copy into
   `/etc/nixos/dots/` so the flake can reference dotfiles by path.
8. Applies selected user dotfiles and reloads Hypr when a session is running.
9. Asks before `nixos-rebuild switch` in TUI mode.
10. Writes `/etc/nixos/mysetup/state.json` only after switch succeeds.

Rollback is intentionally scoped to `/etc/nixos`. If user dotfile sync fails
after partial writes under `~/.config`, run `mysetup doctor` and re-apply or
clean those user-level files separately.

`/etc/nixos` is the canonical activation target. Building a full system
toplevel directly from the cloned checkout can fail until host-local files are
present; use the installer or build from `/etc/nixos` for real activation
checks.

## Commands

```bash
nix run "path:$PWD?dir=Linux/NixOS#mysetup"
nix run "path:$PWD?dir=Linux/NixOS#mysetup" -- tui
nix run "path:$PWD?dir=Linux/NixOS#mysetup" -- doctor
nix run "path:$PWD?dir=Linux/NixOS#mysetup" -- print-state
nix run "path:$PWD?dir=Linux/NixOS#mysetup" -- apply --no-switch
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
├── modules/
│   └── mysetup-options.nix        # `mysetup.*` NixOS options
├── profiles/                      # base / desktop / developer / features
│                                  # import layers (composed in hosts/NixOS)
├── home/                          # home-manager root + shell profiles
│   ├── home.nix
│   ├── apps.nix
│   ├── dev-packages.nix
│   ├── theming.nix
│   ├── avatar.jpg
│   ├── lib/                       # dotfile sync + shell selector helpers
│   ├── caelestia/                 # caelestia-shell profile
│   ├── noctalia/                  # noctalia-shell profile
│   ├── end4/                      # end-4 Illogical Impulse profile
│   ├── programs/                  # btop, cava, fastfetch, fish, foot, git,
│   │                              # packages (preset-gated home pkgs),
│   │                              # starship, thunar, vesktop, uwsm, …
│   ├── secrets/                   # optional sops-nix user secrets
│   └── shells/
│       └── quickshell/mysetup-shell-selector/  # Super+Shift+W picker
├── packages/                      # fonts, dev-tools,
│                                  # zen-browser, sddm-meowrch-theme, …
├── programs/                      # system-wide program modules
│                                  # (fish, hyprland, gaming, thunar, …)
├── services/                      # databases, observability, sddm,
│                                  # virtualization, zapret, omnirouter, …
├── system/                        # hardware, kernel, locale, networking,
│   │                              # nvidia, power, security, settings
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
| NixOS module (`programs/<x>.nix`, `services/<x>.nix`, `packages/<x>.nix`) | system-wide, applied by root via `nixos-rebuild` | package install in `/etc`, polkit/dbus/systemd, mime database, kernel/udev | `programs/thunar.nix`: `programs.thunar.enable`, `environment.systemPackages = [ tumbler ffmpegthumbnailer ]` |
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

Garbage collection:

```bash
sudo nix-collect-garbage -d
sudo nixos-rebuild boot --flake /etc/nixos#NixOS
```
