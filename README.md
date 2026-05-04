# My Setup

> **! READ THE README FOR YOUR PLATFORM BEFORE DOING ANYTHING !**
>
> - Linux (NixOS): [Linux/README.md](Linux/README.md)
> - Windows: [Windows/README.md](Windows/README.md)

Personal system configuration for Linux (NixOS + Hyprland) and Windows (Komorebi + YASB).

## Structure

- **Linux/** - NixOS configuration with Hyprland, custom themes, dev environment
- **Windows/** - Windows configuration with Komorebi tiling WM and YASB status bar

## Quick Start

### Linux (NixOS)

> **Read [Linux/README.md](Linux/README.md) first** - contains pre-installation requirements,
> path configuration, regional notes, and troubleshooting.

```bash
git clone https://github.com/skr1ms/MySetup.git
cd MySetup
nix run ./Linux/NixOS#mysetup
```

The installer will ask you about:

- Username and password
- Shell profile (Caelestia / Noctalia)
- Package preset (personal / developer / desktop / minimal)
- Display and keyboard layout
- Secure Boot (Lanzaboote)
- GPU type (AMD / Intel / NVIDIA)
- Region (Russia - disables DataGrip, enables Zapret by default)
- Zapret DPI bypass config
- CTF tools
- User dotfiles (Hypr, Zen Browser theme, Neovim, wallpapers)

### Windows

> **Read [Windows/README.md](Windows/README.md) first** - you must update paths in
> `yasb/config.yaml` before running the installer.

1. Open PowerShell as Administrator
2. Run:

```powershell
git clone https://github.com/skr1ms/MySetup.git
cd MySetup\Windows
.\install.ps1
```

## License

GPL-3.0
