{
  config,
  lib,
  pkgs,
  ...
}:

let
  ghidraMcpVersion = "1.4";
  ghidraMcpVersionTag = builtins.replaceStrings [ "." ] [ "-" ] ghidraMcpVersion;

  ghidraMcpRelease = pkgs.fetchurl {
    url = "https://github.com/LaurieWired/GhidraMCP/releases/download/${ghidraMcpVersion}/GhidraMCP-release-${ghidraMcpVersionTag}.zip";
    hash = "sha256-uBylJA/d5X6k6JkXDc2f3ubtKSRigMdCY6UJv8/H5zQ=";
  };

  ghidraMcpAssets = pkgs.runCommand "ghidra-mcp-assets-${ghidraMcpVersion}"
    {
      nativeBuildInputs = [ pkgs.unzip ];
    }
    ''
      mkdir -p "$out"
      unzip -q ${ghidraMcpRelease} -d "$out"
      rm -f "$out"/GhidraMCP-release-${ghidraMcpVersionTag}/.DS_Store
      mv "$out"/GhidraMCP-release-${ghidraMcpVersionTag}/* "$out"/
      rmdir "$out"/GhidraMCP-release-${ghidraMcpVersionTag}
    '';

  ghidraMcpPython = pkgs.python313.withPackages (
    ps: with ps; [
      requests
      mcp
    ]
  );

  ghidraWithMcpPackage = pkgs.runCommand "ghidra-with-mcp-${pkgs.ghidra-bin.version}"
    {
      inherit (pkgs.ghidra-bin) meta;
      nativeBuildInputs = [ pkgs.unzip ];
    }
    ''
      cp -rL ${pkgs.ghidra-bin} "$out"
      chmod -R u+w "$out"

      mkdir -p "$out/lib/ghidra/Extensions"
      unzip -q ${ghidraMcpAssets}/GhidraMCP-${ghidraMcpVersionTag}.zip -d "$out/lib/ghidra/Extensions"
    '';

  ghidraWithMcp = pkgs.writeShellScriptBin "ghidra-with-mcp" ''
    set -euo pipefail
    exec ${ghidraWithMcpPackage}/bin/ghidra "$@"
  '';

  ghidraMcpServer = pkgs.writeShellScriptBin "ghidra-mcp-server" ''
    set -euo pipefail
    exec ${ghidraMcpPython}/bin/python ${ghidraMcpAssets}/bridge_mcp_ghidra.py \
      --ghidra-server "''${GHIDRA_MCP_HTTP_SERVER:-http://127.0.0.1:8080/}" \
      --transport "''${GHIDRA_MCP_TRANSPORT:-stdio}" \
      --mcp-host "''${GHIDRA_MCP_HOST:-127.0.0.1}" \
      --mcp-port "''${GHIDRA_MCP_PORT:-8081}" \
      "$@"
  '';

  ghidraMcpExtensionPath = pkgs.writeShellScriptBin "ghidra-mcp-extension-path" ''
    set -euo pipefail
    printf '%s\n' ${ghidraMcpAssets}/GhidraMCP-${ghidraMcpVersionTag}.zip
  '';
in
{
  environment.systemPackages = lib.optionals config.mysetup.features.ctfTools [
      ghidraWithMcp
      ghidraMcpServer
      ghidraMcpExtensionPath
    ];
}
