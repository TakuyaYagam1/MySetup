if status is-interactive
    # Starship custom prompt
    starship init fish | source

    # Direnv + Zoxide
    command -v direnv &>/dev/null && direnv hook fish | source
    command -v zoxide &>/dev/null && zoxide init fish --cmd cd | source

    # Better ls
    alias ls='eza --icons --group-directories-first -1'
    alias install='sudo nixos-rebuild switch --flake /etc/nixos#NixOS'
    alias update='cd /etc/nixos; and sudo nix flake update; and sudo nixos-rebuild switch --flake /etc/nixos#NixOS; and cd -'
    alias cleanup="sudo nix-collect-garbage -d && nix-collect-garbage && nix store optimise && sudo nix-store --gc --print-dead && sudo nix-store --gc"
    alias logout='systemctl restart display-manager.servic'
    alias suspend='systemctl suspend'
    alias hibernate='systemctl hibernate'

    # Abbrs
    abbr lg lazygit
    abbr ld lazydocker
    abbr gd 'git diff'
    abbr ga 'git add .'
    abbr gc 'git commit -am'
    abbr gl 'git log'
    abbr gs 'git status'
    abbr gst 'git stash'
    abbr gsp 'git stash pop'
    abbr gp 'git push'
    abbr gpl 'git pull'
    abbr gsw 'git switch'
    abbr gsm 'git switch main'
    abbr gb 'git branch'
    abbr gbd 'git branch -d'
    abbr gco 'git checkout'
    abbr gsh 'git show'
    abbr l ls
    abbr ll 'ls -l'
    abbr la 'ls -a'
    abbr lla 'ls -la'

    # Custom colours
    cat ~/.local/state/caelestia/sequences.txt 2>/dev/null

    # For jumping between prompts in foot terminal
    function mark_prompt_start --on-event fish_prompt
        echo -en "\e]133;A\e\\"
    end

    function wifi-connect
        set iface $argv[1]
        set ssid $argv[2]
        set pass $argv[3]
        sudo nmcli dev wifi connect "$ssid" password "$pass"
    end

    # Additional aliases
    alias ll='eza -la --icons --group-directories-first'
    alias la='eza -a --icons --group-directories-first'
    alias lt='eza --tree --level=2 --icons'
    alias optimize='sudo nix-store --optimize'
    alias reboot='systemctl reboot'
    alias poweroff='systemctl poweroff'
    alias dc='podman-compose'
    alias dps='podman ps'
    alias dpsa='podman ps -a'
    alias di='podman images'
    alias drm='podman rm'
    alias drmi='podman rmi'
    alias k='kubectl'
    alias kgp='kubectl get pods'
    alias kgs='kubectl get services'
    alias kgd='kubectl get deployments'
    alias kdp='kubectl describe pod'
    alias kl='kubectl logs'
    alias c='clear'
    alias q='exit'
    alias ..='cd ..'
    alias ...='cd ../..'
    alias h='history'
    alias bottles='flatpak run com.usebottles.bottles'
    alias jwt='node -e "console.log(require(\'crypto\').randomBytes(32).toString(\'hex\'))"'
    command -v bat &>/dev/null && alias cat='bat'
    command -v rg &>/dev/null && alias grep='rg'
    command -v fd &>/dev/null && alias find='fd'

end
