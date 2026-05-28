{
  config,
  lib,
  pkgs,
  ...
}:

let
  burpMcpVersion = "1.3.0";

  burpMcpBapp = pkgs.fetchurl {
    url = "https://portswigger.net/bappstore/bapps/download/9952290f04ed4f628e624d0aa9dccebc/10";
    hash = "sha256-TWdbEHlv9R1EDFpA84VaPTihZmqRoLKPuIU6JqvVssU=";
  };

  burpMcpAssets = pkgs.runCommand "burp-mcp-assets-${burpMcpVersion}"
    {
      nativeBuildInputs = [ pkgs.unzip ];
    }
    ''
      mkdir -p "$out"
      install -Dm644 ${burpMcpBapp} "$out/mcp-server-${burpMcpVersion}.bapp"
      unzip -q ${burpMcpBapp} -d "$out"
    '';

  installBurpMcp = pkgs.writeShellScriptBin "install-burp-mcp" ''
    set -euo pipefail

    target_dir="''${XDG_DATA_HOME:-$HOME/.local/share}/mysetup/burp-mcp"
    mkdir -p "$target_dir"

    ln -sfn ${burpMcpAssets}/mcp-server-${burpMcpVersion}.bapp "$target_dir/mcp-server-${burpMcpVersion}.bapp"
    ln -sfn ${burpMcpAssets}/burp-mcp-all.jar "$target_dir/burp-mcp-all.jar"

    cat <<EOF
Burp MCP assets staged in:
  $target_dir

One-time Burp step still required:
  1. Launch Burp Suite.
  2. Open Extensions -> Installed -> Add.
  3. Select $target_dir/mcp-server-${burpMcpVersion}.bapp
  4. In the MCP tab, leave the default listener at http://127.0.0.1:9876

After that, Codex/Claude can use the existing SSE URL configuration.
EOF
  '';

  burpMcpBappPath = pkgs.writeShellScriptBin "burp-mcp-bapp-path" ''
    set -euo pipefail
    printf '%s\n' ${burpMcpAssets}/mcp-server-${burpMcpVersion}.bapp
  '';

  burpMcpJarPath = pkgs.writeShellScriptBin "burp-mcp-jar-path" ''
    set -euo pipefail
    printf '%s\n' ${burpMcpAssets}/burp-mcp-all.jar
  '';

  burpsuiteWithMcp = pkgs.writeShellScriptBin "burpsuite-with-mcp" ''
    set -euo pipefail

    target_dir="''${XDG_DATA_HOME:-$HOME/.local/share}/mysetup/burp-mcp"
    notice_file="$target_dir/.wrapper-notice-shown"

    mkdir -p "$target_dir"
    ln -sfn ${burpMcpAssets}/mcp-server-${burpMcpVersion}.bapp "$target_dir/mcp-server-${burpMcpVersion}.bapp"
    ln -sfn ${burpMcpAssets}/burp-mcp-all.jar "$target_dir/burp-mcp-all.jar"

    if [ ! -f "$notice_file" ]; then
      cat >&2 <<EOF
Prepared Burp MCP assets in:
  $target_dir

Burp still needs a one-time import:
  Extensions -> Installed -> Add -> $target_dir/mcp-server-${burpMcpVersion}.bapp
EOF
      : > "$notice_file"
    fi

    exec ${pkgs.burpsuite}/bin/burpsuite "$@"
  '';
in
{
  environment.systemPackages = lib.optionals config.mysetup.features.ctfTools [
      installBurpMcp
      burpMcpBappPath
      burpMcpJarPath
      burpsuiteWithMcp
    ];
}
