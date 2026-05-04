{ config, lib, pkgs, ... }:

let
  preset = config.var.packagePreset or "personal";
  enabled = lib.elem preset [ "developer" "personal" ];
in
{
  config = lib.mkIf enabled {
    environment.systemPackages = with pkgs; [
      # Build tools
      cmake
      ninja
      gnumake
      pkg-config
      tree-sitter
      just

      # Compilers & debuggers
      gcc
      libgcc
      lldb

      # Python Ecosystem
      python3
      pipx
      uv
      ruff
      mypy

      # Go ecosystem
      go
      gopls
      gotools
      go-tools
      go-outline
      gopkgs
      delve
      air
      gomodifytags
      impl
      gotests
      golangci-lint
      gofumpt
      govulncheck
      grpcurl
      mockgen
      go-swagger
      go-swag
      oapi-codegen
      go-mockery
      go-migrate
      protobuf
      protoc-gen-go
      protoc-gen-go-grpc
      buf
      sqlc
      swagger-codegen

      # Node.js ecosystem
      nodejs
      yarn
      typescript
      bun
      playwright

      # Java
      jdk

      # Infrastructure as Code
      terraform
      tflint
      terraform-docs
      ansible

      # Container & K8s tools
      lazydocker
      k9s
      kubectl
      kind
      kubernetes-helm
      kustomize
      tilt
      cilium-cli
      cosign
      cri-tools
      eksctl
      spire

      # API tools
      httpie
      yq-go

      # Version control
      lazygit
      mercurial
      subversion

      # Modern Terminal Utilities
      tldr

      # Other
      opencv
    ];

    environment.variables = {
      PLAYWRIGHT_BROWSERS_PATH = "${pkgs.playwright-driver.browsers}";
      PLAYWRIGHT_SKIP_VALIDATE_HOST_REQUIREMENTS = "true";
    };
  };
}
