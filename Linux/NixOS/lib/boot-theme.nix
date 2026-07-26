let
  extensions = [
    "png"
    "jpg"
    "jpeg"
  ];
  stringToPath = s: /. + s;

  findFile =
    dir: name:
    let
      candidates = map (ext: "${dir}/${name}.${ext}") extensions;
      existing = builtins.filter builtins.pathExists candidates;
    in
    if existing != [ ] then stringToPath (builtins.head existing) else null;
in
{
  # Resolves the logo image for a boot-time theme (grub/sddm/plymouth).
  # Priority: ~/.config/wahrwelt/boot-theme/<service>.{png,jpg,jpeg}
  #        -> ~/.config/wahrwelt/boot-theme/logo.{png,jpg,jpeg}
  #
  # If the boot-theme directory doesn't exist yet (nothing has ever been
  # seeded - the very first apply on a fresh install), `default` is used
  # silently. Once the directory exists, every service must resolve to a
  # real file (its own override or the shared logo) - a directory that
  # exists but covers some services and not others is treated as a
  # mistake and fails the build instead of silently guessing.
  resolveLogo =
    {
      homeDirectory,
      service,
      default,
    }:
    let
      canonicalDir = "${homeDirectory}/.config/wahrwelt/boot-theme";
      legacyDir = "${homeDirectory}/.config/mysetup/boot-theme";
      dir = if builtins.pathExists canonicalDir then canonicalDir else legacyDir;
    in
    if !(builtins.pathExists canonicalDir) && !(builtins.pathExists legacyDir) then
      default
    else
      let
        perService = findFile dir service;
        shared = findFile dir "logo";
      in
      if perService != null then
        perService
      else if shared != null then
        shared
      else
        throw ''
          Wahrwelt boot-theme: ${dir} exists but has neither
          ${service}.{png,jpg,jpeg} nor logo.{png,jpg,jpeg}. Add one of
          them, or delete the whole boot-theme directory to fall back to
          the built-in default.
        '';
}
