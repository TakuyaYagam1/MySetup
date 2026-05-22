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
    liquibase
    protobuf
    protoc-gen-go
    protoc-gen-go-grpc
    buf
    sqlc
    swagger-codegen
  ];
  jsTools = with pkgs; [
    nodejs
    yarn
    typescript
    bun
    playwright
  ];
  jvmTools = with pkgs; [ jdk ];
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
