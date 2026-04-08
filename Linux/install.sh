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

# Resolve absolute script path before any exec
SCRIPT_PATH="$(cd "$(dirname "$0")" && pwd)/$(basename "$0")"
SCRIPT_DIR="$(dirname "$SCRIPT_PATH")"

# Verify script is run from Linux/ directory
cd "$SCRIPT_DIR"
[ -d "NixOS" ] || error "Run this script from the Linux/ directory of the repository"

# Verify whiptail is available
if ! command -v whiptail >/dev/null 2>&1; then
    info "whiptail not found, launching nix-shell..."
    exec nix-shell -p newt --run "bash '$SCRIPT_PATH'"
fi

# Interactive configuration
step "Starting interactive configuration"

while true; do

USERNAME=$(whiptail --inputbox "Enter username:" 10 50 "user" 3>&1 1>&2 2>&3) || error "Configuration cancelled"

while true; do
    PASSWORD=$(whiptail --passwordbox "Enter password for $USERNAME:" 10 50 3>&1 1>&2 2>&3) || error "Configuration cancelled"
    PASSWORD_CONFIRM=$(whiptail --passwordbox "Confirm password:" 10 50 3>&1 1>&2 2>&3) || error "Configuration cancelled"
    [ "$PASSWORD" = "$PASSWORD_CONFIRM" ] && break
    whiptail --msgbox "Passwords do not match. Please try again." 8 50
done

DETECTED_TZ=$(curl -s --connect-timeout 5 https://ipapi.co/timezone 2>/dev/null || echo "UTC")
[ -z "$DETECTED_TZ" ] && DETECTED_TZ="UTC"
TIMEZONE=$(whiptail --inputbox "Enter timezone:\n(auto-detected from IP)" 10 60 "$DETECTED_TZ" 3>&1 1>&2 2>&3) || error "Configuration cancelled"

LOCALE=$(whiptail --menu "Select system locale:" 12 60 2 \
    "en_US.UTF-8" "English (US) — default" \
    "ru_RU.UTF-8" "Russian (Русский)" \
    3>&1 1>&2 2>&3) || error "Configuration cancelled"

if whiptail --yesno "Enable Secure Boot (Lanzaboote)?\n\nNOTE: You must enable Setup Mode in BIOS BEFORE continuing." 12 60 3>&1 1>&2 2>&3; then
    SECURE_BOOT=1
else
    SECURE_BOOT=0
fi

GPU=$(whiptail --menu "Select GPU type:" 15 60 4 \
    "amd"    "AMD GPU (amdgpu driver)" \
    "intel"  "Intel integrated GPU (i915 driver)" \
    "nvidia" "NVIDIA proprietary drivers" \
    "other"  "Other / VM (no GPU module for Plymouth)" \
    3>&1 1>&2 2>&3) || error "Configuration cancelled"

if whiptail --yesno "Are you installing from Russia?\n\nThis will comment out jetbrains.datagrip (regional licensing restriction) and suggest enabling Zapret." 12 65 3>&1 1>&2 2>&3; then
    RUSSIA=1
else
    RUSSIA=0
fi

PGADMIN_EMAIL=$(whiptail --inputbox "Enter pgAdmin admin email:" 10 60 "admin@localhost.local" 3>&1 1>&2 2>&3) || error "Configuration cancelled"

while true; do
    PGADMIN_PASSWORD=$(whiptail --passwordbox "Enter pgAdmin admin password:" 10 50 3>&1 1>&2 2>&3) || error "Configuration cancelled"
    PGADMIN_PASSWORD_CONFIRM=$(whiptail --passwordbox "Confirm pgAdmin password:" 10 50 3>&1 1>&2 2>&3) || error "Configuration cancelled"
    [ "$PGADMIN_PASSWORD" = "$PGADMIN_PASSWORD_CONFIRM" ] && break
    whiptail --msgbox "Passwords do not match. Please try again." 8 50
done

if whiptail --yesno "Install CTF tools?" 10 50 3>&1 1>&2 2>&3; then
    CTF_TOOLS=1
else
    CTF_TOOLS=0
fi

if whiptail --yesno "Enable OmniRouter (Local LLM router)?\n\nRuns on localhost:20128" 10 60 3>&1 1>&2 2>&3; then
    OMNIROUTER=1
else
    OMNIROUTER=0
fi

ZAPRET=0
ZAPRET_CONFIG=""
ZAPRET_ARGS=""
[ $RUSSIA -eq 0 ] && ZAPRET_ARGS="--defaultno"
if whiptail $ZAPRET_ARGS --yesno "Enable Zapret (DPI bypass for Russia)?\n\nNeeded for YouTube, Discord, etc." 12 60 3>&1 1>&2 2>&3; then
    ZAPRET=1
    ZAPRET_CONFIG=$(whiptail --menu "Select Zapret config:" 30 70 20 \
        "general"                   "General (default)" \
        "general (FAKE_TLS_AUTO)"   "General - FAKE TLS AUTO" \
        "general (FAKE_TLS_AUTO_ALT)"  "General - FAKE TLS AUTO ALT" \
        "general (FAKE_TLS_AUTO_ALT2)" "General - FAKE TLS AUTO ALT2" \
        "general (FAKE_TLS_AUTO_ALT3)" "General - FAKE TLS AUTO ALT3" \
        "general (SIMPLE FAKE ALT)" "General - SIMPLE FAKE ALT" \
        "general (SIMPLE FAKE)"     "General - SIMPLE FAKE" \
        "general (SIMPLE_FAKE_ALT2)" "General - SIMPLE FAKE ALT2" \
        "general(ALT)"   "General ALT" \
        "general(ALT2)"  "General ALT2" \
        "general(ALT3)"  "General ALT3" \
        "general(ALT4)"  "General ALT4" \
        "general(ALT5)"  "General ALT5" \
        "general(ALT6)"  "General ALT6" \
        "general(ALT7)"  "General ALT7" \
        "general(ALT8)"  "General ALT8" \
        "general(ALT9)"  "General ALT9" \
        "general(ALT10)" "General ALT10" \
        "general(ALT11)" "General ALT11" \
        3>&1 1>&2 2>&3) || error "Configuration cancelled"
fi

# Generate configuration summary
SUMMARY="Configuration Summary:\n\n"
SUMMARY+="Username:      $USERNAME\n"
SUMMARY+="Timezone:      $TIMEZONE\n"
SUMMARY+="Locale:        $LOCALE\n"
SUMMARY+="Region:        $([ $RUSSIA -eq 1 ] && echo 'Russia (DataGrip & GoLand disabled)' || echo 'Other')\n"
SUMMARY+="Secure Boot:   $([ $SECURE_BOOT -eq 1 ] && echo 'Enabled' || echo 'Disabled')\n"
SUMMARY+="GPU Driver:    $GPU\n"
SUMMARY+="pgAdmin Email: $PGADMIN_EMAIL\n"
SUMMARY+="CTF Tools:     $([ $CTF_TOOLS -eq 1 ] && echo 'Enabled' || echo 'Disabled')\n"
SUMMARY+="OmniRouter:    $([ $OMNIROUTER -eq 1 ] && echo 'Enabled (port 20128)' || echo 'Disabled')\n"
SUMMARY+="Zapret:        $([ $ZAPRET -eq 1 ] && echo "Enabled ($ZAPRET_CONFIG)" || echo 'Disabled')\n\n"
SUMMARY+="Proceed with installation?"

if whiptail --yesno "$SUMMARY" 20 60 3>&1 1>&2 2>&3; then
    break
else
    warn "Restarting configuration..."
fi

done

# Create temporary directory for modified files
TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

step "Generating user configuration"

# Generate hashed password
HASHED_PASSWORD=$(printf '%s' "$PASSWORD" | nix-shell -p mkpasswd --run "mkpasswd -sm sha-512")

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

cp NixOS/configuration.nix "$TEMP_DIR/configuration.nix"

# Apply Secure Boot configuration
if [ $SECURE_BOOT -eq 1 ]; then
    info "Enabling Secure Boot configuration"
    # grub.nix stays imported — secure.nix overrides grub via mkForce, but EFI settings are still needed
    sed -i 's|    # \./system/boot/secure\.nix|    ./system/boot/secure.nix|' "$TEMP_DIR/configuration.nix"
    # Plymouth conflicts with Lanzaboote signed initrd
    sed -i 's|    \./system/boot/plymouth\.nix|    # ./system/boot/plymouth.nix|' "$TEMP_DIR/configuration.nix"
else
    sed -i 's|    \./system/boot/secure\.nix|    # ./system/boot/secure.nix|' "$TEMP_DIR/configuration.nix"
fi

# Apply GPU configuration
if [ "$GPU" = "nvidia" ]; then
    info "Enabling NVIDIA drivers"
    sed -i 's|    # \./system/nvidia-drivers\.nix|    ./system/nvidia-drivers.nix|' "$TEMP_DIR/configuration.nix"
else
    sed -i 's|    \./system/nvidia-drivers\.nix|    # ./system/nvidia-drivers.nix|' "$TEMP_DIR/configuration.nix"
fi

# Patch initrd kernel module in hardware.nix based on GPU
cp NixOS/system/hardware.nix "$TEMP_DIR/hardware.nix"
case "$GPU" in
    amd)
        # amdgpu is already the default placeholder value — no change needed
        sed -i 's| # GPU_MODULE_PLACEHOLDER||' "$TEMP_DIR/hardware.nix"
        ;;
    intel)
        sed -i 's|"amdgpu" ]; # GPU_MODULE_PLACEHOLDER|"i915" ];|' "$TEMP_DIR/hardware.nix"
        ;;
    nvidia)
        # initrd modules are handled by nvidia-drivers.nix
        sed -i 's|boot.initrd.kernelModules = \[ "amdgpu" \]; # GPU_MODULE_PLACEHOLDER||' "$TEMP_DIR/hardware.nix"
        ;;
    other)
        sed -i 's|boot.initrd.kernelModules = \[ "amdgpu" \]; # GPU_MODULE_PLACEHOLDER||' "$TEMP_DIR/hardware.nix"
        ;;
esac

# Apply CTF tools configuration
if [ $CTF_TOOLS -eq 1 ]; then
    info "Enabling CTF tools"
    sed -i 's|    # \./packages/ctf-tools\.nix|    ./packages/ctf-tools.nix|' "$TEMP_DIR/configuration.nix"
else
    sed -i 's|    \./packages/ctf-tools\.nix|    # ./packages/ctf-tools.nix|' "$TEMP_DIR/configuration.nix"
fi

# Apply OmniRouter configuration
cp NixOS/services/system-services.nix "$TEMP_DIR/system-services.nix"
if [ $OMNIROUTER -eq 1 ]; then
    info "Enabling OmniRouter"
    sed -i 's|^[[:space:]]*#\?[[:space:]]*\./modules/omnirouter\.nix|    ./modules/omnirouter.nix|' "$TEMP_DIR/configuration.nix"
    sed -i 's|^[[:space:]]*#\?[[:space:]]*services\.omnirouter\.enable = true;|  services.omnirouter.enable = true;|' "$TEMP_DIR/system-services.nix"
else
    sed -i 's|^[[:space:]]*#\?[[:space:]]*\./modules/omnirouter\.nix|    # ./modules/omnirouter.nix|' "$TEMP_DIR/configuration.nix"
    sed -i 's|^[[:space:]]*#\?[[:space:]]*services\.omnirouter\.enable = true;|  # services.omnirouter.enable = true;|' "$TEMP_DIR/system-services.nix"
fi

# Comment out JetBrains tools unavailable in Russia (regional licensing)
if [ $RUSSIA -eq 1 ]; then
    info "Commenting out jetbrains.datagrip and jetbrains.goland (Russia regional restriction)"
    cp NixOS/home/apps.nix "$TEMP_DIR/apps.nix"
    sed -i 's|jetbrains\.datagrip|# jetbrains.datagrip|' "$TEMP_DIR/apps.nix"
    sed -i 's|jetbrains\.goland|# jetbrains.goland|' "$TEMP_DIR/apps.nix"
fi

# Apply Zapret configuration
cp NixOS/services/zapret.nix "$TEMP_DIR/zapret.nix"
if [ $ZAPRET -eq 1 ]; then
    info "Enabling Zapret with config: $ZAPRET_CONFIG"
    sed -i "s|config = \".*\";|config = \"${ZAPRET_CONFIG}\";|" "$TEMP_DIR/zapret.nix"
    sed -i 's|    # \./services/zapret\.nix|    ./services/zapret.nix|' "$TEMP_DIR/configuration.nix"
else
    sed -i 's|    \./services/zapret\.nix|    # ./services/zapret.nix|' "$TEMP_DIR/configuration.nix"
fi

step "Patching databases.nix for pgAdmin email: $PGADMIN_EMAIL"

cp NixOS/services/databases.nix "$TEMP_DIR/databases.nix"
sed -E -i "s|initialEmail = \"[^\"]+\"|initialEmail = \"${PGADMIN_EMAIL}\"|" "$TEMP_DIR/databases.nix"

step "Patching nix files for username: $USERNAME"

cp NixOS/flake.nix "$TEMP_DIR/flake.nix"
sed -E -i "s|users\.[A-Za-z0-9._-]+ = import \./home/home\.nix|users.${USERNAME} = import ./home/home.nix|g" "$TEMP_DIR/flake.nix"

cp NixOS/home/home.nix "$TEMP_DIR/home.nix"
sed -E -i "s|home\.username = \"[^\"]+\"|home.username = \"${USERNAME}\"|g" "$TEMP_DIR/home.nix"
sed -E -i "s|home\.homeDirectory = \"/home/[^\"]+\"|home.homeDirectory = \"/home/${USERNAME}\"|g" "$TEMP_DIR/home.nix"

cp NixOS/home/caelestia.nix "$TEMP_DIR/caelestia.nix"
sed -E -i "s|\"pkill\" \"-KILL\" \"-u\" \"[^\"]+\"|\"pkill\" \"-KILL\" \"-u\" \"${USERNAME}\"|g" "$TEMP_DIR/caelestia.nix"

cp NixOS/services/sddm.nix "$TEMP_DIR/sddm.nix"
sed -E -i "s|sddm/faces/[^\"]+\.face\.icon|sddm/faces/${USERNAME}.face.icon|g" "$TEMP_DIR/sddm.nix"

cp NixOS/system/settings.nix "$TEMP_DIR/settings.nix"
sed -E -i "s|trusted-users = \[ \"root\" \"@wheel\" \"[^\"]+\" \];|trusted-users = [ \"root\" \"@wheel\" \"${USERNAME}\" ];|" "$TEMP_DIR/settings.nix"

cp NixOS/programs/system-tools.nix "$TEMP_DIR/system-tools.nix"
sed -E -i "s|flake = \"/home/[^\"]+\"|flake = \"/home/${USERNAME}/MySetup/Linux/NixOS\"|" "$TEMP_DIR/system-tools.nix"

step "Patching locale.nix with timezone: $TIMEZONE and locale: $LOCALE"

cp NixOS/system/locale.nix "$TEMP_DIR/locale.nix"
sed -i "s|time\.timeZone = \".*\"|time.timeZone = \"${TIMEZONE}\"|" "$TEMP_DIR/locale.nix"
sed -i "s|i18n\.defaultLocale = \".*\"|i18n.defaultLocale = \"${LOCALE}\"|" "$TEMP_DIR/locale.nix"

step "Copying configuration to /etc/nixos/"

sudo cp -rf NixOS/* /etc/nixos/
sudo cp "$TEMP_DIR/user.nix"            /etc/nixos/users/user.nix
sudo cp "$TEMP_DIR/configuration.nix"  /etc/nixos/configuration.nix
sudo cp "$TEMP_DIR/flake.nix"          /etc/nixos/flake.nix
sudo cp "$TEMP_DIR/hardware.nix"       /etc/nixos/system/hardware.nix
sudo cp "$TEMP_DIR/system-services.nix" /etc/nixos/services/system-services.nix
sudo cp "$TEMP_DIR/databases.nix"      /etc/nixos/services/databases.nix
sudo cp "$TEMP_DIR/zapret.nix"         /etc/nixos/services/zapret.nix
sudo cp "$TEMP_DIR/sddm.nix"           /etc/nixos/services/sddm.nix
sudo cp "$TEMP_DIR/locale.nix"         /etc/nixos/system/locale.nix
sudo cp "$TEMP_DIR/home.nix"           /etc/nixos/home/home.nix
sudo cp "$TEMP_DIR/caelestia.nix"      /etc/nixos/home/caelestia.nix
sudo cp "$TEMP_DIR/settings.nix"       /etc/nixos/system/settings.nix
sudo cp "$TEMP_DIR/system-tools.nix"   /etc/nixos/programs/system-tools.nix
[ $RUSSIA -eq 1 ] && sudo cp "$TEMP_DIR/apps.nix" /etc/nixos/home/apps.nix
sudo chown -R root:root /etc/nixos/

step "Creating secrets files"

sudo mkdir -p /etc/nixos/secrets
sudo chmod 700 /etc/nixos/secrets

# pgAdmin password — written to file referenced by databases.nix
printf '%s' "$PGADMIN_PASSWORD" | sudo tee /etc/nixos/secrets/pgadmin-password > /dev/null
sudo chmod 600 /etc/nixos/secrets/pgadmin-password

sudo chown -R root:root /etc/nixos/secrets/

step "Building NixOS configuration"

cd /etc/nixos
if ! sudo nixos-rebuild switch --flake .#NixOS; then
    error "NixOS rebuild failed. Check the error messages above."
fi

# Print installation summary
echo ""
info "Installation completed successfully"
echo ""
echo "Configuration Summary:"
echo "  Username:    $USERNAME"
echo "  Timezone:    $TIMEZONE"
echo "  Locale:      $LOCALE"
echo "  Secure Boot: $([ $SECURE_BOOT -eq 1 ] && echo 'Enabled' || echo 'Disabled')"
echo "  GPU Driver:  $GPU"
echo "  pgAdmin:     $PGADMIN_EMAIL"
echo "  CTF Tools:   $([ $CTF_TOOLS -eq 1 ] && echo 'Enabled' || echo 'Disabled')"
echo "  OmniRouter:  $([ $OMNIROUTER -eq 1 ] && echo 'Enabled' || echo 'Disabled')"
echo "  Zapret:      $([ $ZAPRET -eq 1 ] && echo "Enabled ($ZAPRET_CONFIG)" || echo 'Disabled')"
echo ""

# Secure Boot post-install instructions
if [ $SECURE_BOOT -eq 1 ]; then
    warn "Secure Boot next steps (after reboot):"
    warn "  1. sudo sbctl create-keys"
    warn "  2. sudo sbctl enroll-keys --microsoft"
    warn "  3. Reboot and verify Secure Boot is active"
    echo ""
fi

warn "After reboot, install dotfiles:"
warn "  fish /etc/nixos/dots/install.fish"
echo ""

# Prompt for reboot
read -rp "Reboot now? (y/N): " REBOOT
if [[ $REBOOT =~ ^[Yy]$ ]]; then
    info "Rebooting system..."
    sudo reboot
else
    info "Please reboot manually to apply all changes"
fi
