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

function reload-hyprland
    if not command -q hyprctl
        return 0
    end

    if not hyprctl monitors >/dev/null 2>&1
        return 0
    end

    echo "Reloading Hyprland..."
    if hyprctl reload >/dev/null 2>&1
        echo "Hyprland config reloaded"
    else
        echo "Hyprland reload failed, run 'hyprctl reload' manually"
    end
end

# Hypr
set hypr_dst $cfg/hypr
if test -e $hypr_dst
    read -P "Path '$hypr_dst' exists. Update from repo? [y/N] " ans
    if test "$ans" != y
        echo "Skipping $hypr_dst"
    else
        if command -q rsync
            mkdir -p $hypr_dst
            rsync -a --delete "$src/hypr/" "$hypr_dst/"
        else
            echo "rsync not found, falling back to replace-copy for $hypr_dst"
            set hypr_tmp (mktemp -d)
            cp -r "$src/hypr" "$hypr_tmp/hypr"
            rm -rf $hypr_dst
            mv "$hypr_tmp/hypr" $hypr_dst
            rm -rf -- $hypr_tmp
        end

        if test -f $hypr_dst/scripts/wsaction.fish
            chmod +x $hypr_dst/scripts/wsaction.fish
        end

        reload-hyprland
    end
else
    mkdir -p $cfg
    cp -r "$src/hypr" $hypr_dst

    if test -f $hypr_dst/scripts/wsaction.fish
        chmod +x $hypr_dst/scripts/wsaction.fish
    end

    reload-hyprland
end


# Zen Browser
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

# Fallback
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
        set sine_bootloader_tag "v0.1.4"
        set sine_engine_tag     "v2.3"

        echo "Installing Sine profile part (bootloader=$sine_bootloader_tag engine=$sine_engine_tag)..."
        set sine_tmp (mktemp -d)

        set bootloader_url "https://github.com/sineorg/bootloader/releases/download/$sine_bootloader_tag"
        set sine_url       "https://github.com/CosmoCreeper/Sine/releases/download/$sine_engine_tag"

        if curl -fsSL "$bootloader_url/profile.zip" -o "$sine_tmp/profile.zip" \
        && curl -fsSL "$sine_url/engine.zip"        -o "$sine_tmp/engine.zip"
            set sine_has_locales 0
            if curl -fsSL "$sine_url/locales.zip" -o "$sine_tmp/locales.zip" 2>/dev/null
                set sine_has_locales 1
            end

            mkdir -p $zen_chrome
            if command -q bsdtar
                bsdtar -xf "$sine_tmp/profile.zip"  -C $zen_chrome
                bsdtar -xf "$sine_tmp/engine.zip"   -C $zen_chrome
                if test $sine_has_locales -eq 1
                    bsdtar -xf "$sine_tmp/locales.zip"  -C $zen_chrome
                end
            else
                echo "bsdtar not found, falling back to unzip (less safe). Install libarchive."
                unzip -qo "$sine_tmp/profile.zip"  -d $zen_chrome
                unzip -qo "$sine_tmp/engine.zip"   -d $zen_chrome
                if test $sine_has_locales -eq 1
                    unzip -qo "$sine_tmp/locales.zip"  -d $zen_chrome
                end
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

echo "✅ Installation complete!"
