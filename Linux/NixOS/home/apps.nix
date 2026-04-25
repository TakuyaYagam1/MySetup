{ ... }:

# Custom XDG desktop entries that override the default ones from upstream
# packages (e.g. forcing X11 backend for apps that misbehave under Wayland).
# Application package lists live in ./programs/user-apps/*.nix.

{
  xdg.desktopEntries.virt-manager = {
    name = "Virtual Machine Manager";
    exec = "env GDK_BACKEND=x11 virt-manager";
    icon = "virt-manager";
    terminal = false;
    categories = [ "System" ];
  };

  xdg.desktopEntries.thunar = {
    name = "Thunar";
    exec = "env GDK_BACKEND=x11 thunar %F";
    icon = "Thunar";
    terminal = false;
    categories = [ "System" "FileManager" ];
    mimeType = [ "inode/directory" ];
  };
}
