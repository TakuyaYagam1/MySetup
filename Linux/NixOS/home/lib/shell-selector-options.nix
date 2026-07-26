{ lib, pkgs }:

{
  # Build a QML fragment listing shell profile entries; suitable for inserting
  # between WAHRWELT_SHELL_OPTIONS_BEGIN/END markers in shell.qml.
  buildOptionsFile =
    profiles:
    pkgs.writeText "mysetup-shell-selector-options.qml" (
      lib.concatMapStringsSep ",\n" (
        profile:
        "    {\n"
        + "      id: ${builtins.toJSON profile.id},\n"
        + "      title: ${builtins.toJSON profile.title},\n"
        + "      accent: ${builtins.toJSON profile.accent},\n"
        + "      surface: ${builtins.toJSON profile.surface},\n"
        + "      logo: Qt.resolvedUrl(${builtins.toJSON profile.logo})\n"
        + "    }"
      ) profiles
    );

  # Stamp the options block into shell.qml of the source tree, returning a
  # store path with the patched copy.
  buildSelectorSource =
    {
      selectorRoot,
      optionsFile,
    }:
    pkgs.runCommand "mysetup-shell-selector" { nativeBuildInputs = [ pkgs.python3 ]; } ''
      cp -r ${selectorRoot} "$out"
      chmod -R u+w "$out"
      python3 - "$out/shell.qml" ${optionsFile} <<'PY'
      import sys
      from pathlib import Path

      target = Path(sys.argv[1])
      options = Path(sys.argv[2]).read_text().rstrip()
      begin = "    // WAHRWELT_SHELL_OPTIONS_BEGIN"
      end = "    // WAHRWELT_SHELL_OPTIONS_END"
      text = target.read_text()
      prefix, rest = text.split(begin, 1)
      _, suffix = rest.split(end, 1)
      target.write_text(prefix + begin + "\n" + options + "\n" + end + suffix)
      PY
    '';
}
