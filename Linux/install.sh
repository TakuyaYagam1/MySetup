#!/bin/bash

set -e

# Color definitions
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Logging functions
info() { echo -e "${GREEN}[INFO]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }
step() { echo -e "${BLUE}[STEP]${NC} $1"; }

# Verify whiptail is available
if ! command -v whiptail >/dev/null 2>&1; then
    info "Installing whiptail for interactive dialogs..."
    nix-shell -p whiptail --run "whiptail --version" >/dev/null 2>&1 || error "Failed to install whiptail"
fi

# Interactive configuration
step "Starting interactive configuration"

USERNAME=$(whiptail --inputbox "Enter username:" 10 50 "user" 3>&1 1>&2 2>&3) || error "Configuration cancelled"
PASSWORD=$(whiptail --passwordbox "Enter password for $USERNAME:" 10 50 3>&1 1>&2 2>&3) || error "Configuration cancelled"
PASSWORD_CONFIRM=$(whiptail --passwordbox "Confirm password:" 10 50 3>&1 1>&2 2>&3) || error "Configuration cancelled"

if [ "$PASSWORD" != "$PASSWORD_CONFIRM" ]; then
    error "Passwords do not match"
fi

if whiptail --yesno "Enable Secure Boot (Lanzaboote)?" 10 50 3>&1 1>&2 2>&3; then
    SECURE_BOOT=1
else
    SECURE_BOOT=0
fi

if whiptail --yesno "Enable NVIDIA drivers?" 10 50 3>&1 1>&2 2>&3; then
    NVIDIA=1
else
    NVIDIA=0
fi

if whiptail --yesno "Install CTF tools?" 10 50 3>&1 1>&2 2>&3; then
    CTF_TOOLS=1
else
    CTF_TOOLS=0
fi

# Generate configuration summary
SUMMARY="Configuration Summary:\n\n"
SUMMARY+="Username: $USERNAME\n"
SUMMARY+="Secure Boot: $([ $SECURE_BOOT -eq 1 ] && echo 'Enabled' || echo 'Disabled')\n"
SUMMARY+="NVIDIA Drivers: $([ $NVIDIA -eq 1 ] && echo 'Enabled' || echo 'Disabled')\n"
SUMMARY+="CTF Tools: $([ $CTF_TOOLS -eq 1 ] && echo 'Enabled' || echo 'Disabled')\n\n"
SUMMARY+="Proceed with installation?"

if ! whiptail --yesno "$SUMMARY" 20 60 3>&1 1>&2 2>&3; then
    warn "Installation cancelled by user"
    exit 0
fi

# Create temporary directory for modified files
TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

step "Generating user configuration"

# Generate hashed password
HASHED_PASSWORD=$(nix-shell -p mkpasswd --run "mkpasswd -sm sha-512 '$PASSWORD'")

# Create user configuration file
cat > "$TEMP_DIR/user.nix" << EOF
{ ... }:

{
  users.users.${USERNAME} = {
    isNormalUser = true;
    extraGroups = [
      "networkmanager"
      "wheel"
      "video"
      "audio"
      "input"
      "docker"
      "kvm"
      "libvirtd"
      "adbusers"
      "vboxusers"
    ];
    initialHashedPassword = "${HASHED_PASSWORD}";
  };

  system.userActivationScripts = {
    android-adb-fix = {
      text = ''
        mkdir -p ~/Android/Sdk/platform-tools
        rm -f ~/Android/Sdk/platform-tools/adb
        ln -s /run/current-system/sw/bin/adb ~/Android/Sdk/platform-tools/adb
      '';
    };
  };
}
EOF

step "Modifying configuration.nix"

# Copy and modify configuration.nix
cp NixOS/configuration.nix "$TEMP_DIR/configuration.nix"

# Apply Secure Boot configuration
if [ $SECURE_BOOT -eq 1 ]; then
    info "Enabling Secure Boot configuration"
    sed -i 's|./system/boot/grub.nix|# ./system/boot/grub.nix|' "$TEMP_DIR/configuration.nix"
    sed -i 's|# ./system/boot/secure.nix|./system/boot/secure.nix|' "$TEMP_DIR/configuration.nix"
else
    sed -i 's|./system/boot/secure.nix|# ./system/boot/secure.nix|' "$TEMP_DIR/configuration.nix"
    sed -i 's|# ./system/boot/grub.nix|./system/boot/grub.nix|' "$TEMP_DIR/configuration.nix"
fi

# Apply NVIDIA configuration
if [ $NVIDIA -eq 1 ]; then
    info "Enabling NVIDIA drivers"
    sed -i 's|# ./system/nvidia-drivers.nix|./system/nvidia-drivers.nix|' "$TEMP_DIR/configuration.nix"
else
    sed -i 's|./system/nvidia-drivers.nix|# ./system/nvidia-drivers.nix|' "$TEMP_DIR/configuration.nix"
fi

# Apply CTF tools configuration
if [ $CTF_TOOLS -eq 1 ]; then
    info "Enabling CTF tools"
    sed -i 's|# ./packages/ctf-tools.nix|./packages/ctf-tools.nix|' "$TEMP_DIR/configuration.nix"
else
    sed -i 's|./packages/ctf-tools.nix|# ./packages/ctf-tools.nix|' "$TEMP_DIR/configuration.nix"
fi

step "Copying configuration to /etc/nixos/"

# Copy all NixOS configuration files
sudo cp -rf NixOS/* /etc/nixos/

# Copy modified files
sudo cp "$TEMP_DIR/user.nix" /etc/nixos/users/user.nix
sudo cp "$TEMP_DIR/configuration.nix" /etc/nixos/configuration.nix

# Set correct ownership
sudo chown -R root:root /etc/nixos/

step "Building NixOS configuration"

cd /etc/nixos
if ! sudo nixos-rebuild switch --flake .#NixOS; then
    error "NixOS rebuild failed. Check the error messages above."
fi

step "Installing dotfiles"

# Install dotfiles if fish is available
if command -v fish >/dev/null 2>&1; then
    fish ~/MySetup/Linux/dots/install.fish || warn "Dotfiles installation failed or incomplete"
else
    warn "Fish shell not available yet. Run 'fish ~/MySetup/Linux/dots/install.fish' after reboot."
fi

# Print installation summary
echo ""
info "Installation completed successfully"
echo ""
echo "Configuration Summary:"
echo "  Username: $USERNAME"
echo "  Secure Boot: $([ $SECURE_BOOT -eq 1 ] && echo 'Enabled' || echo 'Disabled')"
echo "  NVIDIA Drivers: $([ $NVIDIA -eq 1 ] && echo 'Enabled' || echo 'Disabled')"
echo "  CTF Tools: $([ $CTF_TOOLS -eq 1 ] && echo 'Enabled' || echo 'Disabled')"
echo ""

# Secure Boot warning
if [ $SECURE_BOOT -eq 1 ]; then
    warn "Secure Boot is enabled. After reboot:"
    warn "  1. Enter BIOS/UEFI settings"
    warn "  2. Enable Secure Boot Setup Mode"
    warn "  3. Boot into NixOS"
    warn "  4. Run: sudo sbctl enroll-keys"
    echo ""
fi

# Prompt for reboot
read -p "Reboot now? (y/N): " REBOOT
if [[ $REBOOT =~ ^[Yy]$ ]]; then
    info "Rebooting system..."
    sudo reboot
else
    info "Please reboot manually to apply all changes"
fi
