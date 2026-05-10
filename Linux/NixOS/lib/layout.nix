{ nixosRoot }:

let
  requirePath =
    label: path:
    if builtins.pathExists path then
      path
    else
      throw "MySetup layout error: missing ${label} at ${toString path}";

  firstExisting =
    label: candidates:
    let
      matches = builtins.filter builtins.pathExists candidates;
    in
    if matches != [ ] then
      builtins.head matches
    else
      throw "MySetup layout error: missing ${label}; tried ${builtins.concatStringsSep ", " (map toString candidates)}";

in
{
  nixos = requirePath "NixOS root" nixosRoot;
  dots = firstExisting "dots root" [
    (nixosRoot + "/dots")
    (nixosRoot + "/../dots")
  ];
  installer = firstExisting "installer root" [
    (nixosRoot + "/installer")
    (nixosRoot + "/../installer")
  ];
}
