{ pkgs }:
let
  buildTools = with pkgs; [
    cmake
    ninja
    gnumake
    pkg-config
    tree-sitter
    just
    templ
    gcc
    libgcc
    lldb
  ];
  pythonTools = with pkgs; [
    python3
    pipx
    uv
    ruff
    mypy
  ];
  goTools = with pkgs; [
    go
    gopls
    delve
    air
    golangci-lint
    gofumpt
    govulncheck
    grpcurl
    mockgen
    go-swag
    oapi-codegen
    go-mockery
    go-migrate
    protobuf
    protoc-gen-go
    protoc-gen-go-grpc
    buf
    sqlc
  ];
  jsTools = with pkgs; [
    nodejs
    yarn
    typescript
    bun
    playwright
  ];
  jvmTools = with pkgs; [
    jdk
    maven
    gradle
    jdt-language-server
    google-java-format
    checkstyle
    pmd
    liquibase
  ];
  iacTools = with pkgs; [
    terraform
    tflint
    terraform-docs
    ansible
  ];
  containerTools = with pkgs; [
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
  ];
  generalTools = with pkgs; [
    httpie
    yq-go
    lazygit
    mercurial
    subversion
    tldr
    opencv
    typst
    ghgrab
  ];
in
{
  devTools =
    buildTools
    ++ pythonTools
    ++ goTools
    ++ jsTools
    ++ jvmTools
    ++ iacTools
    ++ containerTools
    ++ generalTools;
}
