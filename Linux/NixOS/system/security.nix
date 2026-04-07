{ pkgs, ... }:

{
  security.polkit = {
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

  services.openssh = {
    enable = true;
    settings = {
      PermitRootLogin = "no";
      PasswordAuthentication = false;
      KbdInteractiveAuthentication = false;
    };
  };
}
