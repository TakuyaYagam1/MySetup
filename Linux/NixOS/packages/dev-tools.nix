{ pkgs, ... }:

{
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
    gdb
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
    helm
    kustomize
    tilt

    # API tools
    httpie
    yq-go

    # Version control
    lazygit

    # Modern Terminal Utilities
    tldr

    # Other
    opencv
  ];
}
