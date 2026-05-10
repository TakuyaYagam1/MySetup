SHELL := /usr/bin/env bash
.DEFAULT_GOAL := help

# Pinned tool versions.
GOLANGCI_LINT_VERSION ?= v2.11.4

GOLANGCI_LINT := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
NIXOS_DIR := $(abspath ../NixOS)
REPO_ROOT := $(abspath ../..)
NIX_CACHE_SUBSTITUTERS := https://nix-community.cachix.org https://hyprland.cachix.org https://quickshell.cachix.org https://numtide.cachix.org
NIX_CACHE_KEYS := nix-community.cachix.org-1:mB9FSh9qf2dCimDSUo8Zy7bkq5CX+/rkCWyvRCYg3Fs= hyprland.cachix.org-1:a7pgxzMz7+chwVL3/pzj6jIBMioiJM7ypFP8PwtkuGc= quickshell.cachix.org-1:tjWMR3PQd01gN6YtjSRUdHHHUgrSLFIgwqrCQjFXVOU= numtide.cachix.org-1:2ps1kLBUWjxIneOy1Ik6cQjb41X0iXVXeHigGmycPPE=
NIX_CACHE_ARGS := --option extra-substituters "$(NIX_CACHE_SUBSTITUTERS)" --option extra-trusted-public-keys "$(NIX_CACHE_KEYS)"

##@ Build & run

.PHONY: build
build: ## Build mysetup into ./bin/mysetup
	@mkdir -p bin
	go build -trimpath -o bin/mysetup ./cmd/mysetup

.PHONY: install
install: build ## Install ./bin/mysetup into $$HOME/.local/bin
	@install -d "$$HOME/.local/bin"
	install -m 0755 bin/mysetup "$$HOME/.local/bin/mysetup"
	@echo "installed: $$HOME/.local/bin/mysetup"

.PHONY: run
run: ## Run the installer
	go run ./cmd/mysetup

.PHONY: dev-deps
dev-deps: ## Download module dependencies into the local cache
	go mod download

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

.PHONY: shell-check
shell-check: ## Check Hypr shell scripts and JSON/Python patch sources
	cd $(REPO_ROOT) && find Linux/dots/hypr/scripts -type f -name '*.sh' -print0 | xargs -0 -r -n1 bash -n
	cd $(REPO_ROOT) && find Linux/dots/hypr/scripts -type f -name '*.sh' -print0 | xargs -0 -r shellcheck -x
	cd $(REPO_ROOT) && fish -n Linux/dots/hypr/scripts/wsaction.fish
	cd $(REPO_ROOT) && find Linux/NixOS/home/end4 Linux/NixOS/home/noctalia/config -type f -name '*.json' -print0 | xargs -0 -r -n1 jq empty
	cd $(REPO_ROOT) && python3 -c 'import ast, pathlib; ast.parse(pathlib.Path("Linux/NixOS/home/end4/patches/quickshell.py").read_text())'

.PHONY: hypr-bind-check
hypr-bind-check: ## Check Hypr keybind contracts and duplicate chords
	go test -count=1 ./internal/dots -run 'Test(HyprKeybindFilesDoNotContainUnexpectedDuplicateChords|ShellKeybindProfilesUseExpectedLaunchers)'

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
	nix build $(NIX_CACHE_ARGS) --no-link --no-write-lock-file "path:$(REPO_ROOT)?dir=Linux/NixOS#mysetup"

.PHONY: nix-package-eval
nix-package-eval: ## Eval mysetup through the repository-root flake path
	nix eval $(NIX_CACHE_ARGS) --no-write-lock-file --impure --raw --expr 'let flake = builtins.getFlake "path:$(REPO_ROOT)?dir=Linux/NixOS"; in flake.packages.x86_64-linux.mysetup.pname' >/dev/null

NIX_HM_ENV := NIXOS_DIR='$(NIXOS_DIR)' \
	NIX_CACHE_SUBSTITUTERS='$(NIX_CACHE_SUBSTITUTERS)' \
	NIX_CACHE_KEYS='$(NIX_CACHE_KEYS)'

.PHONY: nix-hm-eval
nix-hm-eval: ## Eval runtime shell module and all-on Home Manager imports, including end4
	@$(NIX_HM_ENV) bash -c 'source scripts/nix-eval-helpers.sh && \
		eval_hm_simple "hm.config.home.activation.seedMySetupHyprConfig.data" && \
		eval_hm_simple "hm.config.xdg.configFile.\"hypr/shell-profile.conf\".text" && \
		eval_hm_full "" "builtins.toString (hm.config.xdg.configFile.\"hypr/end4\".source)"'

.PHONY: nix-profile-eval
nix-profile-eval: ## Eval Home Manager imports for every package preset
	@$(NIX_HM_ENV) bash -c 'source scripts/nix-eval-helpers.sh && \
		for preset in minimal desktop developer personal; do \
			echo "nix profile eval: $$preset"; \
			eval_hm_full "$$preset" "hm.config.home.username"; \
		done'

.PHONY: nix-installed-mirror-build
nix-installed-mirror-build: ## Build mysetup from an /etc/nixos-style temporary mirror
	@tmp="$$(mktemp -d)"; \
	trap 'rm -rf "$$tmp"' EXIT; \
	rsync -a ../NixOS/ "$$tmp"/; \
	rsync -a ../dots/ "$$tmp"/dots/; \
	rsync -a --exclude=/bin/ --exclude=/tmp/ --exclude=/coverage.out --exclude=/coverage.html ./ "$$tmp"/installer/; \
	nix build $(NIX_CACHE_ARGS) --no-link --no-write-lock-file "path:$$tmp#mysetup"

.PHONY: check
check: ## Run the full local verification suite and remove build artefacts
	@status=0; \
	$(MAKE) fmt-check || status=$$?; \
	if [ $$status -eq 0 ]; then $(MAKE) lint || status=$$?; fi; \
	if [ $$status -eq 0 ]; then $(MAKE) vet || status=$$?; fi; \
	if [ $$status -eq 0 ]; then $(MAKE) shell-check || status=$$?; fi; \
	if [ $$status -eq 0 ]; then $(MAKE) hypr-bind-check || status=$$?; fi; \
	if [ $$status -eq 0 ]; then $(MAKE) test || status=$$?; fi; \
	if [ $$status -eq 0 ]; then $(MAKE) build || status=$$?; fi; \
	if [ $$status -eq 0 ]; then $(MAKE) nix-package-eval || status=$$?; fi; \
	if [ $$status -eq 0 ]; then $(MAKE) nix-profile-eval || status=$$?; fi; \
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
