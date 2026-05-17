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

No-clone install (recommended for fresh boxes - Nix fetches the flake source
into `/nix/store`, the wrapped installer points `MYSETUP_REPO_ROOT` at that
immutable source, then stages it to `/etc/nixos`):

```bash
nix run 'github:TakuyaYagam1/MySetup?dir=Linux/NixOS#mysetup'
```

Or with a local clone (useful when iterating on the config):

```bash
git clone https://github.com/TakuyaYagam1/MySetup.git
cd MySetup
nix run "path:$PWD?dir=Linux/NixOS#mysetup"
```

The installer will ask you about:

- Username and password
- Package preset (personal / developer / desktop / minimal)
- Display and keyboard layout
- Secure Boot (Lanzaboote)
- GPU type (AMD / Intel / NVIDIA)
- Region (Russia - disables DataGrip, enables Zapret by default)
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

Additional thanks to [@outfoxxed](https://github.com/outfoxxed) for
[QuickShell](https://github.com/outfoxxed/quickshell), which all three shell
profiles depend on, and to the wider Hyprland community for the help,
suggestions, and reference configs that shaped this setup.

## License

GPL-3.0
