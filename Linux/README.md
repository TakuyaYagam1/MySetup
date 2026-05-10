# NixOS Configuration

NixOS + Hyprland configuration managed by the `mysetup` Go TUI/CLI installer.

The old `install.sh` and `dots/install.fish` entry points are gone. System
settings, host-local generated files, and user dotfiles are applied through the
flake app.

## Install / Reconfigure

Run from the repository root:

```bash
nix run "path:$PWD?dir=Linux/NixOS#mysetup"
```

Use the repository-root `path:...?dir=Linux/NixOS` form for local runs so the
flake source still includes sibling `Linux/dots` and `Linux/installer`.

Or run directly from GitHub without cloning first:

```bash
nix run 'github:skr1ms/MySetup?dir=Linux/NixOS#mysetup'
```

This is still the normal full apply path. Nix fetches the repository source into
`/nix/store`, the wrapped installer points `MYSETUP_REPO_ROOT` at that immutable
source, then the installer stages `NixOS`, `dots`, and `installer` into `/tmp`
before applying them to `/etc/nixos`.

For reproducible installs, pin a commit:

```bash
nix run 'github:skr1ms/MySetup/<commit>?dir=Linux/NixOS#mysetup'
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
/etc/nixos/secrets/pgadmin-password
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
  --user-password-file /path/to/user-password \
  --pgadmin-password-file /path/to/pgadmin-password
```

Secret files must be regular files, non-symlinks, and not group/world readable.

## What The Installer Handles

- Host/user settings: hostname, username, full name, home directory.
- Git identity: `user.name` and `user.email` for Home Manager Git config.
- Region: timezone, locale, console keymap, weather location, Russia mode.
- Display: monitor name, mode, position, scale, Hypr keyboard layouts/toggle.
- Package preset: `personal`, `developer`, `desktop`, or `minimal`.
- Feature flags: GPU type, Secure Boot, CTF tools, OmniRouter.
- Services: pgAdmin email, Zapret enable flag, Zapret config preset.
- Passwords: Linux user password hash and pgAdmin web password secret.
- Dots: Hypr config, scripts chmod, wallpapers, Zen Browser Catppuccin chrome,
  optional Sine profile, Neovim, v2rayN `sing-box`.

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
  entrypoint with runtime `shell-keybinds.conf` and `shell-launcher.conf`.
- `end4` uses a dedicated Home Manager profile based on
  `end-4/dots-hyprland`, with patched Hyprland and QuickShell paths.
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
7. Mirrors external dots into `/etc/nixos/dots`.
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

```bash
make -C Linux/installer check
make -C Linux/installer shell-check
make -C Linux/installer nix-hm-eval
make -C Linux/installer nix-installed-mirror-build
```

## Configuration Structure

```text
Linux/NixOS/
├── flake.nix
├── hosts/NixOS/
│   ├── default.nix
│   ├── host-vars.nix
│   ├── variables.nix
│   ├── hashed-password.nix        # generated locally when password is reset
│   └── hardware-configuration.nix # host-local, preserved from /etc/nixos
├── profiles/
├── home/
│   ├── caelestia/
│   ├── noctalia/
│   ├── end4/
│   └── shells/
│       └── quickshell/mysetup-shell-selector/
├── packages/
├── programs/
├── services/
├── system/
├── themes/
└── Wallpapers/

Linux/dots/
├── hypr/
│   ├── caelestia/
│   ├── noctalia/
│   ├── end4/
│   ├── scripts/
│   ├── shell-common-keybinds.conf
│   └── shell-workspace-keybinds.conf
└── zen/

Linux/installer/
└── Go source for mysetup
```

## Recovery

The installer keeps `/etc/nixos.bak.<timestamp>` backups before replacing
`/etc/nixos`.

To recover manually:

```bash
sudo rsync -a --delete /etc/nixos.bak.<timestamp>/ /etc/nixos/
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
