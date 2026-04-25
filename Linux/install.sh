#!/bin/bash

set -e
set -o pipefail

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

step "Starting interactive configuration"

while true; do

while true; do
    USERNAME=$(whiptail --inputbox "Enter username:" 10 50 "" 3>&1 1>&2 2>&3) || error "Configuration cancelled"
    # POSIX-portable username: lowercase letter/underscore start, alnum/dash/underscore body, max 32 chars.
    if [[ "$USERNAME" =~ ^[a-z_][a-z0-9_-]{0,31}$ ]]; then
        break
    fi
    whiptail --msgbox "Invalid username. Must match ^[a-z_][a-z0-9_-]{0,31}\$ (POSIX)." 10 60
done

while true; do
    FULL_NAME=$(whiptail --inputbox "Enter full name (used in users.users.<name>.description):" 10 60 "" 3>&1 1>&2 2>&3) || error "Configuration cancelled"
    [ -n "$FULL_NAME" ] && break
    whiptail --msgbox "Full name cannot be empty." 8 50
done

while true; do
    HOSTNAME=$(whiptail --inputbox "Enter hostname:" 10 50 "" 3>&1 1>&2 2>&3) || error "Configuration cancelled"
    # RFC 1123 single label: alnum at edges, dashes inside, max 63 chars.
    if [[ "$HOSTNAME" =~ ^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$ ]]; then
        break
    fi
    whiptail --msgbox "Invalid hostname. Must be RFC 1123 (alnum, dashes inside, no dots/spaces, max 63 chars)." 10 70
done

while true; do
    PASSWORD=$(whiptail --passwordbox "Enter password for $USERNAME:" 10 50 3>&1 1>&2 2>&3) || error "Configuration cancelled"
    PASSWORD_CONFIRM=$(whiptail --passwordbox "Confirm password:" 10 50 3>&1 1>&2 2>&3) || error "Configuration cancelled"
    [ "$PASSWORD" = "$PASSWORD_CONFIRM" ] && break
    whiptail --msgbox "Passwords do not match. Please try again." 8 50
done

DETECTED_TZ=$(curl -s --connect-timeout 5 https://ipapi.co/timezone 2>/dev/null || echo "UTC")
[ -z "$DETECTED_TZ" ] && DETECTED_TZ="UTC"
TIMEZONE=$(whiptail --inputbox "Enter timezone:\n(auto-detected from IP)" 10 60 "$DETECTED_TZ" 3>&1 1>&2 2>&3) || error "Configuration cancelled"

# Default city for the Caelestia weather widget - derived from the timezone tail.
# "Europe/Moscow" -> "Moscow", "America/New_York" -> "New York".
DEFAULT_WEATHER=$(echo "$TIMEZONE" | awk -F/ '{print $NF}' | tr _ ' ')
WEATHER_LOCATION=$(whiptail --inputbox "Enter city for weather widget (Caelestia shell):" 10 60 "$DEFAULT_WEATHER" 3>&1 1>&2 2>&3) || error "Configuration cancelled"

LOCALE=$(whiptail --menu "Select system locale:" 12 60 2 \
    "en_US.UTF-8" "English (US) - default" \
    "ru_RU.UTF-8" "Russian" \
    3>&1 1>&2 2>&3) || error "Configuration cancelled"

# Pick a sensible extraLocale default that differs from the primary one.
[ "$LOCALE" = "ru_RU.UTF-8" ] && DEFAULT_EXTRA="en_US.UTF-8" || DEFAULT_EXTRA="ru_RU.UTF-8"
while true; do
    EXTRA_LOCALE=$(whiptail --inputbox "Enter additional supported locale (extraLocale):" 10 60 "$DEFAULT_EXTRA" 3>&1 1>&2 2>&3) || error "Configuration cancelled"
    # Glibc locale format: ll_CC.UTF-8 (e.g. en_US.UTF-8, ru_RU.UTF-8).
    if [[ "$EXTRA_LOCALE" =~ ^[a-z]{2,3}_[A-Z]{2}\.UTF-8$ ]]; then
        break
    fi
    whiptail --msgbox "Invalid locale. Must match ^[a-z]{2,3}_[A-Z]{2}\\.UTF-8\$ (e.g. en_US.UTF-8)." 10 70
done

CONSOLE_KEYMAP=$(whiptail --menu "Select console keymap:" 14 60 4 \
    "us"                  "US English (default)" \
    "ruwin_alt_sh-UTF-8"  "Russian (Win, alt+shift toggle)" \
    "uk"                  "UK English" \
    "de"                  "German" \
    3>&1 1>&2 2>&3) || error "Configuration cancelled"

while true; do
    GIT_USERNAME=$(whiptail --inputbox "Enter Git user.name:" 10 50 "" 3>&1 1>&2 2>&3) || error "Configuration cancelled"
    [ -n "$GIT_USERNAME" ] && break
    whiptail --msgbox "Git user.name cannot be empty." 8 50
done

while true; do
    GIT_EMAIL=$(whiptail --inputbox "Enter Git user.email:" 10 50 "" 3>&1 1>&2 2>&3) || error "Configuration cancelled"
    if [[ "$GIT_EMAIL" =~ ^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$ ]]; then
        break
    fi
    whiptail --msgbox "Invalid email address." 8 50
done

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

while true; do
    PGADMIN_EMAIL=$(whiptail --inputbox "Enter pgAdmin admin email:" 10 60 "" 3>&1 1>&2 2>&3) || error "Configuration cancelled"
    if [[ "$PGADMIN_EMAIL" =~ ^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$ ]]; then
        break
    fi
    whiptail --msgbox "Invalid email address." 8 50
done

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
SUMMARY+="Full name:     $FULL_NAME\n"
SUMMARY+="Hostname:      $HOSTNAME\n"
SUMMARY+="Timezone:      $TIMEZONE\n"
SUMMARY+="Locale:        $LOCALE (extra: $EXTRA_LOCALE)\n"
SUMMARY+="Console map:   $CONSOLE_KEYMAP\n"
SUMMARY+="Weather:       $WEATHER_LOCATION\n"
SUMMARY+="Git name:      $GIT_USERNAME\n"
SUMMARY+="Git email:     $GIT_EMAIL\n"
SUMMARY+="Region:        $([ $RUSSIA -eq 1 ] && echo 'Russia (DataGrip & GoLand disabled)' || echo 'Other')\n"
SUMMARY+="Secure Boot:   $([ $SECURE_BOOT -eq 1 ] && echo 'Enabled' || echo 'Disabled')\n"
SUMMARY+="GPU Driver:    $GPU\n"
SUMMARY+="pgAdmin Email: $PGADMIN_EMAIL\n"
SUMMARY+="CTF Tools:     $([ $CTF_TOOLS -eq 1 ] && echo 'Enabled' || echo 'Disabled')\n"
SUMMARY+="OmniRouter:    $([ $OMNIROUTER -eq 1 ] && echo 'Enabled (port 20128)' || echo 'Disabled')\n"
SUMMARY+="Zapret:        $([ $ZAPRET -eq 1 ] && echo "Enabled ($ZAPRET_CONFIG)" || echo 'Disabled')\n\n"
SUMMARY+="Proceed with installation?"

# whiptail returns 0=Yes, 1=No, 255=Esc/Ctrl-C; handle cancel explicitly.
SUMMARY_RC=0
whiptail --yesno "$SUMMARY" 22 70 3>&1 1>&2 2>&3 || SUMMARY_RC=$?
case $SUMMARY_RC in
    0)   break ;;
    1)   warn "Restarting configuration..." ;;
    *)   error "Configuration cancelled" ;;
esac

done

# Create temporary directory for modified files
TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

step "Generating hashed-password.nix"

# --rounds=656000 matches the Yescrypt-equivalent strength recommended by mkpasswd upstream;
# the legacy default of 5000 SHA-512 rounds is considered weak in 2025+.
HASHED_PASSWORD=$(printf '%s' "$PASSWORD" | nix-shell -p mkpasswd --run "mkpasswd -sm sha-512 --rounds=656000")
[ -n "$HASHED_PASSWORD" ] || error "mkpasswd produced empty hash"

# A dedicated file imported from hosts/NixOS/default.nix. Kept separate so the
# sensitive hash never lands in variables.nix (which is tracked by git).
cat > "$TEMP_DIR/hashed-password.nix" << EOF
{ config, ... }:

{
  users.users.\${config.var.username}.initialHashedPassword = "${HASHED_PASSWORD}";
}
EOF

step "Patching hosts/NixOS/variables.nix (single source of truth)"

cp NixOS/hosts/NixOS/variables.nix "$TEMP_DIR/variables.nix"

# All values consumed via config.var.* live here. Keep paths inside matching
# the real user home - other modules (programs.nh.flake, home.homeDirectory,
# etc.) derive from these.
# Note: scoped seds for git.{username,email} (below) must run AFTER global username/email seds.
sed -E -i "s|hostname = \"[^\"]+\";|hostname = \"${HOSTNAME}\";|"         "$TEMP_DIR/variables.nix"
sed -E -i "s|username = \"[^\"]+\";|username = \"${USERNAME}\";|"          "$TEMP_DIR/variables.nix"
sed -E -i "s|fullName = \"[^\"]+\";|fullName = \"${FULL_NAME}\";|"         "$TEMP_DIR/variables.nix"
sed -E -i "s|configDirectory = \"[^\"]+\";|configDirectory = \"${SCRIPT_DIR}/NixOS\";|" "$TEMP_DIR/variables.nix"
sed -E -i "s|homeDirectory = \"[^\"]+\";|homeDirectory = \"/home/${USERNAME}\";|" "$TEMP_DIR/variables.nix"
sed -E -i "s|timeZone = \"[^\"]+\";|timeZone = \"${TIMEZONE}\";|"          "$TEMP_DIR/variables.nix"
sed -E -i "s|defaultLocale = \"[^\"]+\";|defaultLocale = \"${LOCALE}\";|"  "$TEMP_DIR/variables.nix"
sed -E -i "s|extraLocale = \"[^\"]+\";|extraLocale = \"${EXTRA_LOCALE}\";|" "$TEMP_DIR/variables.nix"
sed -E -i "s|consoleKeyMap = \"[^\"]+\";|consoleKeyMap = \"${CONSOLE_KEYMAP}\";|" "$TEMP_DIR/variables.nix"
sed -E -i "s|weatherLocation = \"[^\"]+\";|weatherLocation = \"${WEATHER_LOCATION}\";|" "$TEMP_DIR/variables.nix"
# git.{username,email} live inside an attrset - scope match by the surrounding key
sed -E -i "/git = \{/,/\};/ s|username = \"[^\"]+\";|username = \"${GIT_USERNAME}\";|" "$TEMP_DIR/variables.nix"
sed -E -i "/git = \{/,/\};/ s|email = \"[^\"]+\";|email = \"${GIT_EMAIL}\";|"          "$TEMP_DIR/variables.nix"

step "Patching hosts/NixOS/default.nix (module toggles)"

cp NixOS/hosts/NixOS/default.nix "$TEMP_DIR/default.nix"

# Secure Boot
if [ $SECURE_BOOT -eq 1 ]; then
    info "Enabling Secure Boot (Lanzaboote)"
    sed -i 's|    # \.\./\.\./system/boot/secure\.nix|    ../../system/boot/secure.nix|'      "$TEMP_DIR/default.nix"
    # Plymouth conflicts with Lanzaboote signed initrd
    sed -i 's|^\(    \)\.\./\.\./system/boot/plymouth\.nix|\1# ../../system/boot/plymouth.nix|' "$TEMP_DIR/default.nix"
else
    sed -i 's|^\(    \)\.\./\.\./system/boot/secure\.nix|\1# ../../system/boot/secure.nix|'   "$TEMP_DIR/default.nix"
fi

# NVIDIA drivers
if [ "$GPU" = "nvidia" ]; then
    info "Enabling NVIDIA drivers"
    sed -i 's|    # \.\./\.\./system/nvidia-drivers\.nix|    ../../system/nvidia-drivers.nix|' "$TEMP_DIR/default.nix"
else
    sed -i 's|^\(    \)\.\./\.\./system/nvidia-drivers\.nix|\1# ../../system/nvidia-drivers.nix|' "$TEMP_DIR/default.nix"
fi

# CTF tools
if [ $CTF_TOOLS -eq 1 ]; then
    info "Enabling CTF tools"
    sed -i 's|    # \.\./\.\./packages/ctf\b|    ../../packages/ctf|' "$TEMP_DIR/default.nix"
else
    sed -i 's|^\(    \)\.\./\.\./packages/ctf\b|\1# ../../packages/ctf|' "$TEMP_DIR/default.nix"
fi

# OmniRouter (custom module) - toggle import + enable flag in system-services.nix
cp NixOS/services/system-services.nix "$TEMP_DIR/system-services.nix"
if [ $OMNIROUTER -eq 1 ]; then
    info "Enabling OmniRouter"
    sed -i 's|^[[:space:]]*#\?[[:space:]]*\.\./\.\./modules/omnirouter\.nix|    ../../modules/omnirouter.nix|' "$TEMP_DIR/default.nix"
    sed -i 's|^[[:space:]]*#\?[[:space:]]*services\.omnirouter\.enable = true;|  services.omnirouter.enable = true;|' "$TEMP_DIR/system-services.nix"
else
    sed -i 's|^[[:space:]]*#\?[[:space:]]*\.\./\.\./modules/omnirouter\.nix|    # ../../modules/omnirouter.nix|' "$TEMP_DIR/default.nix"
    sed -i 's|^[[:space:]]*#\?[[:space:]]*services\.omnirouter\.enable = true;|  # services.omnirouter.enable = true;|' "$TEMP_DIR/system-services.nix"
fi

# Zapret
cp NixOS/services/zapret.nix "$TEMP_DIR/zapret.nix"
if [ $ZAPRET -eq 1 ]; then
    info "Enabling Zapret with config: $ZAPRET_CONFIG"
    sed -i "s|config = \".*\";|config = \"${ZAPRET_CONFIG}\";|" "$TEMP_DIR/zapret.nix"
    sed -i 's|    # \.\./\.\./services/zapret\.nix|    ../../services/zapret.nix|' "$TEMP_DIR/default.nix"
else
    sed -i 's|^\(    \)\.\./\.\./services/zapret\.nix|\1# ../../services/zapret.nix|' "$TEMP_DIR/default.nix"
fi

# GPU initrd kernel module (placeholder-based - stable across refactors)
cp NixOS/system/hardware.nix "$TEMP_DIR/hardware.nix"
case "$GPU" in
    amd)
        sed -i 's| # GPU_MODULE_PLACEHOLDER||' "$TEMP_DIR/hardware.nix"
        ;;
    intel)
        sed -i 's|"amdgpu" \]; # GPU_MODULE_PLACEHOLDER|"i915" ];|' "$TEMP_DIR/hardware.nix"
        ;;
    nvidia|other)
        sed -i 's|boot\.initrd\.kernelModules = \[ "amdgpu" \]; # GPU_MODULE_PLACEHOLDER||' "$TEMP_DIR/hardware.nix"
        ;;
esac

# JetBrains Russia regional licensing - toggle in BOTH directions so re-running
# the installer with a different RUSSIA choice always converges to the right state.
# Always start from the pristine source, then comment out only when needed.
# jetbrains.datagrip lives in home/programs/user-apps/api-tools.nix (DB GUIs).
# jetbrains.goland lives in home/programs/user-apps/dev.nix (editors / IDEs).
# Both are blocked for Russian regional licensing - toggle them based on $RUSSIA.
cp NixOS/home/programs/user-apps/api-tools.nix "$TEMP_DIR/api-tools.nix"
cp NixOS/home/programs/user-apps/dev.nix       "$TEMP_DIR/dev.nix"
# Normalise: ensure both lines are uncommented first (idempotent).
sed -i -E 's|^([[:space:]]*)#[[:space:]]*jetbrains\.datagrip|\1jetbrains.datagrip|' "$TEMP_DIR/api-tools.nix"
sed -i -E 's|^([[:space:]]*)#[[:space:]]*jetbrains\.goland|\1jetbrains.goland|'     "$TEMP_DIR/dev.nix"
# Catch upstream renames before they silently no-op the sed above.
grep -qE '^[[:space:]]*jetbrains\.datagrip\b' "$TEMP_DIR/api-tools.nix" \
    || error "jetbrains.datagrip not found in api-tools.nix after normalisation; check upstream rename"
grep -qE '^[[:space:]]*jetbrains\.goland\b'   "$TEMP_DIR/dev.nix" \
    || error "jetbrains.goland not found in dev.nix after normalisation; check upstream rename"
if [ $RUSSIA -eq 1 ]; then
    info "Commenting out jetbrains.datagrip and jetbrains.goland (Russia)"
    sed -i -E 's|^([[:space:]]*)jetbrains\.datagrip|\1# jetbrains.datagrip|' "$TEMP_DIR/api-tools.nix"
    sed -i -E 's|^([[:space:]]*)jetbrains\.goland|\1# jetbrains.goland|'     "$TEMP_DIR/dev.nix"
    grep -qE '^[[:space:]]*#[[:space:]]*jetbrains\.datagrip\b' "$TEMP_DIR/api-tools.nix" \
        || error "Failed to comment jetbrains.datagrip"
    grep -qE '^[[:space:]]*#[[:space:]]*jetbrains\.goland\b'   "$TEMP_DIR/dev.nix" \
        || error "Failed to comment jetbrains.goland"
fi

step "Patching databases.nix (pgAdmin email: $PGADMIN_EMAIL)"

cp NixOS/services/databases.nix "$TEMP_DIR/databases.nix"
sed -E -i "s|initialEmail = \"[^\"]+\"|initialEmail = \"${PGADMIN_EMAIL}\"|" "$TEMP_DIR/databases.nix"

step "Copying configuration to /etc/nixos/"

# Use rsync (instead of `cp -rf`) so we don't clobber:
#   - /secrets/                              (root-level: pgadmin-password)
#   - hosts/NixOS/secrets/secrets.yaml       (system sops, user-created locally)
#   - home/secrets/secrets.yaml              (HM sops, user-created locally)
#   - flake.lock                             (preserved across re-runs; copied on first install)
#   - hardware-configuration.nix             (host-local, generated by nixos-generate-config)
#
# Skeleton files (sops.nix, default.nix, *.example, .sops.yaml) are always copied.
# rsync ships in the NixOS installer ISO and the live system; fall back to nix-shell otherwise.

# Preserve existing /etc/nixos/flake.lock; seed it on first install for reproducibility.
RSYNC_EXCLUDES=(
    --exclude='/secrets/'
    --exclude='hosts/NixOS/secrets/secrets.yaml'
    --exclude='home/secrets/secrets.yaml'
    --exclude='hardware-configuration.nix'
)
if [ -f /etc/nixos/flake.lock ]; then
    RSYNC_EXCLUDES+=(--exclude='flake.lock')
    info "Preserving existing /etc/nixos/flake.lock"
else
    info "Seeding /etc/nixos/flake.lock from repo (first install)"
fi

# rsync --delete wipes anything not in source tree; back up first to keep manual edits.
if [ -d /etc/nixos ] && [ -n "$(sudo ls -A /etc/nixos 2>/dev/null)" ]; then
    BACKUP="/etc/nixos.bak.$(date +%s)"
    info "Backing up existing /etc/nixos -> $BACKUP (rsync --delete will prune untracked files)"
    sudo cp -a /etc/nixos "$BACKUP"
fi

if command -v rsync >/dev/null 2>&1; then
    sudo rsync -a --delete "${RSYNC_EXCLUDES[@]}" NixOS/ /etc/nixos/
else
    info "rsync not found, falling back to nix-shell"
    sudo nix-shell -p rsync --run "rsync -a --delete ${RSYNC_EXCLUDES[*]} NixOS/ /etc/nixos/"
fi

sudo install -D -m 644 "$TEMP_DIR/variables.nix"       /etc/nixos/hosts/NixOS/variables.nix
sudo install -D -m 644 "$TEMP_DIR/default.nix"         /etc/nixos/hosts/NixOS/default.nix
sudo install -D -m 600 "$TEMP_DIR/hashed-password.nix" /etc/nixos/hosts/NixOS/hashed-password.nix
sudo install -D -m 644 "$TEMP_DIR/system-services.nix" /etc/nixos/services/system-services.nix
sudo install -D -m 644 "$TEMP_DIR/zapret.nix"          /etc/nixos/services/zapret.nix
sudo install -D -m 644 "$TEMP_DIR/hardware.nix"        /etc/nixos/system/hardware.nix
sudo install -D -m 644 "$TEMP_DIR/databases.nix"       /etc/nixos/services/databases.nix
sudo install -D -m 644 "$TEMP_DIR/api-tools.nix" /etc/nixos/home/programs/user-apps/api-tools.nix
sudo install -D -m 644 "$TEMP_DIR/dev.nix"       /etc/nixos/home/programs/user-apps/dev.nix
sudo chown -R root:root /etc/nixos/

# Enable the ./hashed-password.nix import (placeholder is commented out in repo).
sudo sed -i 's|    # \./hashed-password\.nix|    ./hashed-password.nix|' /etc/nixos/hosts/NixOS/default.nix

# Place hardware-configuration.nix next to default.nix so the flake can import it
# via a relative path. Absolute paths like /etc/nixos/hardware-configuration.nix
# are forbidden in pure flake evaluation.
if [ -f /etc/nixos/hardware-configuration.nix ]; then
    sudo install -D -m 644 /etc/nixos/hardware-configuration.nix /etc/nixos/hosts/NixOS/hardware-configuration.nix
elif [ ! -f /etc/nixos/hosts/NixOS/hardware-configuration.nix ]; then
    error "hardware-configuration.nix not found. Run 'sudo nixos-generate-config --root /' first."
fi

step "Writing secrets"

sudo mkdir -p /etc/nixos/secrets
sudo chmod 700 /etc/nixos/secrets

# pgAdmin password - referenced from databases.nix (initialPasswordFile)
printf '%s' "$PGADMIN_PASSWORD" | sudo tee /etc/nixos/secrets/pgadmin-password > /dev/null
sudo chmod 600 /etc/nixos/secrets/pgadmin-password
sudo chown -R root:root /etc/nixos/secrets/

step "Building NixOS configuration"

# The flake derives nixosConfigurations.<hostname> dynamically from
# variables.nix (see flake.nix `let hostname = ...`). Build target name
# always matches networking.hostName, so plain `nixos-rebuild switch --flake .`
# works too - but we pin it explicitly here for safety.
cd /etc/nixos

# dry-build first - surface flake eval errors without half-activating the system.
info "Validating flake (dry-build)..."
if ! sudo nixos-rebuild dry-build --flake ".#${HOSTNAME}"; then
    error "Flake evaluation failed. /etc/nixos has been written but NOT activated. Fix errors and re-run 'sudo nixos-rebuild switch --flake .#${HOSTNAME}'."
fi

if ! sudo nixos-rebuild switch --flake ".#${HOSTNAME}"; then
    error "NixOS rebuild failed. Check the error messages above."
fi

echo ""
info "Installation completed successfully"
echo ""
echo "Configuration Summary:"
echo "  Username:    $USERNAME"
echo "  Hostname:    $HOSTNAME"
echo "  Timezone:    $TIMEZONE"
echo "  Locale:      $LOCALE"
echo "  Git:         $GIT_USERNAME <$GIT_EMAIL>"
echo "  Secure Boot: $([ $SECURE_BOOT -eq 1 ] && echo 'Enabled' || echo 'Disabled')"
echo "  GPU Driver:  $GPU"
echo "  pgAdmin:     $PGADMIN_EMAIL"
echo "  CTF Tools:   $([ $CTF_TOOLS -eq 1 ] && echo 'Enabled' || echo 'Disabled')"
echo "  OmniRouter:  $([ $OMNIROUTER -eq 1 ] && echo 'Enabled' || echo 'Disabled')"
echo "  Zapret:      $([ $ZAPRET -eq 1 ] && echo "Enabled ($ZAPRET_CONFIG)" || echo 'Disabled')"
echo ""

if [ $SECURE_BOOT -eq 1 ]; then
    warn "Secure Boot next steps (after reboot):"
    warn "  1. sudo sbctl create-keys"
    warn "  2. sudo sbctl enroll-keys --microsoft"
    warn "  3. Reboot and verify Secure Boot is active"
    echo ""
fi

warn "After reboot, install remaining dotfiles (hypr/nvim/thunar/uwsm/vesktop/zen):"
warn "  fish ${SCRIPT_DIR}/dots/install.fish"
echo ""

read -rp "Reboot now? (y/N): " REBOOT
if [[ $REBOOT =~ ^[Yy]$ ]]; then
    info "Rebooting system..."
    sudo reboot
else
    info "Please reboot manually to apply all changes"
fi
