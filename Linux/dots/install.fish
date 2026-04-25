#!/usr/bin/env fish

set script_dir (path dirname (realpath (status filename)))

set src $script_dir
set cfg ~/.config

function confirm-overwrite
    set -l target $argv[1]

    if test -e $target
        read -P "Path '$target' exists. Overwrite? [y/N] " ans
        if test "$ans" != y
            echo "Skipping $target"
            return 1
        end
        echo "Removing old $target..."
        rm -rf $target
    end

    return 0
end

# NOTE: avatar (~/.face), thunar, uwsm, vesktop are managed declaratively
# by home-manager (see NixOS/home/home.nix and NixOS/home/programs/*.nix).
# Do NOT copy them here - home-manager renders them on activation.
#
# fish, foot, btop, cava, fastfetch, starship - same: HM-managed.


# Hypr
if confirm-overwrite $cfg/hypr
    mkdir -p $cfg
    cp -r $src/hypr $cfg/hypr
end


# Zen Browser
# Profile can live in ~/.zen/ (upstream) or ~/.config/zen/ (NixOS flake)
set zen_profile_dir ""
for base_dir in ~/.zen ~/.config/zen
    for d in $base_dir/*/
        if string match -q "*.default*" $d; or string match -q "*Default*" $d
            set zen_profile_dir (string trim -r -c / $d)
            break
        end
    end
    if test -n "$zen_profile_dir"
        break
    end
end

# Fallback: if no *.default* profile found, take the first existing profile dir
if test -z "$zen_profile_dir"
    for base_dir in ~/.zen ~/.config/zen
        for d in $base_dir/*/
            if test -d $d
                set zen_profile_dir (string trim -r -c / $d)
                break
            end
        end
        if test -n "$zen_profile_dir"
            break
        end
    end
end

if test -z "$zen_profile_dir"
    echo "Zen Browser profile not found (checked ~/.zen/ and ~/.config/zen/)."
    echo " -> Launch Zen Browser once to create the profile, then re-run this script."
else
    set zen_chrome $zen_profile_dir/chrome

    read -P "Install Zen Browser theme (Catppuccin Macchiato)? [y/N] " ans
    if test "$ans" = y
        if confirm-overwrite $zen_chrome
            mkdir -p $zen_chrome
            cp -r $src/zen/chrome/. $zen_chrome/
            echo "Zen Browser theme installed to $zen_chrome"
        end
    end

    read -P "Install Sine mod manager profile part for Zen Browser? [y/N] " ans
    if test "$ans" = y
        # Sine has TWO parts:
        #   1. Bootloader (sine-config.js + autoconfig.js) - installed system-wide
        #      via NixOS overlay in NixOS/packages/zen-browser.nix. DO NOT
        #      duplicate that here.
        #   2. Per-profile chrome/ files (profile.zip + engine.zip + locales.zip)
        #      - must live in the user profile, cannot ship via Nix.
        # This block handles ONLY part #2.
        #
        # Versions are pinned to avoid drift between bootloader (Nix-pinned)
        # and the profile chrome/ files. Bump together when updating.
        set sine_bootloader_tag "v1.0.4"
        set sine_engine_tag     "v1.0.0"

        echo "Installing Sine profile part (bootloader=$sine_bootloader_tag engine=$sine_engine_tag)..."
        set sine_tmp (mktemp -d)

        set bootloader_url "https://github.com/sineorg/bootloader/releases/download/$sine_bootloader_tag"
        set sine_url       "https://github.com/CosmoCreeper/Sine/releases/download/$sine_engine_tag"

        if curl -fsSL "$bootloader_url/profile.zip" -o "$sine_tmp/profile.zip" \
        && curl -fsSL "$sine_url/engine.zip"        -o "$sine_tmp/engine.zip" \
        && curl -fsSL "$sine_url/locales.zip"       -o "$sine_tmp/locales.zip"
            mkdir -p $zen_chrome
            # H8: bsdtar refuses absolute paths and `..` traversal by default,
            # protecting against zip-slip from compromised release assets.
            if command -q bsdtar
                bsdtar -xf "$sine_tmp/profile.zip"  -C $zen_chrome
                bsdtar -xf "$sine_tmp/engine.zip"   -C $zen_chrome
                bsdtar -xf "$sine_tmp/locales.zip"  -C $zen_chrome
            else
                echo "bsdtar not found, falling back to unzip (less safe). Install libarchive."
                unzip -qo "$sine_tmp/profile.zip"  -d $zen_chrome
                unzip -qo "$sine_tmp/engine.zip"   -d $zen_chrome
                unzip -qo "$sine_tmp/locales.zip"  -d $zen_chrome
            end
            echo "Sine profile part installed to $zen_chrome"
            echo "Go to about:support and click 'Clear Startup Cache', then restart Zen Browser"
        else
            echo "Failed to download Sine files (check tags above), skipping"
        end

        rm -rf -- $sine_tmp
    end
end


# sing-box binary for v2rayN
echo "Setting up sing-box for v2rayN..."
set singbox_dst ~/.local/share/v2rayN/bin/sing_box/sing-box
if not test -f $singbox_dst
    if command -q sing-box
        mkdir -p ~/.local/share/v2rayN/bin/sing_box
        install -m 755 (which sing-box) $singbox_dst
        echo "sing-box copied to v2rayN bin folder"
    else
        echo "sing-box not found in PATH, skipping. Add it to systemPackages and rerun"
    end
else
    echo "sing-box already present in v2rayN bin folder"
end

if confirm-overwrite $cfg/nvim
    # H9: data dirs (plugins, swap, undo, sessions) are *separate* from config.
    # Wiping them is destructive (LazyVim plugin cache, project sessions) and
    # must be opt-in.
    read -P "Also wipe ~/.local/share/nvim, ~/.local/state/nvim, ~/.cache/nvim? [y/N] " ans
    if test "$ans" = y
        echo "Cleaning all Neovim data..."
        rm -rf ~/.local/share/nvim
        rm -rf ~/.local/state/nvim
        rm -rf ~/.cache/nvim
    end

    mkdir -p $cfg
    cp -r $src/nvim $cfg/nvim
    echo "Neovim config installed. Run 'nvim' to download plugins."
end

# Make Hypr scripts executable
if test -f $cfg/hypr/scripts/wsaction.fish
    chmod +x $cfg/hypr/scripts/wsaction.fish
end

# Reload Hyprland once at the end (no-op if Hyprland is not running)
hyprctl reload >/dev/null 2>&1

echo "✅ Installation complete!"
