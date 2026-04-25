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
      nixos-install = "sudo nixos-rebuild switch --flake ${var.configDirectory}#${var.hostname}";
      nixos-update = "pushd ${var.configDirectory}; and sudo nix flake update; and sudo nixos-rebuild switch --flake .#${var.hostname}; and popd";
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
