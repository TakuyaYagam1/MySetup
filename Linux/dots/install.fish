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