# NixOS Configuration

NixOS + Hyprland configuration managed by a single Go TUI installer.

The old `install.sh` and `dots/install.fish` entry points are gone. System settings and user dotfiles are now applied in one session through the flake app.

## Install / Reconfigure

From the repository root:

```bash
nix run ./Linux/NixOS#mysetup
```

The installer writes machine-local state to `/etc/nixos/mysetup/state.json` and keeps a draft in `$XDG_STATE_HOME/mysetup/draft.json`. Passwords are never stored in state.

Fresh install through `/mnt` is not supported in v1. Run this on an already booted NixOS system that has `/etc/nixos/hardware-configuration.nix`.

## What The Installer Handles

- Host/user settings: hostname, username, full name, home directory.
- Region: timezone, locale, console keymap, optional weather location.
- Display: monitor name and mode, for example `eDP-1` + `2560x1600@120`.
- Shell profile: `caelestia` or `noctalia`.
- Package preset: `personal` (Takuya's full config, default), `developer`, `desktop`, or `minimal`.
- Feature flags: GPU type, Secure Boot, CTF tools, OmniRouter, Russia mode, Zapret.
- Services: pgAdmin email and optional password reset.
- Dots: Hypr config, scripts chmod, wallpapers, Zen Browser Catppuccin chrome, optional Sine profile, Neovim, v2rayN `sing-box`.

The apply flow builds a temporary staging copy first, preserves host-local files
such as `hardware-configuration.nix` and `flake.lock`, runs
`nixos-rebuild dry-build` against that staging flake, and only after a
successful dry-build backs up/syncs `/etc/nixos`, applies dots, optionally runs
`switch`, and writes `/etc/nixos/mysetup/state.json`.

`/etc/nixos` is the canonical activation target. Building a full system
toplevel directly from the cloned checkout can fail until host-local hardware
files are present; use the installer or build from `/etc/nixos` for real system
activation checks.

Package presets are intentionally coarse:

- `personal`: Takuya's full config, including desktop apps, dev stacks, CTF toggles, proxy/VPN tools, Wine, games, containers, and virtualization.
- `developer`: desktop apps plus developer/API/container tooling, without the personal gaming/proxy/Wine layer.
- `desktop`: browser, chat, office, media, shell/Wayland essentials, and common desktop utilities.
- `minimal`: shell/Wayland essentials and core CLI/system tools only.

## Commands

```bash
nix run ./Linux/NixOS#mysetup -- tui
nix run ./Linux/NixOS#mysetup -- doctor
nix run ./Linux/NixOS#mysetup -- print-state
nix run ./Linux/NixOS#mysetup -- apply --no-switch
nix run ./Linux/NixOS#mysetup -- cleanup
```

Use `--dry-run` to preview filesystem actions where supported:

```bash
nix run ./Linux/NixOS#mysetup -- apply --dry-run --no-switch
```

Passwords are accepted only through files when using the non-interactive CLI,
so secrets do not leak through shell history:

```bash
nix run ./Linux/NixOS#mysetup -- apply \
  --user-password-file /path/to/user-password \
  --pgadmin-password-file /path/to/pgadmin-password
```

## Installer Development

The Go installer lives in `Linux/installer` and has its own Makefile:

```bash
make -C Linux/installer help
make -C Linux/installer check
make -C Linux/installer lint
make -C Linux/installer fmt
make -C Linux/installer nix-build
```

`check` runs formatting checks, GolangCI-Lint, `go vet`, race-enabled unit tests,
the local binary build, and the Nix flake package build.

## Configuration Structure

```text
Linux/NixOS/
├── flake.nix
├── hosts/NixOS/
│   ├── default.nix
│   ├── variables.nix
│   ├── hashed-password.nix        # generated locally when password is reset
│   └── hardware-configuration.nix # host-local, preserved from /etc/nixos
├── system/
├── services/
├── programs/
├── packages/
├── users/
├── home/
├── themes/
└── Wallpapers/

Linux/dots/
├── hypr/
└── zen/

Linux/installer/
└── Go source for mysetup
```

## Recovery

The installer keeps `/etc/nixos.bak.<timestamp>` backups before replacing `/etc/nixos`.

To recover manually:

```bash
sudo rsync -a --delete /etc/nixos.bak.<timestamp>/ /etc/nixos/
sudo nixos-rebuild switch --flake /etc/nixos#NixOS
```

Run Doctor for checks and recovery hints:

```bash
nix run ./Linux/NixOS#mysetup -- doctor
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
