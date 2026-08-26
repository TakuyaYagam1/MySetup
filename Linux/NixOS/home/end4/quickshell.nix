{ end4Lib, pkgs, ... }:

let
  wahrweltPkgs = pkgs.wahrwelt or (pkgs.mysetup or { });
  qsPackage = wahrweltPkgs.quickshell or pkgs.quickshell;
in
{
  home.packages = [
    (pkgs.writeShellScriptBin "qs-end4" ''
      runtime_env="''${XDG_CONFIG_HOME:-$HOME/.config}/hypr/scripts/end4-runtime-env.sh"

      declare -a wahrwelt_preserved_env_names=()
      declare -a wahrwelt_preserved_env_values=()
      preserve_end4_env() {
        local name="$1"
        if [[ -v "$name" ]]; then
          wahrwelt_preserved_env_names+=("$name")
          wahrwelt_preserved_env_values+=("''${!name}")
        fi
      }

      qs_config_was_set=0
      if [[ -v qsConfig ]]; then
        qs_config_was_set=1
      fi
      for name in WAHRWELT_END4_PROFILE WAHRWELT_QS_CONFIG qsConfig; do
        preserve_end4_env "$name"
      done
      while IFS= read -r name; do
        preserve_end4_env "$name"
      done < <(compgen -e ILLOGICAL_IMPULSE_)

      if [ -r "$runtime_env" ]; then
        . "$runtime_env"
      else
        ${end4Lib.runtimeEnv.quickshellExports}
      fi

      for index in "''${!wahrwelt_preserved_env_names[@]}"; do
        name="''${wahrwelt_preserved_env_names[$index]}"
        printf -v "$name" '%s' "''${wahrwelt_preserved_env_values[$index]}"
        export "$name"
      done

      if [ "$qs_config_was_set" -eq 0 ] && [ -n "''${WAHRWELT_QS_CONFIG:-}" ]; then
        export qsConfig="$WAHRWELT_QS_CONFIG"
      fi
      exec ${qsPackage}/bin/qs "$@"
    '')
  ];
}
