{ mysetup, pkgs, ... }:

{
  programs = {
    git = {
      enable = true;
      package = pkgs.gitFull;

      settings = {
        user.name = mysetup.git.username;
        user.email = mysetup.git.email;
        init.defaultBranch = "main";
        pull.rebase = true;
        push.autoSetupRemote = true;
        push.followTags = true;
        fetch.prune = true;
        rebase.autoStash = true;
        rebase.autoSquash = true;
        merge.conflictStyle = "zdiff3";
        diff.algorithm = "histogram";
        diff.colorMoved = "default";
        column.ui = "auto";
        branch.sort = "-committerdate";
        help.autocorrect = "prompt";
        core.editor = "nvim";
        core.autocrlf = "input";
        color.ui = "auto";
        commit.verbose = true;
        tag.sort = "version:refname";
        alias = {
          st = "status -sb";
          co = "checkout";
          br = "branch";
          ci = "commit";
          cim = "commit -m";
          cia = "commit --amend --no-edit";
          lg = "log --oneline --graph --decorate --all";
          last = "log -1 HEAD --stat";
          unstage = "reset HEAD --";
          undo = "reset --soft HEAD~1";
          wip = "commit -am 'wip'";
        };
      };
      ignores = [
        ".DS_Store"
        "*.swp"
        "*.swo"
        ".direnv/"
        ".envrc.local"
        "result"
        "result-*"
        ".idea/"
        ".vscode/"
        "__pycache__/"
        "*.pyc"
        ".venv/"
        "node_modules/"
      ];
      lfs.enable = true;
    };
    delta = {
      enable = true;
      enableGitIntegration = true;
      options = {
        navigate = true;
        line-numbers = true;
        side-by-side = false;
        syntax-theme = "base16";
      };
    };
    lazygit = {
      enable = true;
      settings = {
        disableStartupPopups = true;
        notARepository = "skip";
        promptToReturnFromSubprocess = false;
        update.method = "never";
        gui = {
          showIcons = true;
          showBottomLine = false;
          showCommandLog = false;
          showListFooter = false;
          showRandomTip = false;
          nerdFontsVersion = "3";
          theme = {
            activeBorderColor = [
              "blue"
              "bold"
            ];
            inactiveBorderColor = [ "default" ];
            selectedLineBgColor = [ "default" ];
          };
        };
        git = {
          paging = {
            colorArg = "always";
            pager = "delta --dark --paging=never";
          };
          parseEmoji = true;
        };
      };
    };
  };
}
