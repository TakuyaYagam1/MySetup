_:

{
  imports = [
    ../services/databases.nix
    ../services/virtualization.nix
    ../programs/development.nix
    ../packages/dev-tools.nix
    # ../packages/ida-pro.nix
    # ../packages/ida-mcp.nix
    # ../packages/ida-plugins.nix
    ../users/android-sdk.nix
  ];
}
