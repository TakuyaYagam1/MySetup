{ config, pkgs, lib, var, ... }:

{
  programs.fish = {
    enable = true;

    shellAliases = {
      # listing
      ls = "eza --icons --group-directories-first -1";
      ll = "eza -la --icons --group-directories-first";
      la = "eza -a --icons --group-directories-first";
      lt = "eza --tree --level=2 --icons";

      # system
      nixos-install = "sudo nixos-rebuild switch --flake /etc/nixos#NixOS";
      nixos-update = "pushd /etc/nixos; and sudo nix flake update; and sudo nixos-rebuild switch --flake .#NixOS; and popd";
      nixos-ida = "nix-store --add-fixed sha256 ~/MySetup/Linux/ida-pro_93_x64linux.run";
      cleanup = "sudo nix-collect-garbage -d && nix-collect-garbage && nix store optimise && sudo nix-store --gc --print-dead && sudo nix-store --gc";
      optimize = "sudo nix-store --optimise";
      logout = "systemctl restart display-manager.service";
      suspend = "systemctl suspend";
      hibernate = "systemctl hibernate";
      reboot = "systemctl reboot";
      poweroff = "systemctl poweroff";

      # containers
      dc = "podman-compose";
      dps = "podman ps";
      dpsa = "podman ps -a";
      di = "podman images";
      drm = "podman rm";
      drmi = "podman rmi";

      # kubernetes
      k = "kubectl";
      kgp = "kubectl get pods";
      kgs = "kubectl get services";
      kgd = "kubectl get deployments";
      kdp = "kubectl describe pod";
      kl = "kubectl logs";

      # misc
      c = "clear";
      q = "exit";
      ".." = "cd ..";
      "..." = "cd ../..";
      h = "history";
      bottles = "flatpak run com.usebottles.bottles";
      randhex = "openssl rand -hex 32";
    };

    shellAbbrs = {
      lg = "lazygit";
      ld = "lazydocker";
      gd = "git diff";
      ga = "git add .";
      gc = "git commit -am";
      gl = "git log";
      gs = "git status";
      gst = "git stash";
      gsp = "git stash pop";
      gp = "git push";
      gpl = "git pull";
      gsw = "git switch";
      gsm = "git switch main";
      gb = "git branch";
      gbd = "git branch -d";
      gco = "git checkout";
      gsh = "git show";
      l = "ls";
    };

    interactiveShellInit = ''
      # direnv + zoxide
      command -v direnv >/dev/null 2>&1 && direnv hook fish | source
      command -v zoxide >/dev/null 2>&1 && zoxide init fish --cmd cd | source

      # Caelestia terminal colour sequences
      if test -f ~/.local/state/caelestia/sequences.txt
          cat ~/.local/state/caelestia/sequences.txt
      end

      # Fish syntax colours — pinned in hex so Caelestia's muted ANSI palette
      # doesn't make paths/commands blend with the background.
      set -g fish_color_command       B3BCFD --bold
      set -g fish_color_keyword        A178FF --bold
      set -g fish_color_param          E8D3DE
      set -g fish_color_option         FFDCF2
      set -g fish_color_quote          FFDCF2
      set -g fish_color_redirection    ADA0ED --bold
      set -g fish_color_end            A178FF
      set -g fish_color_operator       FFDCF2
      set -g fish_color_escape         44DEF5
      set -g fish_color_autosuggestion 6C6F85
      set -g fish_color_comment        7F7596 --italics
      set -g fish_color_error          FF6B6B --bold
      set -g fish_color_match          --background=4B3F6B
      set -g fish_color_search_match   --background=4B3F6B
      set -g fish_color_selection      --background=35223E
      set -g fish_color_history_current --bold
      set -g fish_pager_color_progress 6C6F85
      set -g fish_pager_color_prefix   B3BCFD --bold
      set -g fish_pager_color_completion E8D3DE
      set -g fish_pager_color_description 7F7596

      # Smart aliases (only if tool exists - avoid breaking `cat` in recovery)
      command -v bat >/dev/null 2>&1 && alias cat='bat'
      command -v rg  >/dev/null 2>&1 && alias grep='rg'
      command -v fd  >/dev/null 2>&1 && alias find='fd'

      # Mark the prompt line for foot's jump-to-prompt feature
      function mark_prompt_start --on-event fish_prompt
          echo -en "\e]133;A\e\\"
      end
    '';

    functions = {
      fish_greeting = ''
        set_color brcyan
        echo '    _______         __                         '
        echo '   |_     _|.---.-.|  |--.--.--.--.--.---.-.   '
        echo '     |   |  |  _  ||    <|  |  |  |  |  _  |   '
        echo '     |___|  |___._||__|__|_____|___  |___._|   '
        echo '                               |_____|         '
        set_color normal
        command -v fastfetch >/dev/null 2>&1 && fastfetch --key-padding-left 5
      '';

      wifi-connect = ''
        set iface $argv[1]
        set ssid $argv[2]
        set pass $argv[3]
        sudo nmcli dev wifi connect "$ssid" password "$pass"
      '';
    };
  };
}
