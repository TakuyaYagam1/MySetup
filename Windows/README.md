# Windows Configuration

Windows configuration with Komorebi tiling window manager, whkd hotkey
daemon, and YASB status bar - keybinds ported 1:1 from the Linux/Hyprland
setup (`Super` mapped to `Alt`).

## Important Notes

### Administrator Rights

The installation script requires administrator privileges (winget installs,
long-path registry key, and writing Zen's autoconfig into `Program Files`).
Run PowerShell as Administrator before executing the installation.

### Fonts

The installation script installs **JetBrainsMono Nerd Font** via winget
(`DEVCOM.JetBrainsMonoNerdFont`) - it is required for the glyphs in the
YASB / komorebi bar. The Nerd Font is used everywhere; the non-patched
JetBrains Mono is not needed. No manual font download required.

### Long Paths

The installation script automatically enables long path support in Windows.
This is required for proper operation of Komorebi.

### Path Configuration

The repo uses `username_here` as a placeholder in machine-specific config
paths (komorebi/YASB wallpapers, etc.). The installation script
**automatically substitutes** it with `$env:USERNAME` on copy to
`~/.config/`, so no manual editing is needed in the common case.

If you copy configs manually (without `install.ps1`), do the substitution
yourself, e.g.:

```powershell
(Get-Content -Raw .\yasb\config.yaml) -replace 'username_here', $env:USERNAME |
    Set-Content "$env:USERPROFILE\.config\yasb\config.yaml" -Encoding UTF8
```

### Dynamic App Launching (no hardcoded paths)

App launcher keybinds do **not** hardcode executable paths. `whkd/launch.ps1`
resolves an app by its Start-menu name via the Windows app index
(`Get-StartApps`) and launches it through `shell:AppsFolder`. This survives
reinstalls, version bumps, drive-letter and username changes - nothing to
edit per machine.

A keybind whose app is not installed is a harmless no-op: `launch.ps1` exits
quietly (hidden window), the rest of the config keeps working. So unused
launchers (e.g. Antigravity, v2rayN) can stay in `whkdrc`.

## Installation

### Automated Installation

1. Open PowerShell as Administrator

2. Navigate to the Windows config directory:

   ```powershell
   cd wahrwelt\Windows
   ```

3. Run the installation script:

   ```powershell
   .\install.ps1
   ```

4. After installation completes, finish the manual steps the script prints
   (Vencord, Zen - see [Post-install](#post-install-manual-steps)).

   The script handles end-to-end: long paths, winget installs (komorebi,
   whkd, yasb, JetBrainsMono Nerd Font, Zen Browser, Discord, PowerToys,
   WinToys, TranslucentTB), `komorebic quickstart`, copying configs +
   `launch.ps1` with username substitution, autostart, Vencord patch, and
   the Zen Sine + Catppuccin setup.

### Manual Installation

1. Install the binaries:

   ```powershell
   winget install --id LGUG2Z.komorebi --accept-package-agreements --accept-source-agreements
   winget install --id LGUG2Z.whkd --accept-package-agreements --accept-source-agreements
   winget install --id AmN.yasb --accept-package-agreements --accept-source-agreements
   winget install --id DEVCOM.JetBrainsMonoNerdFont --accept-package-agreements --accept-source-agreements
   ```

2. Enable long paths support:

   ```powershell
   Set-ItemProperty 'HKLM:\SYSTEM\CurrentControlSet\Control\FileSystem' -Name 'LongPathsEnabled' -Value 1
   ```

3. Copy configuration files from the repository root (substitute
   `username_here` as shown in [Path Configuration](#path-configuration)):

   ```powershell
   Copy-Item -Path '.\Windows\komorebi\*' -Destination $env:USERPROFILE -Recurse -Force
   New-Item -ItemType Directory -Force -Path "$env:USERPROFILE\.config\yasb"
   Copy-Item -Path '.\Windows\yasb\*' -Destination "$env:USERPROFILE\.config\yasb" -Recurse -Force
   Copy-Item -Path '.\Windows\whkd\whkdrc' -Destination "$env:USERPROFILE\.config\whkdrc" -Force
   Copy-Item -Path '.\Windows\whkd\launch.ps1' -Destination "$env:USERPROFILE\.config\launch.ps1" -Force
   ```

4. Initialize Komorebi:

   ```powershell
   komorebic quickstart
   komorebic enable-autostart --whkd
   komorebic start --whkd
   ```

   > `komorebic quickstart` overwrites `~/.config/whkdrc` with a default -
   > re-copy our `whkdrc` (step 3) **after** running it.

5. (Optional) Zen Sine + Catppuccin theme:

   ```powershell
   .\Windows\zen\setup-zen.ps1
   ```

## Keybindings

`Super` is mapped to `Alt`. Source: `Linux/dots/hypr` keybinds.

### Window management (komorebi)

| Keybind | Action |
| --- | --- |
| `Alt + Q` | Close focused window |
| `Alt + H/J/K/L` | Focus left / down / up / right |
| `Alt + Shift + H/J/K/L` | Move window left / down / up / right |
| `Alt + [` / `Alt + ]` | Cycle focus in container |
| `Alt + 1..7` | Focus workspace 1..7 |
| `Alt + Shift + 1..7` | Move window to workspace 1..7 |
| `Alt + Space` | Toggle float |
| `Alt + Shift + Space` | Toggle monocle |
| `Alt + P` | Toggle pause |
| `Alt + Shift + R` | Retile |
| `Alt + Shift + Enter` | Promote to main |
| `Alt + -` / `Alt + =` | Resize horizontal decrease / increase |
| `Alt + Shift + -` / `Alt + Shift + =` | Resize vertical decrease / increase |
| `Alt + X` / `Alt + Y` | Flip layout horizontal / vertical |

### App launchers

| Keybind | App |
| --- | --- |
| `Alt + Enter` | Termius (terminal) |
| `Alt + Shift + F` | Zen Browser |
| `Alt + Shift + C` | Cursor |
| `Alt + Shift + V` | VS Code |
| `Alt + Shift + D` | DataGrip |
| `Alt + Shift + A` | Antigravity |
| `Alt + E` | File Explorer |
| `Alt + Shift + Q` | Amnezia VPN |
| `Alt + Shift + N` | v2rayN |
| `Alt + Shift + I` | Spotify |
| `Alt + Shift + T` | Telegram |
| `Alt + Shift + B` | Discord |

## Post-install (manual steps)

1. **YASB** - `yasbc start` if it did not autostart.
2. **komorebi** - verify with `komorebic state`.
3. **Vencord** - launch Discord once. If client mods do not appear, rerun:

   ```powershell
   winget install --id Vendicated.Vencord --override "-install -branch stable"
   ```

4. **Zen Sine + Catppuccin** - open `about:support` -> **Clear startup
   cache**, then fully restart Zen. Re-run `Windows\zen\setup-zen.ps1`
   after Zen browser updates (updates wipe the install-dir autoconfig).

## Structure

```text
Windows/
├── install.ps1           # Automated installation script
├── komorebi/             # Komorebi configuration
│   ├── komorebi.json     # Main Komorebi configuration
│   ├── komorebi.bar.json # Komorebi bar configuration
│   └── applications.json # Application-specific rules
├── whkd/
│   ├── whkdrc            # whkd hotkey bindings (komorebi nav + launchers)
│   └── launch.ps1        # Dynamic app resolver (Start-menu index)
├── yasb/                 # YASB status bar configuration
│   ├── config.yaml       # YASB main configuration
│   └── styles.css        # YASB styling
└── zen/
    └── setup-zen.ps1     # Zen Sine mod loader + Catppuccin theme
```

## Features

+ Komorebi tiling window manager with multiple layout modes
  (BSP, VerticalStack, HorizontalStack, etc.)
+ YASB status bar with system information and workspace indicators
+ whkd hotkey daemon with dynamic, path-free app launching
+ Application-specific window rules
+ **Zen Browser** with the Sine mod loader and Catppuccin Macchiato
  Mauve theme (pinned, SHA256-verified Sine archives - parity with the
  Linux/Nix setup)
+ **Discord + Vencord** - Vencord is patched headlessly during install
  (winget verifies the installer hash)
+ Extras: **PowerToys** (Microsoft utility suite, manages its own
  run-at-startup), **WinToys** (system tweak utility, on-demand -
  intentionally not auto-started) and **TranslucentTB** (taskbar styler,
  auto-starts via a Startup-folder shortcut)

## Credits

+ [Komorebi](https://github.com/LGUG2Z/komorebi) - Tiling window manager
+ [YASB](https://github.com/amnweb/yasb) - Status bar
+ [whkd](https://github.com/LGUG2Z/whkd) - Hotkey daemon
+ [Sine](https://github.com/CosmoCreeper/Sine) - Zen/Firefox mod loader
+ [Vencord](https://github.com/Vendicated/Vencord) - Discord client mod

## License

GPL-3.0
