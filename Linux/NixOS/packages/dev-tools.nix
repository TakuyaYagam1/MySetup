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
    
    # Python
    python3
    pipx
    
    # Go ecosystem
    go
    grpcurl
    mockgen
    go-swagger
    go-swag
    go-mockery
    go-migrate
    delve
    protobuf
    protoc-gen-go
    protoc-gen-go-grpc
    buf
    golangci-lint
    sqlc
    swagger-codegen
    
    # Node.js ecosystem
    nodejs
    yarn
    typescript
    
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
    
    # Other
    opencv
  ];
}
