#!/usr/bin/env fish

set script_dir (path dirname (realpath (status filename)))
set nixos_root /etc/nixos

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

# Avatar -> ~/.face
set avatar_src (ls $src/avatar.gif $src/avatar.* 2>/dev/null | head -1)
if test -n "$avatar_src"
    echo "Using avatar: $avatar_src"
    cp $avatar_src ~/.face
    chmod 644 ~/.face
else
    echo "No avatar.* found in $src, skipping ~/.face"
end


# Hypr
if confirm-overwrite $cfg/hypr
    mkdir -p $cfg
    cp -r $src/hypr $cfg/hypr
end


# Reload Hyprland a few times to apply changes
for i in 1 2 3
    hyprctl reload >/dev/null 2>&1
end


# Starship
if confirm-overwrite $cfg/starship.toml
    mkdir -p $cfg
    cp $src/starship.toml $cfg/starship.toml
end


# Foot
if confirm-overwrite $cfg/foot
    mkdir -p $cfg
    cp -r $src/foot $cfg/foot
end


# Fish
if confirm-overwrite $cfg/fish
    mkdir -p $cfg
    cp -r $src/fish $cfg/fish
end


# Fastfetch
if confirm-overwrite $cfg/fastfetch
    mkdir -p $cfg
    cp -r $src/fastfetch $cfg/fastfetch
end


# Thunar
if confirm-overwrite $cfg/Thunar
    mkdir -p $cfg
    cp -r $src/thunar $cfg/Thunar
end


# Uwsm
if confirm-overwrite $cfg/uwsm
    mkdir -p $cfg
    cp -r $src/uwsm $cfg/uwsm
end


# Btop
if confirm-overwrite $cfg/btop
    mkdir -p $cfg
    cp -r $src/btop $cfg/btop
end


# Cava
if confirm-overwrite $cfg/cava
    mkdir -p $cfg/cava
    cp $src/cava/config $cfg/cava/config
end


# Vesktop (Vencord QuickCSS - Catppuccin Macchiato)
set vesktop_css $cfg/vesktop/settings/quickCss.css
if confirm-overwrite $vesktop_css
    mkdir -p $cfg/vesktop/settings
    cp $src/vesktop/quickCss.css $vesktop_css
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

    read -P "Install Sine mod manager for Zen Browser? [y/N] " ans
    if test "$ans" = y
        # Sine profile part: profile.zip + engine.zip + locales.zip
        # The bootloader (program.zip) is applied via NixOS overlay in zen-browser.nix
        echo "Installing Sine mod manager (profile part)..."
        set sine_tmp (mktemp -d)

        set bootloader_url "https://github.com/sineorg/bootloader/releases/latest/download"
        set sine_url "https://github.com/CosmoCreeper/Sine/releases/latest/download"

        if curl -fsSL "$bootloader_url/profile.zip" -o "$sine_tmp/profile.zip" \
        && curl -fsSL "$sine_url/engine.zip"        -o "$sine_tmp/engine.zip" \
        && curl -fsSL "$sine_url/locales.zip"       -o "$sine_tmp/locales.zip"
            mkdir -p $zen_chrome
            unzip -qo "$sine_tmp/profile.zip"  -d $zen_chrome
            unzip -qo "$sine_tmp/engine.zip"   -d $zen_chrome
            unzip -qo "$sine_tmp/locales.zip"  -d $zen_chrome
            echo "Sine installed to $zen_chrome"
            echo "Go to about:support and click 'Clear Startup Cache', then restart Zen Browser"
        else
            echo "Failed to download Sine files, skipping"
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
    echo "Cleaning all Neovim data..."
    rm -rf ~/.local/share/nvim
    rm -rf ~/.local/state/nvim
    rm -rf ~/.cache/nvim

    mkdir -p $cfg
    cp -r $src/nvim $cfg/nvim
    echo "Neovim config installed. Run 'nvim' to download plugins."
end

# Make Hypr scripts executable
if test -f $cfg/hypr/scripts/wsaction.fish
    chmod +x $cfg/hypr/scripts/wsaction.fish
end

echo "✅ Installation complete!"
