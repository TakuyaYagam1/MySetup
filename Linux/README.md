# NixOS Configuration

NixOS configuration with Hyprland window manager based on [caelestia-dots](https://github.com/caelestia-dots) and [meowrch](https://github.com/meowrch).

This configuration uses a modular file structure for better maintainability and organization.

## System Requirements

- **Display Resolution**: Optimized for 2560x1600. Other resolutions may require adjustments in display parameters.
- **Architecture**: x86_64 Linux
- **Minimum Disk Space**: 300GB recommended
- **Network**: Internet connection required for initial installation

## Pre-Installation Configuration

### Bootloader Selection (GRUB vs Secure Boot)

**Configuration Location**: `system/boot/`

**Instructions**:
Select the appropriate bootloader configuration in `configuration.nix`.

#### GRUB (Default)

Ensure `grub.nix` is imported and `secure.nix` is commented out:

```nix
imports = [ ./system/boot/grub.nix ]; # Comment secure.nix
```

#### Secure Boot (lanzaboote)

**Requirement**: Active Secure Boot in BIOS with cleared keys.

> **Important**: `system/boot/plymouth.nix` must be **commented out** when using Secure Boot.
> Plymouth hooks into the early boot stage and interferes with Lanzaboote's signed initrd, causing a black screen or signature verification failure.

1. **BIOS Configuration**:
   - Enter BIOS -> Secure Boot.
   - Select "Clear Secure Boot Keys" (Setup Mode).
   - Set OS Type to "Windows UEFI Mode".
   - Save & Exit.

2. **NixOS Configuration**:
   - Edit `configuration.nix`: comment `plymouth.nix` and `grub.nix`, uncomment `secure.nix`.

   ```nix
   imports = [
     ./system/boot/grub.nix         # keep for EFI params — or comment if fully migrating
     # ./system/boot/plymouth.nix   # must be commented with Secure Boot
     ./system/boot/secure.nix
   ];
   ```

   - Rebuild system:

   ```bash
   sudo nixos-rebuild boot --install-bootloader --flake .#NixOS
   ```

3. **Key Enrollment** (One-time):

   ```bash
   sudo sbctl create-keys
   sudo sbctl enroll-keys --microsoft
   ```

#### Migrating from GRUB (Troubleshooting)

**If GRUB still appears or boots instead of Lanzaboote:**

1. Remove old GRUB entry from NVRAM:

   ```bash
   sudo efibootmgr              # Find BootXXXX for GRUB/NixOS
   sudo efibootmgr -b XXXX -B   # Delete it
   ```

2. Reinstall bootloader:

   ```bash
   sudo nixos-rebuild boot --install-bootloader --flake .#NixOS
   ```

### Regional Restrictions

**Users in the Russian Federation** must comment out `jetbrains.datagrip` and `jetbrains.goland` before installation due to regional licensing restrictions.

**Location**: `home/apps.nix`

```nix
# jetbrains.datagrip # Comment this line if installing from Russia
# jetbrains.goland   # Comment this line if installing from Russia
```

These packages require VPN/VPS connection for installation within the Russian Federation.

### Internet Censorship Circumvention (DPI Bypass)

**Users in Russia**: This configuration includes a pre-configured `zapret` service to bypass DPI censorship (YouTube, Discord, etc.).

**Location**: `services/zapret.nix`

- **Enabled by default**: The service is active out of the box.
- **Strategy**: Configured with `fake,multidisorder` strategy which works on most Russian ISPs (including MGTS/GPON).
- **Customization**: If you experience issues, edit `NFQWS_OPT_DESYNC` in `services/zapret.nix`.

### GPU Configuration (NVIDIA / AMD)

**Configuration Location**: `system/hardware.nix` and `system/nvidia-drivers.nix`

**NVIDIA Users**:
To enable NVIDIA drivers, uncomment the corresponding import in `configuration.nix`. Do not edit the hardware configuration directly unless necessary.

```nix
imports = [
  # ...
  ./system/nvidia-drivers.nix # Uncomment this line
];
```

### Password Configuration

**CRITICAL**: Change the default user password before system installation.

**Location**: `users/user.nix`

```nix
initialPassword = "your_password_here";  # Replace with your secure password
```

### PostgreSQL Security

After system installation, immediately change the PostgreSQL superuser password:

```bash
sudo -u postgres psql -p 5442
```

```sql
ALTER USER postgres WITH PASSWORD 'your_secure_password';
\q
```

### CTF Tools (Optional)

**For CTF participants**: This configuration includes a comprehensive set of CTF tools, but the package list is commented out by default (based on [Nix Security Tool Box](https://fabaff.github.io/nix-security-box/list)).

**Location**: `packages/ctf-tools.nix`, import in `configuration.nix` line ~36

To enable CTF tools, uncomment the import in `configuration.nix`:

```nix
./packages/ctf-tools.nix
```

## Installation Procedures

### Method 1: Fresh System Installation

1. **Prepare Installation Media**

   - Boot from NixOS live USB
   - Verify network connectivity: `ping nixos.org`

2. **Partition and Mount Disk**

   ```bash
   # Example partition scheme (adjust to your needs)
   # /dev/sda1: 512M EFI partition
   # /dev/sda2: Remaining space for root

   mkfs.fat -F 32 /dev/sda1
   mkfs.ext4 /dev/sda2

   mount /dev/sda2 /mnt
   mkdir -p /mnt/boot
   mount /dev/sda1 /mnt/boot
   ```

3. **Clone Repository**

   ```bash
   git clone https://github.com/skr1ms/MySetup.git
   cd MySetup
   ```

4. **Generate Hardware Configuration**

   ```bash
   sudo nixos-generate-config --root /mnt
   ```

5. **Deploy Configuration Files**

   ```bash
   sudo cp -r Linux/NixOS/* /mnt/etc/nixos/

   # Remove documentation files
   cd /mnt/etc/nixos
   sudo rm -f LICENSE README.md
   ```

6. **Configure User Credentials**

   ```bash
   sudo nano /mnt/etc/nixos/users/user.nix
   # Change initialPassword value
   ```

7. **Verify Configuration**

   ```bash
   cd /mnt/etc/nixos
   sudo nix --extra-experimental-features 'nix-command flakes' flake check
   ```

8. **Install System**

   ```bash
   sudo nixos-install --flake .#NixOS
   ```

9. **Post-Installation Setup**

   ```bash
   # After reboot and login
   fish /etc/nixos/dots/install.fish
   ```

### Method 2: Existing NixOS System Migration

1. **Backup Current Configuration**

   ```bash
   sudo cp -r /etc/nixos /etc/nixos.backup
   ```

2. **Clone Repository**

   ```bash
   cd /tmp
   git clone https://github.com/skr1ms/MySetup.git
   ```

3. **Preserve Hardware Configuration**

   ```bash
   sudo cp /etc/nixos/hardware-configuration.nix /tmp/hw-backup.nix
   ```

4. **Deploy New Configuration**

   ```bash
   sudo rm -rf /etc/nixos/*
   sudo cp -r MySetup/Linux/NixOS/* /etc/nixos/
   sudo cp /tmp/hw-backup.nix /etc/nixos/hardware-configuration.nix

   # Remove documentation
   cd /etc/nixos
   sudo rm -f LICENSE README.md
   ```

5. **Configure User Password**

   ```bash
   sudo nano /etc/nixos/users/user.nix
   # Update initialPassword
   ```

6. **Validate and Apply**

   ```bash
   sudo nix --extra-experimental-features 'nix-command flakes' flake check
   sudo nixos-rebuild switch --flake .#NixOS
   ```

7. **Apply Desktop Configuration**

   ```bash
   fish /etc/nixos/dots/install.fish
   ```

8. **Reboot System**

   ```bash
   sudo reboot
   ```

## Configuration Structure

```text
Linux/NixOS/
├── configuration.nix             # Main entry point, imports all modules
├── flake.nix                     # Flake inputs: nixpkgs-unstable, nixpkgs-stable, home-manager, etc.
├── hardware-configuration.nix    # Auto-generated, machine-specific (do not version control)
│
├── system/                       # Core NixOS system configuration
│   ├── boot/
│   │   ├── grub.nix              # Bootloader, EFI, GRUB theme, swap, kernel params
│   │   ├── plymouth.nix          # Boot splash screen (meowrch theme) — comment with Secure Boot
│   │   └── secure.nix            # Secure Boot via Lanzaboote (disabled by default)
│   ├── settings.nix              # Global system settings
│   ├── locale.nix                # Timezone, i18n, keyboard layout
│   ├── networking.nix            # Network, firewall, DNS
│   ├── security.nix              # Polkit, SSH daemon, user permissions
│   ├── power.nix                 # Power management
│   ├── hardware.nix              # Hardware-specific configuration
│   ├── nvidia-drivers.nix        # NVIDIA drivers (disabled by default)
│   └── nix.nix                   # Nix daemon settings, garbage collection
│
├── services/                     # System services
│   ├── display.nix               # SDDM display manager + Hyprland
│   ├── databases.nix             # PostgreSQL, MySQL, Redis, ClickHouse
│   ├── observability.nix         # Grafana, Prometheus, Loki (disabled by default)
│   ├── virtualization.nix        # libvirtd, VirtualBox, Docker, Podman
│   ├── system-services.nix       # Essential system services
│   └── zapret.nix                # DPI bypass for Russian ISPs
│
├── programs/                     # NixOS-level program configuration
│   ├── fish.nix                  # Fish shell — enables + sets as default
│   ├── thunar.nix                # Thunar file manager + plugins
│   ├── gaming.nix                # Steam, Gamescope, etc.
│   ├── development.nix           # Dev-related program flags
│   └── system-tools.nix          # System utility programs
│
├── packages/                     # environment.systemPackages — available to all users
│   ├── system-packages.nix       # Core system utilities, icons, wine (stable)
│   ├── dev-tools.nix             # Compilers, language toolchains (Go, Node, Python, Java)
│   ├── ctf-tools.nix             # CTF & security tools — all from nixpkgs-stable
│   └── fonts.nix                 # System fonts + fontconfig defaults
│
├── users/                        # User accounts
│   └── user.nix                  # User definition, groups, initial password
│
├── home/                         # Home Manager — per-user configuration (user: takuya)
│   ├── home.nix                  # Entry point, imports all HM modules
│   ├── caelestia.nix             # Caelestia shell + Quickshell desktop config
│   ├── theming.nix               # GTK/QT theme, cursor, dconf
│   ├── apps.nix                  # User GUI apps (browsers, IDEs, messengers, AI tools)
│   └── dev-packages.nix          # Python environment (python3.withPackages)
│
├── themes/                       # Visual themes (referenced from system configs)
│   ├── grub-theme/               # GRUB bootloader theme (Meowrch)
│   ├── plymouth-theme/           # Boot splash theme (Meowrch)
│   └── sddm-theme/               # SDDM login screen theme (Meowrch)
│
└── Wallpapers/                   # Wallpaper collection (copied to ~/Pictures/Wallpapers)
```

## Key Features

### Desktop Environment

- **Compositor**: Hyprland (Wayland)
- **Display Manager**: SDDM with **Meowrch** custom theme
- **Shell**: Fish with Starship prompt
- **File Manager**: Thunar with plugins
- **Theme**: Dark theme (Papirus icons, Bibata cursors, adw-gtk3)

### Development Environment

- **Languages**: Go, Node.js, TypeScript, Python, Java
- **Databases**: PostgreSQL 17 (port 5442), MySQL/MariaDB (port 3316)
- **Containers**: Docker, Podman (Docker-compatible)
- **Orchestration**: kubectl, helm, k9s, kind, kustomize
- **Tools**: Neovim (nightly), VSCode, Cursor, Android Studio

### Virtualization Support

- libvirtd (KVM/QEMU)
- VirtualBox with Extension Pack
- Docker with custom DNS
- Podman with Docker compatibility layer

### Database Services

- **PostgreSQL 17**: Custom port 5442, remote access enabled
- **MySQL/MariaDB**: Custom port 3316
- **Optional Services** (commented out by default):
  - Redis (port 6389)
  - Memcached
  - ClickHouse (ports 8133, 9010)

### Observability Stack (Optional)

Disabled by default, uncomment in `services/observability.nix`:

- Grafana (port 3010)
- Prometheus (port 9100)
- Loki (port 3110)

## Customization

### Modifying System Configuration

Each module is self-contained and can be edited independently:

1. **Change networking settings**: Edit `system/networking.nix`
2. **Add/remove packages**: Edit appropriate files in `packages/`
3. **Configure services**: Edit files in `services/`
4. **User applications**: Edit `home/apps.nix`

After modifications, rebuild the system:

```bash
sudo nixos-rebuild switch --flake /etc/nixos#NixOS
```

### Adding New Modules

Create new `.nix` files in appropriate directories and add imports to `configuration.nix`:

```nix
imports = [
  # ...
  ./your-category/your-module.nix
];
```

## Troubleshooting

### Build Failures

**JetBrains licensing errors in Russia** (DataGrip / GoLand):

```bash
nano /etc/nixos/home/apps.nix
# Comment out jetbrains.datagrip and jetbrains.goland lines
```

**Flake evaluation errors**:

```bash
sudo nix flake update /etc/nixos
sudo nixos-rebuild switch --flake /etc/nixos#NixOS
```

**Hardware detection issues**:

```bash
sudo nixos-generate-config --root /mnt --force
```

### Service Issues

**PostgreSQL connection refused**:

```bash
sudo systemctl status postgresql
sudo journalctl -u postgresql
```

**Hyprland crashes**:

```bash
# Check logs
journalctl --user -u hyprland
```

## Maintenance

### System Updates

```fish
# Using the update alias (defined in dots/fish/config.fish):
update
# expands to: cd /etc/nixos && nix flake update && nixos-rebuild switch --flake /etc/nixos#NixOS

# Or manually:
cd /etc/nixos
sudo nix flake update
sudo nixos-rebuild switch --flake /etc/nixos#NixOS
```

`nix flake update` bumps all inputs including `nixos-cachyos-kernel`, so the kernel updates alongside the rest of the system.

### Garbage Collection

Automatic garbage collection runs daily at 03:15. Manual cleanup:

```bash
sudo nix-collect-garbage -d
sudo nixos-rebuild boot --flake /etc/nixos#NixOS
```

## Credits

Configuration based on:

- [caelestia-dots](https://github.com/caelestia-dots) - Hyprland shell and desktop environment
- [meowrch](https://github.com/meowrch) - Theme inspiration

## License

GPL-3.0
