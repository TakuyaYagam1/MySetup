{ lib, osConfig, pkgs, ... }:

# Custom XDG desktop entries that override the default ones from upstream
# packages (e.g. forcing X11 backend for apps that misbehave under Wayland).
# Application package lists live in ./programs/user-apps/*.nix.

let
  hasSystemPackage = name:
    lib.any (pkg: lib.getName pkg == name) osConfig.environment.systemPackages;
in
{
  xdg.desktopEntries =
    {
      virt-manager = {
        name = "Virtual Machine Manager";
        exec = "env GDK_BACKEND=x11 virt-manager";
        icon = "virt-manager";
        terminal = false;
        categories = [ "System" ];
      };

      thunar = {
        name = "Thunar";
        exec = "env GDK_BACKEND=x11 thunar %F";
        icon = "Thunar";
        terminal = false;
        categories = [ "System" "FileManager" ];
        mimeType = [ "inode/directory" ];
      };

      "termius-app" = {
        name = "Termius";
        comment = "The SSH client that works on Desktop and Mobile";
        exec = "termius-app";
        icon = "${pkgs.termius}/share/icons/termius-app.png";
        terminal = false;
        categories = [ "Network" ];
      };
    }
    // lib.optionalAttrs (hasSystemPackage "zim") {
      "org.zim_wiki.Zim" = {
        name = "Zim Desktop Wiki";
        comment = "Edit text files \"wiki style\"";
        exec = "zim %f";
        icon = "zim";
        terminal = false;
        categories = [ "Utility" "TextEditor" ];
        mimeType = [ "application/x-zim-notebook" "text/x-zim-wiki" ];
      };
    };
}
