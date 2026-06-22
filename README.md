# My Setup

> **! READ THE README FOR YOUR PLATFORM BEFORE DOING ANYTHING !**
>
> - Linux (NixOS): [Linux/README.md](Linux/README.md)
> - Windows: [Windows/README.md](Windows/README.md)

Personal system configuration for Linux (NixOS + Hyprland) and Windows (Komorebi + YASB).
The Linux Hyprland runtime targets Hyprland 0.55+ and uses Lua config
entrypoints (`hyprland.lua` plus Lua modules) instead of legacy hyprlang
`hyprland.conf` fragments.

[![NixOS Rice & Dev Environment](assets/preview.png)](https://youtu.be/fgmueUOnfhk)

*[8-minute video tour of the NixOS side](https://youtu.be/fgmueUOnfhk) - click the image above*

## Screenshots

| Zen Browser (Catppuccin chrome) | Zen + Sine mods | Neovim (LazyVim) |
| :---: | :---: | :---: |
| ![Zen Browser](assets/zen.png) | ![Zen + Sine mods](assets/zen_sine_mods.png) | ![LazyVim](assets/lazyvim.png) |

> Zen Browser theming lives in [`Linux/dots/zen/chrome/`](Linux/dots/zen/chrome).
> Neovim config (LazyVim-based) lives in [`Linux/dots/nvim/`](Linux/dots/nvim).

### TUI Installer

| User section | Display / Region section |
| :---: | :---: |
| ![TUI installer - User](assets/tui1.png) | ![TUI installer - Display](assets/tui2.png) |

### Windows (Komorebi + YASB)

![Windows - Komorebi tiling WM + YASB status bar](assets/windows.png)

> Contributors: read [CONTRIBUTING.md](CONTRIBUTING.md) before opening a PR. Security issues: see [SECURITY.md](SECURITY.md).

## Structure

- **Linux/** - NixOS configuration with Hyprland 0.55+ Lua config, custom themes, dev environment
- **Windows/** - Windows configuration with Komorebi tiling WM and YASB status bar

## Quick Start

### Linux (NixOS)

> **Read [Linux/README.md](Linux/README.md) first** - contains pre-installation requirements,
> path configuration, regional notes, and troubleshooting.

No-clone install or reconfigure (recommended - Nix fetches the installer into
`/nix/store`; the installed `/etc/nixos` stays thin and tracks the reusable
MySetup NixOS flake). This opens the interactive TUI and uses the stable
`main` branch by default:

```bash
nix run --refresh 'github:TakuyaYagam1/MySetup'
```

If you want the latest fixes before they are merged to `main`, run the TUI from
the `dev` branch and select `General -> MySetup channel -> development` before
applying:

```bash
nix run --refresh 'github:TakuyaYagam1/MySetup/dev?dir=Linux/NixOS#mysetup' -- tui
```

In the TUI `General` section, `MySetup channel` controls what the installed
thin `/etc/nixos/flake.nix` follows after apply. `stable` tracks the `main`
branch; `development` tracks the `dev` branch for testing fixes before they are
merged.

Fresh NixOS/KDE bootstrap note: on the first adoption, MySetup replaces the
active `/etc/nixos/configuration.nix` with a clean host-local override file and
backs up the previous `/etc/nixos` to `/etc/nixos.bak.<timestamp>...`. Your
hardware config, password hash, `private/`, and `secrets/` stay preserved. If
the first live `switch` fails because dbus/systemd could not fully reactivate,
reboot and run:

```bash
sudo nixos-rebuild switch --flake /etc/nixos#NixOS
```

VPN/proxy tools such as Amnezia are only a network workaround before running
MySetup; they are not required in the bootstrap `configuration.nix`.

Or with a local clone (useful when iterating on the config):

```bash
git clone https://github.com/TakuyaYagam1/MySetup.git
cd MySetup
nix run "path:$PWD"
```

The direct NixOS installer app also exposes CLI subcommands. These commands are
non-interactive; `apply` uses the saved state and does not open the TUI:

```bash
# Apply the default thin /etc/nixos layout with host-owned dependency locks.
nix run --refresh 'github:TakuyaYagam1/MySetup?dir=Linux/NixOS#mysetup' -- apply

# Apply from the development channel without opening the TUI.
nix run --refresh 'github:TakuyaYagam1/MySetup/dev?dir=Linux/NixOS#mysetup' -- \
  apply --source-channel development

# Validate the staged system build without writing /etc/nixos or switching.
nix run --refresh 'github:TakuyaYagam1/MySetup?dir=Linux/NixOS#mysetup' -- apply --no-switch

# Keep the legacy full mirror layout while migrating or debugging.
nix run --refresh 'github:TakuyaYagam1/MySetup?dir=Linux/NixOS#mysetup' -- apply --layout full

# Compatibility mode: only update the MySetup input and use MySetup's tested
# transitive flake.lock for nixpkgs/home-manager/stylix/etc.
nix run --refresh 'github:TakuyaYagam1/MySetup?dir=Linux/NixOS#mysetup' -- apply --lock-mode managed

# Low-RAM bootstrap: keep the first installer run from oversubscribing CPU/RAM.
nix run --refresh --option max-jobs 1 --option cores 2 \
  'github:TakuyaYagam1/MySetup?dir=Linux/NixOS#mysetup' -- \
  apply --lock-mode managed

# Inspect or repair an installed host.
nix run --refresh 'github:TakuyaYagam1/MySetup?dir=Linux/NixOS#mysetup' -- doctor
```

After install, normal updates happen from `/etc/nixos`:

```bash
nixos-update
```

That command runs `nix flake update` in `/etc/nixos` and then switches the
system. The default thin wrapper owns the important external inputs in
`/etc/nixos/flake.lock`, so users can advance nixpkgs, Home Manager, Stylix,
Quickshell, and related flake inputs locally without copying the full
repository into `/etc/nixos`. If `MySetup channel` is set to `development`,
`nixos-update` updates the wrapper from the `dev` branch instead of `main`.

Useful post-apply checks:

```bash
systemctl cat omnirouter.service | rg 'ExecStartPre|omnirouter-ensure-server-env'
sudo systemctl status omnirouter.service
```

The repository root flake also exposes reusable shell modules and the full host
constructor for external NixOS flakes. Pin a release tag for immutable installs,
or use a moving branch such as `main` for stable updates or `dev` for
pre-merge testing via `nix flake update`:

```nix
inputs.mysetup = {
  url = "github:TakuyaYagam1/MySetup/main";
  inputs.nixpkgs.follows = "nixpkgs";
};
```

Then import the shell module in your NixOS host:

```nix
modules = [
  inputs.mysetup.nixosModules.shells

  {
    system.stateVersion = "26.05";
    mysetup.user = {
      username = "alice";
      fullName = "Alice";
      homeDirectory = "/home/alice";
    };
  }
];
```

For a full MySetup host wrapper, use `mysetup.lib.mkMySetupHost` from the
`Linux/NixOS` flake or the root re-export. The installer generates this shape
automatically in `/etc/nixos/flake.nix`; it is the supported public API for the
complete workstation stack.

The installer will ask you about:

- MySetup channel (stable/main or development/dev)
- Username and password
- Package preset (personal / developer / desktop / minimal)
- Display and keyboard layout
- Secure Boot (Lanzaboote)
- GPU type (AMD / Intel / NVIDIA)
- Locale and timezone
- Zapret DPI bypass config
- CTF tools
- User dotfiles (Hypr, Zen Browser theme, Neovim, wallpapers)

Shell profile is no longer an install-time question. After the system is
applied, switch between `caelestia-shell`, `noctalia-shell`, and `end-4`
(Illogical Impulse) at runtime via `Super+Shift+W` - see
[Linux/README.md](Linux/README.md) for details.

### Windows

> **Read [Windows/README.md](Windows/README.md) first** - you must update paths in
> `yasb/config.yaml` before running the installer.

1. Open PowerShell as Administrator
2. Run:

```powershell
git clone https://github.com/TakuyaYagam1/MySetup.git
cd MySetup\Windows
.\install.ps1
```

## Credits

A huge thanks to the upstream projects that make the Linux side of this setup
possible. The Hyprland shells, rices, and themes here are built directly on
top of their work - these dots would not exist without them:

- [meowrch/meowrch](https://github.com/meowrch/meowrch) - original rice that
  inspired the SDDM theme, Plymouth/GRUB visuals, and the overall Hyprland
  aesthetic baked into this config.
- [noctalia-dev/noctalia-shell](https://github.com/noctalia-dev/noctalia-shell) - QuickShell-based desktop
  shell shipped as one of the runtime profiles.
- [end-4/dots-hyprland](https://github.com/end-4/dots-hyprland) - the
  Illogical Impulse Hyprland/QuickShell dotfiles wired in as the `end4`
  runtime profile.
- [caelestia-dots/shell](https://github.com/caelestia-dots/shell) -
  Caelestia QuickShell, used as the default runtime profile and as the
  base for the Caelestia Hypr/dots layer.
- [anotherhadi/nixy](https://github.com/anotherhadi/nixy) - reference NixOS
  workstation config used for ideas around Home Manager modules, MIME defaults,
  Yazi/Night Shift ergonomics, and Nix utility wiring.
- [PortSwigger/mcp-server](https://github.com/PortSwigger/mcp-server) -
  official Burp Suite MCP extension. MySetup does not package it; install it
  from upstream when you need Burp MCP access.
- [LaurieWired/GhidraMCP](https://github.com/LaurieWired/GhidraMCP) -
  Ghidra MCP bridge. MySetup does not package it; install it from upstream
  when you need Ghidra MCP access.

Additional thanks to [@outfoxxed](https://github.com/outfoxxed) for
[QuickShell](https://github.com/outfoxxed/quickshell), which all three shell
profiles depend on, and to the wider Hyprland community for the help,
suggestions, and reference configs that shaped this setup.

## License

GPL-3.0
