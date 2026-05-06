SHELL := /usr/bin/env bash
.DEFAULT_GOAL := help

# Pinned tool versions.
GOLANGCI_LINT_VERSION ?= v2.11.4

GOLANGCI_LINT := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
NIXOS_DIR := $(abspath ../NixOS)

##@ Build & run

.PHONY: build
build: ## Build mysetup into ./bin/mysetup
	@mkdir -p bin
	go build -trimpath -o bin/mysetup ./cmd/mysetup

.PHONY: run
run: ## Run the installer
	go run ./cmd/mysetup

.PHONY: tidy
tidy: ## go mod tidy
	go mod tidy

.PHONY: clean
clean: ## Remove build and coverage artefacts
	rm -rf bin/ tmp/ coverage.out coverage.html

##@ Lint, format, test

.PHONY: lint
lint: ## golangci-lint run
	$(GOLANGCI_LINT) run ./...

.PHONY: lint-fix
lint-fix: ## golangci-lint run --fix
	$(GOLANGCI_LINT) run --fix ./...

.PHONY: fmt
fmt: ## golangci-lint fmt
	$(GOLANGCI_LINT) fmt

.PHONY: fmt-check
fmt-check: ## Check Go formatting without rewriting files
	$(GOLANGCI_LINT) fmt --diff

.PHONY: vet
vet: ## go vet ./...
	go vet ./...

.PHONY: test
test: ## Unit tests with race detector
	go test -race -count=1 ./...

.PHONY: cover
cover: ## Unit-test coverage report (HTML)
	go test -race -count=1 -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

##@ Nix

.PHONY: nix-build
nix-build: ## Build the mysetup flake package
	nix build --no-link --no-write-lock-file ../NixOS#mysetup

.PHONY: nix-hm-eval
nix-hm-eval: ## Eval Home Manager shell keybind sources for both shell profiles
	nix eval --no-write-lock-file --impure --raw --expr 'let flake = builtins.getFlake "path:$(NIXOS_DIR)"; system = "x86_64-linux"; pkgs = import flake.inputs.nixpkgs { inherit system; config.allowUnfree = true; }; mod = profile: import $(NIXOS_DIR)/home/shells/default.nix { config = { xdg.configHome = "/tmp/mysetup"; }; lib = pkgs.lib; inherit pkgs; var = { shellProfile = profile; }; }; in builtins.toString ((mod "caelestia").xdg.configFile."hypr/shell-keybinds.conf".source)' >/dev/null
	nix eval --no-write-lock-file --impure --raw --expr 'let flake = builtins.getFlake "path:$(NIXOS_DIR)"; system = "x86_64-linux"; pkgs = import flake.inputs.nixpkgs { inherit system; config.allowUnfree = true; }; mod = profile: import $(NIXOS_DIR)/home/shells/default.nix { config = { xdg.configHome = "/tmp/mysetup"; }; lib = pkgs.lib; inherit pkgs; var = { shellProfile = profile; }; }; in builtins.toString ((mod "noctalia").xdg.configFile."hypr/shell-keybinds.conf".source)' >/dev/null

.PHONY: nix-installed-mirror-build
nix-installed-mirror-build: ## Build mysetup from an /etc/nixos-style temporary mirror
	@tmp="$$(mktemp -d)"; \
	trap 'rm -rf "$$tmp"' EXIT; \
	rsync -a ../NixOS/ "$$tmp"/; \
	rsync -a ../dots/ "$$tmp"/dots/; \
	rsync -a --exclude=/bin/ --exclude=/tmp/ --exclude=/coverage.out --exclude=/coverage.html ./ "$$tmp"/installer/; \
	nix build --no-link --no-write-lock-file "path:$$tmp#mysetup"

.PHONY: check
check: ## Run the full local verification suite and remove build artefacts
	@status=0; \
	$(MAKE) fmt-check || status=$$?; \
	if [ $$status -eq 0 ]; then $(MAKE) lint || status=$$?; fi; \
	if [ $$status -eq 0 ]; then $(MAKE) vet || status=$$?; fi; \
	if [ $$status -eq 0 ]; then $(MAKE) test || status=$$?; fi; \
	if [ $$status -eq 0 ]; then $(MAKE) build || status=$$?; fi; \
	if [ $$status -eq 0 ]; then $(MAKE) nix-build || status=$$?; fi; \
	if [ $$status -eq 0 ]; then $(MAKE) nix-hm-eval || status=$$?; fi; \
	if [ $$status -eq 0 ]; then $(MAKE) nix-installed-mirror-build || status=$$?; fi; \
	$(MAKE) clean || status=$$?; \
	exit $$status

##@ Help

.PHONY: help
help: ## Show this help
	@awk 'BEGIN{FS=":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} \
		/^[a-zA-Z0-9_-]+:.*?##/ {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2} \
		/^##@/ {printf "\n\033[1m%s\033[0m\n", substr($$0, 5)}' $(MAKEFILE_LIST)
