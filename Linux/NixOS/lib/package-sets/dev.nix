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
    clang
    clang-tools
    lld
    mold
    lldb
    gdb
    valgrind
    cppcheck
    bear
    meson
    conan
    vcpkg
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
    ogen
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
    corepack
    yarn
    typescript
    typescript-language-server
    vscode-langservers-extracted
    tailwindcss-language-server
    yaml-language-server
    bun
    deno
    eslint_d
    prettierd
    biome
    playwright
  ];
  rustTools = with pkgs; [
    rustc
    cargo
    rustfmt
    clippy
    rust-analyzer
    cargo-nextest
    cargo-edit
    cargo-watch
    cargo-audit
    cargo-deny
    cargo-expand
    sccache
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
  rubyTools =
    with pkgs;
    [
      ruby
      bundler
      rubocop
      ruby-lsp
      solargraph
      rubyfmt
      brakeman
      bundler-audit
    ]
    ++ (with pkgs.rubyPackages; [
      pry
      rspec
      standard
    ]);
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
    ++ rustTools
    ++ jvmTools
    ++ rubyTools
    ++ iacTools
    ++ containerTools
    ++ generalTools;
}
