# Windows Configuration

Windows configuration with Komorebi tiling window manager and YASB status bar.

## Important Notes

### Administrator Rights

The installation script requires administrator privileges. Run PowerShell as Administrator before executing the installation.

### Fonts

Install the following fonts before using the configuration:

- **JetBrains Mono**: [jetbrains.com/lp/mono](https://www.jetbrains.com/lp/mono/)
- **JetBrains Mono Nerd Font**: [nerdfonts.com/font-downloads](https://www.nerdfonts.com/font-downloads)

### Long Paths

The installation script automatically enables long path support in Windows. This is required for proper operation of Komorebi.

### Path Configuration

Before running the installation script, you must update all paths in `yasb/config.yaml` to match your system:

1. Replace `username_here` with your actual Windows username in application paths:
   - Telegram: `C:\Users\username_here\AppData\Roaming\Telegram Desktop\Telegram.exe`
   - Discord: `C:\Users\username_here\AppData\Local\Discord\app-1.0.9219\Discord.exe`
   - TeraBox: `C:\Users\username_here\AppData\Roaming\TeraBox\terabox.exe`
   - Spotify: `C:\Users\username_here\AppData\Roaming\Spotify\Spotify.exe`

2. Update wallpaper paths in the `wallpapers` widget section:
   - Default: `C:\\Users\\username_here\\Pictures\\Wallpapers`
   - Add or modify paths according to your wallpaper directory locations

3. Verify application executable paths in the `apps` widget section:
   - Update paths for applications installed in non-standard locations
   - Ensure all application icons and launch commands are correct

All path modifications must be made in `yasb/config.yaml` before copying configuration files to your system.

## Installation

### Automated Installation

1. Open PowerShell as Administrator

2. Navigate to the repository directory:

   ```powershell
   cd MyLinuxSetup\Windows
   ```

3. Run the installation script:

   ```powershell
   .\install.ps1
   ```

4. After installation completes, install the required fonts (see Important Notes)

5. Restart PowerShell and run the following commands:

   ```powershell
   komorebic quickstart
   Copy-Item -Path '.\Windows\komorebi\*' -Destination $env:USERPROFILE -Recurse -Force
   Copy-Item -Path '.\Windows\yasb\*' -Destination "$env:USERPROFILE\.config\yasb" -Recurse -Force
   komorebic enable-autostart --whkd
   komorebic start --whkd
   ```

### Manual Installation

1. Install Komorebi:

   ```powershell
   winget install --id LGUG2Z.komorebi --accept-package-agreements --accept-source-agreements
   ```

2. Install whkd:

   ```powershell
   winget install --id LGUG2Z.whkd --accept-package-agreements --accept-source-agreements
   ```

3. Install YASB:

   ```powershell
   winget install --id AmN.yasb --accept-package-agreements --accept-source-agreements
   ```

4. Enable long paths support:

   ```powershell
   Set-ItemProperty 'HKLM:\SYSTEM\CurrentControlSet\Control\FileSystem' -Name 'LongPathsEnabled' -Value 1
   ```

5. Copy configuration files:

   ```powershell
   Copy-Item -Path '.\Windows\komorebi\*' -Destination $env:USERPROFILE -Recurse -Force
   Copy-Item -Path '.\Windows\yasb\*' -Destination "$env:USERPROFILE\.config\yasb" -Recurse -Force
   ```

6. Initialize Komorebi:

   ```powershell
   komorebic quickstart
   komorebic enable-autostart --whkd
   komorebic start --whkd
   ```

## Structure

```text
Windows/
├── install.ps1          # Automated installation script
├── komorebi/            # Komorebi configuration
│   ├── komorebi.json    # Main Komorebi configuration
│   ├── komorebi.bar.json # Komorebi bar configuration
│   └── applications.json # Application-specific rules
└── yasb/                # YASB status bar configuration
    ├── config.yaml      # YASB main configuration
    └── styles.css       # YASB styling
```

## Features

- Komorebi tiling window manager with multiple layout modes
- YASB status bar with system information and workspace indicators
- whkd hotkey daemon for window management
- Application-specific window rules
- Customizable themes and animations
- Multiple workspace layouts (BSP, VerticalStack, HorizontalStack, etc.)

## Credits

- [Komorebi](https://github.com/LGUG2Z/komorebi) - Tiling window manager
- [YASB](https://github.com/amnweb/yasb) - Status bar
- [whkd](https://github.com/LGUG2Z/whkd) - Hotkey daemon

## License

GPL-3.0
