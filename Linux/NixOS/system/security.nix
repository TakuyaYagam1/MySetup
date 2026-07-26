{ config, ... }:

{
  security = {
    polkit = {
      enable = true;
      extraConfig = ''
        polkit.addRule(function(action, subject) {
          if (subject.isInGroup("networkmanager") &&
              (action.id.indexOf("org.freedesktop.NetworkManager.") == 0)) {
            return polkit.Result.YES;
          }
        });
      '';
    };

    sudo = {
      # Ask for sudo password once per tty session, expire after 60 minutes of inactivity.
      # Infinity (-1) was convenient but turned wheel access into permanent root for any
      # process running under the user's session.
      extraConfig = ''
        Defaults timestamp_type=tty,timestamp_timeout=60
      '';

      # Let the primary user run nixos-rebuild without re-entering sudo password.
      extraRules = [
        {
          users = [ config.wahrwelt.user.username ];
          commands = [
            {
              command = "/run/current-system/sw/bin/nixos-rebuild";
              options = [ "NOPASSWD" ];
            }
            {
              command = "/run/current-system/sw/bin/nix-collect-garbage";
              options = [ "NOPASSWD" ];
            }
          ];
        }
      ];
    };
  };

  services.openssh = {
    enable = true;
    settings = {
      PermitRootLogin = "no";
      PasswordAuthentication = false;
      KbdInteractiveAuthentication = false;
    };
  };
}
