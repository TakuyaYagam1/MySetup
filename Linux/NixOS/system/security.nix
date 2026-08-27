_:

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

      # Rebuild and garbage-collection commands use the ordinary sudo
      # credential cache. An unrestricted passwordless nixos-rebuild is equivalent
      # to passwordless root because a flake can define arbitrary activation
      # scripts.
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
