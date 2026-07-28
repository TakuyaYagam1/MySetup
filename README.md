# Wahrwelt

> **! READ THE README FOR YOUR PLATFORM BEFORE DOING ANYTHING !**
>
> - Linux (NixOS): [Linux/README.md](Linux/README.md)
> - Windows: [Windows/README.md](Windows/README.md)

Personal system configuration for Linux (NixOS + Hyprland) and Windows (Komorebi + YASB).

[![NixOS Rice & Dev Environment](assets/preview.png)](https://youtu.be/fgmueUOnfhk)

*[8-minute video tour](https://youtu.be/fgmueUOnfhk) - click the image above*

---

## `SUPER + SHIFT + W` -> switch shell

Three interchangeable Hyprland shells - **caelestia**, **noctalia**, **end-4** - swap between them at
runtime, no reinstall needed. Details: [Linux/README.md](Linux/README.md#runtime-shells).

Full keybind reference (every shell, every bind): [GitHub Wiki](https://github.com/TakuyaYagam1/wahrwelt/wiki)
or [`Linux/keybinds.md`](Linux/keybinds.md).

Want your own logo on the GRUB/SDDM/Plymouth boot screens instead of the default one? Drop an
image in `~/.config/wahrwelt/boot-theme/` - details in
[Linux/README.md](Linux/README.md#boot-theme).

Want your own Hyprland keybinds/rules/execs without forking the repo? Everything under
`~/.config/hypr/` is Home Manager-managed and read-only, but `~/.config/hypr/wahrwelt/` isn't -
details in
[Linux/README.md](Linux/README.md#customizing-hyprland-without-forking-the-repo).

Any image works as-is (sizing/format is handled automatically), but if you want the same
circular, transparent-background look as the built-in default, crop it yourself first with
[ImageMagick](https://imagemagick.org/):

```bash
convert your-photo.jpg -resize 320x320^ -gravity center -extent 320x320 \
  \( +clone -alpha extract -fill black -colorize 100 -fill white -draw "circle 160,160 160,0" \) \
  -alpha off -compose CopyOpacity -composite logo.png
```

This is entirely optional - a plain rectangular photo works fine too, it just renders as a
square instead of a circle.

---

## Screenshots

| caelestia-shell | noctalia v5 | noctalia v4 | end-4 (Illogical Impulse) |
| :---: | :---: | :---: | :---: |
| ![caelestia-shell](assets/caelestia-shell.png) | ![noctalia v5](assets/noctalia-v5.png) | ![noctalia v4](assets/noctalia-v4.png) | ![end-4](assets/end-4.png) |

| Zen Browser (Catppuccin chrome) | Zen + Sine mods | Neovim (LazyVim) |
| :---: | :---: | :---: |
| ![Zen Browser](assets/zen.png) | ![Zen + Sine mods](assets/zen_sine_mods.png) | ![LazyVim](assets/lazyvim.png) |

| TUI Installer - User | TUI Installer - Display |
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

> **Read [Linux/README.md](Linux/README.md) first** - pre-installation requirements, path
> config, regional notes, troubleshooting, and the full command reference all live there.

| Scenario | Command |
| --- | --- |
| Install (stable `main`, opens TUI) | `nix run --refresh 'github:TakuyaYagam1/wahrwelt'` |
| Install (latest `dev`, opens TUI) | `nix run --refresh 'github:TakuyaYagam1/wahrwelt/dev?dir=Linux/NixOS#wahrwelt' -- tui` |
| Install with more build cores (match your CPU's thread count via `nproc`, not just the `4 4` example below - and keep `max-jobs` low regardless of core count on machines with less than ~16GB RAM, since each job builds independently and OOM doesn't check core count), stable `main`, opens TUI | `nix run --refresh --option max-jobs 4 --option cores 4 'github:TakuyaYagam1/wahrwelt'` |
| Install with more build cores (same caveats as above), latest `dev`, opens TUI | `nix run --refresh --option max-jobs 4 --option cores 4 'github:TakuyaYagam1/wahrwelt/dev?dir=Linux/NixOS#wahrwelt' -- tui` |
| Reapply saved config, no TUI | `nix run --refresh 'github:TakuyaYagam1/wahrwelt?dir=Linux/NixOS#wahrwelt' -- apply` |
| Inspect / repair an installed host | `nix run --refresh 'github:TakuyaYagam1/wahrwelt?dir=Linux/NixOS#wahrwelt' -- doctor` |
| Update an already-installed system | `nixos-update` |

Existing installations remain compatible with the legacy repository URL, flake output, CLI
name, module namespace, and host constructor. For example,
`nix run --refresh 'github:TakuyaYagam1/MySetup?dir=Linux/NixOS#mysetup' -- doctor`
continues to work. On the first successful Wahrwelt update, recognized legacy state and
configuration paths are migrated automatically to `wahrwelt`; compatibility names remain
available only as source/API aliases.

The installer asks about: Wahrwelt channel, username/password, package preset, display and
keyboard layout, Secure Boot, GPU type, locale/timezone, CTF tools, and user
dotfiles. Full flag reference (`--layout`, `--lock-mode`, low-RAM bootstrap, flake module usage
for external hosts, etc.) is in [Linux/README.md](Linux/README.md).

#### Package presets

Presets are **cumulative** - each tier includes everything below it and adds more on top
(`minimal` -> `desktop` -> `developer` -> `personal`). Pick by what the machine is for:

| Preset | Boots into | What it adds on top of the tier below | Best for |
| --- | --- | --- | --- |
| `minimal` | **Text console only** (no display manager, no Hyprland) | Base system + core CLI/system tools, Wayland libraries/tools, and audio utilities (cava, pavucontrol). **No graphical desktop** - you log in at a TTY. | Servers, VMs, low-spec boxes, or anyone who just wants the CLI base. If you boot to a black screen with a blinking cursor, that is the console login working as intended - type your user and password. |
| `desktop` | **The graphical Hyprland rice** | SDDM greeter + Hyprland compositor + the three runtime shells, plus everyday GUI apps (Firefox, Zen, Spotify, Telegram, Vesktop, LibreOffice, mpv, Obsidian, file manager, screenshot/calculator/terminal-multiplexer tools) and gaming/portal/font support. | Most people who want the desktop but not a dev or private-workstation load. This is the lowest preset that gives you a graphical session. |
| `developer` | Same graphical desktop | Developer/API/container tooling: VS Code, API clients (yaak, ngrok), a SQLite TUI, and container extras. | A workstation you also code on, without the full personal app pile. |
| `personal` | Same graphical desktop | The full private-workstation load: Chrome, extra office suites, Anytype, DBeaver/DataGrip, many editors/IDEs (Cursor, Zed, QtCreator, Android Studio), AI CLIs (claude-code, codex, gemini-cli, ollama, opencode), Flutter/Android tooling, plus games (Lutris, Heroic). | **My own machine, basically.** I built this preset specifically for my personal daily-driver, so it's opinionated and packed with my exact toolset - most people should stick to one of the first three. This is by far the heaviest build; on <16GB RAM keep `max-jobs` low (see the build-cores note above) to avoid OOM during the first build. |

### Windows

> **Read [Windows/README.md](Windows/README.md) first** - you must update paths in
> `yasb/config.yaml` before running the installer.

```powershell
git clone https://github.com/TakuyaYagam1/wahrwelt.git
cd wahrwelt\Windows
.\install.ps1
```

## Credits

The Hyprland shells, rices, and themes here are built directly on top of these upstream
projects - these dots would not exist without them:

- [meowrch/meowrch](https://github.com/meowrch/meowrch) - original rice that inspired the SDDM
  theme, Plymouth/GRUB visuals, and the overall Hyprland aesthetic.
- [noctalia-dev/noctalia](https://github.com/noctalia-dev/noctalia) - QuickShell-based desktop
  shell shipped as one of the runtime profiles.
- [end-4/dots-hyprland](https://github.com/end-4/dots-hyprland) - the Illogical Impulse
  Hyprland/QuickShell dotfiles wired in as the `end4` runtime profile.
- [caelestia-dots/shell](https://github.com/caelestia-dots/shell) - Caelestia QuickShell, the
  default runtime profile.
- [anotherhadi/nixy](https://github.com/anotherhadi/nixy) - reference NixOS workstation config
  used for ideas around Home Manager modules, MIME defaults, and Nix utility wiring.

Additional thanks to [@outfoxxed](https://github.com/outfoxxed) for
[QuickShell](https://github.com/outfoxxed/quickshell), which all three shell profiles depend on,
and to the wider Hyprland community for the help, suggestions, and reference configs.

## License

GPL-3.0
