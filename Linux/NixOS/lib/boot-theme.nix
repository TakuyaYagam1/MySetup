let
  extensions = [
    "png"
    "jpg"
    "jpeg"
  ];
  stringToPath = s: /. + s;
in
{
  # Resolves the logo image for a boot-time theme (grub/sddm/plymouth),
  # preferring a per-service override, then a shared logo, then `default`.
  # Priority: ~/.config/mysetup/boot-theme/<service>.{png,jpg,jpeg}
  #        -> ~/.config/mysetup/boot-theme/logo.{png,jpg,jpeg}
  #        -> default
  resolveLogo =
    {
      homeDirectory,
      service,
      default,
    }:
    let
      dir = "${homeDirectory}/.config/mysetup/boot-theme";
      candidates =
        map (ext: "${dir}/${service}.${ext}") extensions
        ++ map (ext: "${dir}/logo.${ext}") extensions;
      existing = builtins.filter builtins.pathExists candidates;
    in
    if existing != [ ] then stringToPath (builtins.head existing) else default;
}
