# CyNodeAI dev environment and scripting
# Run `just` for a list of recipes. Requires: https://github.com/casey/just
# shellcheck disable=SC2148,SC1007
# (justfile: not a shell script; recipe bodies are run by just with set shell)

# Strict bash for all recipes (explicit errors, safe pipelines).
set shell := ["bash", "-euo", "pipefail", "-c"]

# Go version to install when using install-go (match go.mod / toolchain)
go_version := "1.26.0"

# Directory containing this justfile (repo root when justfile is at root)
root_dir := justfile_directory()

# Workspace modules (from go.work).
go_work_path := root_dir / "go.work"
go_modules := shell("grep -E '^[[:space:]]*\\./' \"$1\" | sed 's|^[[:space:]]*\\./||;s/[[:space:]].*//' | tr '\\n' ' ' | sed 's/ $//'", go_work_path)

# Show list of available recipes (same as just --list).
default:
    @just --list

# Local CI: all lint, all tests (with 90% coverage), Go vuln check, BDD suites.
# test-go-cover runs coverage for all go_modules (orchestrator uses testcontainers for Postgres when POSTGRES_TEST_DSN unset).
# test-bdd runs Godog BDD for orchestrator and worker_node (orchestrator scenarios need POSTGRES_TEST_DSN for DB steps).
# Run full CI locally: lint (Go, Python, Markdown, containers), tests, vuln check, BDD.
ci: lint vulncheck-go test-go-cover test-bdd
    @:

# Local docs check: lint Markdown, validate doc links, validate feature files.
# With no arguments runs all checks on the repo. Pass paths to run checks only on those files (e.g. just docs-check docs/foo.md).
# Lint Markdown, validate doc links and feature files; pass paths to limit scope.
docs-check *PATHS:
    @just fix-cynode {{ PATHS }}
    @just lint-md {{ PATHS }}
    @just validate-doc-links
    @just validate-requirements
    @just validate-feature-files

# Fix all instances of "Cynode" to "CyNode" in Markdown files. With no arguments fixes all .md; pass paths to fix only those.
fix-cynode *PATHS:
    #!/usr/bin/env bash
    set -e
    root="{{ root_dir }}"
    if [ -z "{{ PATHS }}" ]; then
    if [ "$(uname)" = "Darwin" ]; then
    find "$root" -type f -iname '*.md' -exec sed -i '' 's/Cynode/CyNode/g' {} \;
    else
    find "$root" -type f -iname '*.md' -exec sed -i "s/Cynode/CyNode/g" {} \;
    fi
    else
    for f in {{ PATHS }}; do
    [ -z "$f" ] && continue
    case "$f" in *.[Mm][Dd]) ;; *) continue ;; esac
    [ -f "$root/$f" ] || [ -f "$f" ] || continue
    if [ "$(uname)" = "Darwin" ]; then
    sed -i '' 's/Cynode/CyNode/g' "$f" 2>/dev/null || sed -i '' 's/Cynode/CyNode/g' "$root/$f"
    else
    sed -i "s/Cynode/CyNode/g" "$f" 2>/dev/null || sed -i "s/Cynode/CyNode/g" "$root/$f"
    fi
    done
    fi

# Full dev setup: podman, Go, and Go tools (incl. deps for .golangci.yml and lint-go-ci)
setup: install-podman install-go install-go-tools install-markdownlint install-gherkin-lint venv
    @:

# Remove temporary files, .out files, compiled binaries (module bin/ dirs), coverage artifacts, and Go test cache.
clean:
    #!/usr/bin/env bash
    set -e
    root="{{ root_dir }}"
    echo "Cleaning $root ..."
    find "$root" -maxdepth 3 -type f -name '*.out' ! -path '*/.git/*' -delete 2>/dev/null || true
    find "$root" -maxdepth 4 -type f -name 'coverage.*' ! -path '*/.git/*' -delete 2>/dev/null || true
    for dir in bin orchestrator/bin worker_node/bin cynork/bin agents/bin; do
      rm -rf "$root/$dir"
    done
    for m in {{ go_modules }}; do
      (pushd "$root/$m" >/dev/null && go clean -testcache 2>/dev/null && popd >/dev/null 2>/dev/null) || true
    done
    echo "Done."

# Install Podman if not already installed (Linux: distro package; macOS: Homebrew)
install-podman:
    #!/usr/bin/env bash
    set -e
    # Detect OS via release files / uname and install via appropriate package manager
    if command -v podman >/dev/null 2>&1; then
        echo "podman already installed: $(podman --version)"
        exit 0
    fi
    if [ -f /etc/arch-release ]; then
        echo "Installing podman (Arch)..."
        sudo pacman -S --noconfirm podman
    elif [ -f /etc/debian_version ]; then
        echo "Installing podman (Debian/Ubuntu)..."
        sudo apt-get update && sudo apt-get install -y podman
    elif [ -f /etc/fedora-release ]; then
        echo "Installing podman (Fedora)..."
        sudo dnf install -y podman
    elif [ "$(uname)" = "Darwin" ]; then
        echo "Installing podman (macOS)..."
        brew install podman
    else
        echo "Unsupported OS for auto-install. Install podman manually: https://podman.io/getting-started/installation"
        exit 1
    fi
    echo "Podman installed: $(podman --version)"

# Enable and start Podman so the API/socket is available (for testcontainers-go and orchestrator DB tests).
# Linux: systemd user socket. macOS: podman machine (VM). Skips if already running.
# Start Podman (Linux: systemd user socket; macOS: podman machine). Skips if already running.
podman-setup: install-podman
    #!/usr/bin/env bash
    set -e
    case "$(uname)" in
      Darwin)
        # macOS: ensure podman machine (default VM) is running. One-time init: podman machine init
        if podman machine list --format '{{ "{{" }}.Running}}{{ "}}" }}' 2>/dev/null | grep -q 'true'; then
            echo "podman machine already running"
            exit 0
        fi
        podman machine start
        ;;
      Linux)
        if ! command -v systemctl >/dev/null 2>&1; then
            echo "podman-setup: systemctl not found; skipping"
            exit 0
        fi
        active=$(systemctl --user is-active podman.socket 2>/dev/null) || true
        enabled=$(systemctl --user is-enabled podman.socket 2>/dev/null) || true
        if [ "$active" = "active" ] && [ "$enabled" = "enabled" ]; then
            echo "podman.socket already enabled and active"
            exit 0
        fi
        systemctl --user enable --now podman.socket
        ;;
      *)
        echo "podman-setup: unsupported OS ($(uname)); skipping"
        exit 0
        ;;
    esac

# Install Go (prefer distro package; fall back to go.dev tarball for {{ go_version }})
install-go:
    #!/usr/bin/env bash
    set -e
    go_version="{{ go_version }}"
    # want_minor = first number after "1." (e.g. 25 from 1.25.7) for version check
    want_minor="${go_version#*.}"
    want_minor="${want_minor%%.*}"
    # go_ok: true if go is in PATH and its minor version >= want_minor
    go_ok() {
        command -v go >/dev/null 2>&1 && go version | sed -n 's/.*go1\.\([0-9]*\)\.*.*/\1/p' | xargs -I{} test "{}" -ge "$want_minor"
    }
    if go_ok; then
        echo "Go {{ go_version }} (or newer) already installed: $(go version)"
        exit 0
    fi
    did_apt_go=0
    if [ -f /etc/arch-release ]; then
        echo "Installing Go (Arch)..."
        sudo pacman -S --noconfirm go
    elif [ -f /etc/debian_version ]; then
        echo "Installing Go (Debian/Ubuntu)..."
        sudo apt-get update && sudo apt-get install -y golang-go
        did_apt_go=1
    elif [ -f /etc/fedora-release ]; then
        echo "Installing Go (Fedora)..."
        sudo dnf install -y golang
    elif [ "$(uname)" = "Darwin" ]; then
        echo "Installing Go (macOS)..."
        brew install go
    fi
    if go_ok; then
        echo "Go installed via package manager: $(go version)"
        exit 0
    fi
    # On Debian/Ubuntu, remove distro go so /usr/local/go takes precedence
    if [ "$did_apt_go" = 1 ]; then
        echo "Removing too-old golang-go package before tarball install"
        sudo apt-get remove -y golang-go || true
    fi
    # go.dev tarballs use three-part version (e.g. 1.25.0)
    tarball_version="$go_version"
    case "$tarball_version" in
        *.*.*) ;;
        *) tarball_version="${go_version}.0";;
    esac
    echo "Installing Go $tarball_version from go.dev/dl (distro package missing or too old)"
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    # Map kernel arch to Go naming (e.g. x86_64 -> amd64)
    arch=$(uname -m)
    case "$arch" in
        x86_64) arch=amd64 ;;
        aarch64|arm64) arch=arm64 ;;
    esac
    tarball="go${tarball_version}.${os}-${arch}.tar.gz"
    url="https://go.dev/dl/${tarball}"
    tmpdir=$(mktemp -d)
    trap 'rm -rf "$tmpdir"' EXIT
    curl -fsSL "$url" -o "$tmpdir/$tarball"
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf "$tmpdir/$tarball"
    go_path="/usr/local/go/bin"
    # Pick shell profile so we can append PATH for this session and future logins
    profile=""
    case "${SHELL:-}" in
        *zsh) profile="$HOME/.zshrc" ;;
        *bash) profile="$HOME/.bashrc" ;;
        *) profile="$HOME/.profile" ;;
    esac
    if [ -n "$profile" ] && [ -f "$profile" ]; then
        if ! grep -qF "$go_path" "$profile" 2>/dev/null; then
            echo '' >> "$profile"
            echo '# Go (just install-go)' >> "$profile"
            echo "export PATH=\"$go_path:\$HOME/go/bin:\$PATH\"" >> "$profile"
            echo "Added $go_path and \$HOME/go/bin to PATH in $profile"
            export PATH="$go_path:\$HOME/go/bin:\$PATH"
        fi
    else
        echo "Add to PATH: export PATH=\"$go_path:\$HOME/go/bin:\$PATH\" (and ensure it is in your shell profile)"
    fi
    [ -n "$profile" ] && [ -f "$profile" ] && . "$profile"
    go version 2>/dev/null || /usr/local/go/bin/go version

# Install Go linting and analysis tools (required for .golangci.yml, lint-go, lint-go-ci, vulncheck-go)
install-go-tools: install-go
    @go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
    @go install honnef.co/go/tools/cmd/staticcheck@latest
    @go install golang.org/x/vuln/cmd/govulncheck@latest
    @echo "Installed: golangci-lint, staticcheck, govulncheck"

# Lint all shell scripts with shellcheck (requires shellcheck in PATH, e.g. pacman -S shellcheck).
lint-sh:
    #!/usr/bin/env bash
    set -e
    root="{{ root_dir }}"
    pushd "$root" >/dev/null
    trap 'popd >/dev/null 2>/dev/null || true' EXIT
    command -v shellcheck >/dev/null 2>&1 || { echo "lint-sh: shellcheck not found. Install e.g. pacman -S shellcheck, apt install shellcheck, brew install shellcheck"; exit 1; }
    files=()
    while IFS= read -r -d '' p; do files+=("$p"); done < <(find . -type f -name '*.sh' ! -path '*/.git/*' ! -path '*/tmp/*' ! -path '*_bdd/tmp/*' -print0 | sort -z)
    if [ ${#files[@]} -eq 0 ]; then echo "No .sh scripts found."; exit 0; fi
    for f in "${files[@]}"; do
      echo "==> shellcheck $f"
      shellcheck "$root/$f"
    done
    echo "Shell script lint: OK"

# Run go vet and staticcheck (quick Go lint; use lint-go-ci for full suite).
# Runs in each workspace module so each go.mod is used correctly.
# Quick Go lint (go vet + staticcheck) in each module; use lint-go-ci for full suite.
lint-go: install-go-tools
    @for m in {{ go_modules }}; do \
      echo "==> go vet ./... ($m)"; \
      (pushd "$m" >/dev/null && go vet ./...); \
      echo "==> staticcheck ./... ($m)"; \
      (pushd "$m" >/dev/null && staticcheck ./...); \
    done

# Full Go lint suite via golangci-lint (uses repo-root .golangci.yml for all modules).
lint-go-ci: install-go-tools
    @for m in {{ go_modules }}; do \
      echo "==> golangci-lint run ./... ($m)"; \
      (pushd "$m" >/dev/null && golangci-lint run -c "{{ root_dir }}/.golangci.yml" ./...); \
    done

# Run vulnerability check on Go dependencies
vulncheck-go: install-go-tools
    @for m in {{ go_modules }}; do \
      echo "==> govulncheck ./... ($m)"; \
      (pushd "$m" >/dev/null && govulncheck ./...); \
    done

# Install markdownlint: create .markdownlint-cli2.jsonc if missing; clone/update
# https://github.com/cypher0n3/docs-as-code-tools markdownlint-rules into .markdownlint-rules/
# when the config points to that dir.
# Bootstrap markdownlint config and custom rules so lint-md works.
install-markdownlint:
    #!/usr/bin/env bash
    set -e
    pushd "{{ root_dir }}" >/dev/null
    trap 'popd >/dev/null 2>/dev/null || true' EXIT
    CONFIG=".markdownlint-cli2.jsonc"
    RULES_DIR=".markdownlint-rules"
    REPO_DIR=".markdownlint-repo"
    REPO_URL="https://github.com/cypher0n3/docs-as-code-tools.git"
    # Bootstrap config with customRules and ignores so lint-md works out of the box
    if [ ! -f "$CONFIG" ]; then
        echo "Creating $CONFIG with customRules in $RULES_DIR/ ..."
        printf '%s\n' '{"config":{"default":true,"extends":".markdownlint.yml"},"customRules":[".markdownlint-rules/allow-custom-anchors.js",".markdownlint-rules/ascii-only.js",".markdownlint-rules/document-length.js",".markdownlint-rules/fenced-code-under-heading.js",".markdownlint-rules/heading-min-words.js",".markdownlint-rules/heading-numbering.js",".markdownlint-rules/heading-title-case.js",".markdownlint-rules/no-duplicate-headings-normalized.js",".markdownlint-rules/no-empty-heading.js",".markdownlint-rules/no-h1-content.js",".markdownlint-rules/no-heading-like-lines.js",".markdownlint-rules/one-sentence-per-line.js"],"ignores":[".github/**","**/node_modules/**","tmp/**","**/*.plan.md"]}' > "$CONFIG"
    fi
    # Only clone/update rules repo if config references .markdownlint-rules/
    if ! grep -q '"\.markdownlint-rules/' "$CONFIG" 2>/dev/null && ! grep -q '"./\.markdownlint-rules/' "$CONFIG" 2>/dev/null; then
        echo "Config does not point to $RULES_DIR; skipping rules download."
        exit 0
    fi
    command -v git >/dev/null 2>&1 || { echo "Error: git required to download markdownlint rules."; exit 1; }
    if [ ! -d "$REPO_DIR" ]; then
        echo "Cloning docs-as-code-tools into $REPO_DIR ..."
        git clone --depth 1 "$REPO_URL" "$REPO_DIR"
        ln -sfn "$REPO_DIR/markdownlint-rules" "$RULES_DIR"
        echo "Rules installed at $RULES_DIR (symlink to $REPO_DIR/markdownlint-rules)."
    else
        echo "Updating $REPO_DIR ..."
        git -C "$REPO_DIR" fetch origin main
        git -C "$REPO_DIR" merge --ff-only origin/main || true
        # Symlink so config customRules paths resolve without copying files
        if [ ! -e "$RULES_DIR" ]; then
            ln -sfn "$REPO_DIR/markdownlint-rules" "$RULES_DIR"
            echo "Linked $RULES_DIR to $REPO_DIR/markdownlint-rules."
        fi
    fi

# Install gherkin-lint globally (npm). Used by lint-gherkin for .feature files in features/.
install-gherkin-lint:
    #!/usr/bin/env bash
    set -e
    if command -v gherkin-lint >/dev/null 2>&1; then
        echo "gherkin-lint already installed: $(gherkin-lint --version 2>/dev/null || echo 'present')"
        exit 0
    fi
    command -v npm >/dev/null 2>&1 || { echo "Error: npm not found. Install Node.js/npm (e.g. pacman -S nodejs npm) to install gherkin-lint."; exit 1; }
    echo "Installing gherkin-lint globally..."
    npm install -g gherkin-lint
    echo "Installed: gherkin-lint"

# Validate internal file links in docs/ (checks markdown hrefs to other files; in-doc #anchors ignored).
validate-doc-links:
    #!/usr/bin/env bash
    set -e
    pushd "{{ root_dir }}" >/dev/null
    trap 'popd >/dev/null 2>/dev/null || true' EXIT
    python3 .ci_scripts/validate_doc_links.py

# Validate docs/requirements: no duplicate REQ ids, sequential ids, each REQ has a spec ref.
validate-requirements:
    #!/usr/bin/env bash
    set -e
    pushd "{{ root_dir }}" >/dev/null
    trap 'popd >/dev/null 2>/dev/null || true' EXIT
    python3 .ci_scripts/check_requirements.py

# Validate Gherkin feature file conventions in features/
validate-feature-files:
    #!/usr/bin/env bash
    set -e
    pushd "{{ root_dir }}" >/dev/null
    trap 'popd >/dev/null 2>/dev/null || true' EXIT
    python3 .ci_scripts/validate_feature_files.py

# Lint Gherkin .feature files in features/ (requires gherkin-lint; run just install-gherkin-lint).
lint-gherkin: install-gherkin-lint
    #!/usr/bin/env bash
    set -e
    pushd "{{ root_dir }}" >/dev/null
    trap 'popd >/dev/null 2>/dev/null || true' EXIT
    gherkin-lint features/

# Check docs/tech_specs for duplicated spec text (CPD-style); output to stdout only.
# Pass script args as recipe args, e.g. just check-tech-spec-duplication --report docs/dev_docs/tech_spec_duplication_report.txt
# Find duplicated text in docs/tech_specs (CPD-style); pass --report PATH for file output.
check-tech-spec-duplication *ARGS:
    #!/usr/bin/env bash
    set -e
    pushd "{{ root_dir }}" >/dev/null
    trap 'popd >/dev/null 2>/dev/null || true' EXIT
    python3 .ci_scripts/check_tech_spec_duplication.py --no-fail {{ ARGS }}

# Lint Containerfiles and Dockerfile (hadolint). Uses hadolint from PATH, or podman run hadolint/hadolint.
lint-containerfiles:
    #!/usr/bin/env bash
    set -e
    root="{{ root_dir }}"
    pushd "$root" >/dev/null
    trap 'popd >/dev/null 2>/dev/null || true' EXIT
    files=()
    while IFS= read -r -d '' p; do files+=("$p"); done < <(find . -type f \( -name 'Containerfile*' -o -name 'Dockerfile*' \) ! -path '*/.git/*' -print0 | sort -z)
    if [ ${#files[@]} -eq 0 ]; then echo "No Containerfile/Dockerfile found."; exit 0; fi
    run_hadolint() {
        local f="$1"
        if command -v hadolint >/dev/null 2>&1; then
            hadolint "$root/$f"
        elif command -v podman >/dev/null 2>&1; then
            podman run --rm -v "$root:/lint:ro" -w /lint hadolint/hadolint hadolint "/lint/$f"
        elif command -v docker >/dev/null 2>&1; then
            docker run --rm -v "$root:/lint:ro" -w /lint hadolint/hadolint hadolint "/lint/$f"
        else
            echo "lint-containerfiles: install hadolint (e.g. pacman -S hadolint) or run with podman/docker"
            exit 1
        fi
    }
    for f in "${files[@]}"; do
        [ -f "$root/$f" ] || { echo "Missing $f"; exit 1; }
        echo "==> hadolint $f"
        run_hadolint "$f"
    done
    echo "Containerfile lint: OK"

# Lint Markdown (markdownlint-cli2; uses .markdownlint-cli2.jsonc). With no arguments lints all .md; pass paths to lint only those.
lint-md *PATHS:
    #!/usr/bin/env bash
    set -e
    pushd "{{ root_dir }}" >/dev/null
    trap 'popd >/dev/null 2>/dev/null || true' EXIT
    if [ -z "{{ PATHS }}" ]; then
    markdownlint-cli2 --fix '**/*.md'
    else
    markdownlint-cli2 --fix {{ PATHS }}
    fi

# Format Go code (each module in go_modules)
fmt-go: install-go
    @for m in {{ go_modules }}; do \
      echo "==> gofmt -s -w ($m)"; \
      (pushd "$m" >/dev/null && gofmt -s -w .); \
      echo "==> go mod tidy ($m)"; \
      (pushd "$m" >/dev/null && go mod tidy); \
    done

# Tidy and update deps in all Go modules (go mod tidy in each)
go-tidy: install-go
    @for m in {{ go_modules }}; do \
      echo "==> go mod tidy ($m)"; \
      (pushd "{{ root_dir }}/$m" >/dev/null && go mod tidy); \
    done

# Upgrade all Go dependencies to latest in each workspace module (go get -u ./... then tidy).
go-upgrade-deps: install-go
    @for m in {{ go_modules }}; do \
      echo "==> go get -u ./... ($m)"; \
      (pushd "{{ root_dir }}/$m" >/dev/null && go get -u ./... && go mod tidy); \
    done

# Stage all Go module directories (from go.work) with git add.
git-add-go-modules:
    @for m in {{ go_modules }}; do \
      echo "git add $m"; \
      git add "$m"; \
    done

# Built binaries go to each module's bin/ (orchestrator/bin, worker_node/bin, cynork/bin, agents/bin).
# Production = stripped + upx; dev = debug symbols. Usage: _build_prod <name> <pkg_path> <out_dir>; _build_dev <name> <pkg_path> <out_dir>.
_build_prod name pkg_path out_dir:
    #!/usr/bin/env bash
    set -e
    root="{{ root_dir }}"
    out="{{ out_dir }}"
    mkdir -p "$root/$out"
    pushd "$root" >/dev/null
    trap 'popd >/dev/null 2>/dev/null || true' EXIT
    echo "Building {{ name }} (production)..."
    CGO_ENABLED=0 GOEXPERIMENT=secret go build -ldflags="-s -w" -o "$root/$out/{{ name }}" {{ pkg_path }}
    if command -v upx >/dev/null 2>&1; then
      upx --best "$root/$out/{{ name }}" || { r=$?; [ "$r" -eq 2 ] && true || exit "$r"; }
    else
      echo "upx not found; install for smaller binary (e.g. pacman -S upx, apt install upx-ucl)"
    fi
    echo "Built: $root/$out/{{ name }}"

_build_dev name pkg_path out_dir:
    #!/usr/bin/env bash
    set -e
    root="{{ root_dir }}"
    out="{{ out_dir }}"
    mkdir -p "$root/$out"
    pushd "$root" >/dev/null
    trap 'popd >/dev/null 2>/dev/null || true' EXIT
    echo "Building {{ name }} (dev, debug)..."
    GOEXPERIMENT=secret go build -gcflags="all=-N -l" -o "$root/$out/{{ name }}-dev" {{ pkg_path }}
    echo "Built: $root/$out/{{ name }}-dev"

# --- Cynork ---
# Build CLI management client (production binary).
build-cynork: install-go
    @just _build_prod cynork ./cynork cynork/bin

# Build CLI management client (dev, debug symbols).
build-cynork-dev: install-go
    @just _build_dev cynork ./cynork cynork/bin

# --- Orchestrator ---
# Build orchestrator control-plane (production binary).
build-control-plane: install-go
    @just _build_prod control-plane ./orchestrator/cmd/control-plane orchestrator/bin

# Build orchestrator control-plane (dev, debug symbols).
build-control-plane-dev: install-go
    @just _build_dev control-plane ./orchestrator/cmd/control-plane orchestrator/bin

# Build user gateway (production binary).
build-user-gateway: install-go
    @just _build_prod user-gateway ./orchestrator/cmd/user-gateway orchestrator/bin

# Build user gateway (dev, debug symbols).
build-user-gateway-dev: install-go
    @just _build_dev user-gateway ./orchestrator/cmd/user-gateway orchestrator/bin

# Build API egress server (production binary).
build-api-egress: install-go
    @just _build_prod api-egress ./orchestrator/cmd/api-egress orchestrator/bin

# Build API egress server (dev, debug symbols).
build-api-egress-dev: install-go
    @just _build_dev api-egress ./orchestrator/cmd/api-egress orchestrator/bin

# Build MCP gateway (production binary).
build-mcp-gateway: install-go
    @just _build_prod mcp-gateway ./orchestrator/cmd/mcp-gateway orchestrator/bin

# Build MCP gateway (dev, debug symbols).
build-mcp-gateway-dev: install-go
    @just _build_dev mcp-gateway ./orchestrator/cmd/mcp-gateway orchestrator/bin

# --- Worker node ---
# Build worker API (production binary).
build-worker-api: install-go
    @just _build_prod worker-api ./worker_node/cmd/worker-api worker_node/bin

# Build worker API (dev, debug symbols).
build-worker-api-dev: install-go
    @just _build_dev worker-api ./worker_node/cmd/worker-api worker_node/bin

# Build node manager (production binary).
build-node-manager: install-go
    @just _build_prod node-manager ./worker_node/cmd/node-manager worker_node/bin

# Build node manager (dev, debug symbols).
build-node-manager-dev: install-go
    @just _build_dev node-manager ./worker_node/cmd/node-manager worker_node/bin

# Build inference proxy (production binary).
build-inference-proxy: install-go
    @just _build_prod inference-proxy ./worker_node/cmd/inference-proxy worker_node/bin

# Build inference proxy (dev, debug symbols).
build-inference-proxy-dev: install-go
    @just _build_dev inference-proxy ./worker_node/cmd/inference-proxy worker_node/bin

# --- Agents ---
# Build CyNode PMA agent (production binary).
build-cynode-pma: install-go
    @just _build_prod cynode-pma ./agents/cmd/cynode-pma agents/bin

# Build CyNode PMA agent (dev, debug symbols).
build-cynode-pma-dev: install-go
    @just _build_dev cynode-pma ./agents/cmd/cynode-pma agents/bin

# Build CyNode SBA agent (production binary).
build-cynode-sba: install-go
    @just _build_prod cynode-sba ./agents/cmd/cynode-sba agents/bin

# Build CyNode SBA agent (dev, debug symbols).
build-cynode-sba-dev: install-go
    @just _build_dev cynode-sba ./agents/cmd/cynode-sba agents/bin

# Build all production binaries (stripped + upx) into each module's bin/.
build: install-go
    @just build-cynork build-control-plane build-user-gateway build-api-egress build-mcp-gateway build-worker-api build-node-manager build-inference-proxy build-cynode-pma build-cynode-sba

# Build all dev binaries (debug symbols) into each module's bin/.
build-dev: install-go
    @just build-cynork-dev build-control-plane-dev build-user-gateway-dev build-api-egress-dev build-mcp-gateway-dev build-worker-api-dev build-node-manager-dev build-inference-proxy-dev build-cynode-pma-dev build-cynode-sba-dev

# Run Go tests
test-go: test-go-cover test-go-race test-go-e2e
    @:

# E2E tests are opt-in (RUN_E2E=1) so default test-go / ci stay fast.
test-go-e2e:
    #!/usr/bin/env bash
    set -e
    if [ "${RUN_E2E:-}" != "1" ]; then
        echo "Skipping e2e (set RUN_E2E=1 to run)"
        exit 0
    fi
    just e2e

# Minimum Go coverage (percent) required per package when running test-go-cover / ci
go_coverage_min := "90"
# testutil (test helpers) has no minimum; all other packages require go_coverage_min (90%)
go_coverage_min_control_plane := "90"
go_coverage_min_mcp_gateway := "90"
go_coverage_min_agents := "90"
go_coverage_min_sba := "90"  # internal/sba: agent/LLM paths need live Ollama or more mocks to reach 90%
go_coverage_min_sba_cmd := "90"  # cmd/cynode-sba: stdin path exercised via subprocess (not instrumented)
go_coverage_min_securestore := "86"  # internal/securestore: platform FIPS (getFIPSStatusLinux/Windows) not covered in unit tests

# Run Go tests with coverage for all go_modules; fail if any package is below go_coverage_min.
# Orchestrator uses testcontainers for Postgres when POSTGRES_TEST_DSN is unset (run just podman-setup first).
# Run Go tests with coverage; fail if any package below minimum (run podman-setup first for orchestrator).
test-go-cover: install-go podman-setup
    #!/usr/bin/env bash
    set -euo pipefail
    root="{{ root_dir }}"
    min="{{ go_coverage_min }}"
    fail=0
    failed=""
    mkdir -p "$root/tmp/go/coverage"
    echo ""
    echo "--- Go coverage (min ${min}% per package) ---"
    echo ""

    for m in {{ go_modules }}; do
      echo "==> $m: go test -coverprofile"
      out="$root/tmp/go/coverage/$m.coverage.out"
      if [ "$m" = "orchestrator" ]; then
        (pushd "$root/$m" >/dev/null && go test -p 1 ./... -coverprofile="$out" -covermode=atomic)
      else
        (pushd "$root/$m" >/dev/null && go test ./... -coverprofile="$out" -covermode=atomic)
      fi

      r=0
      min_cp="{{ go_coverage_min_control_plane }}"
      min_mcp="{{ go_coverage_min_mcp_gateway }}"
      min_agents="{{ go_coverage_min_agents }}"
      min_sba="{{ go_coverage_min_sba }}"
      min_sba_cmd="{{ go_coverage_min_sba_cmd }}"
      min_securestore="{{ go_coverage_min_securestore }}"
      below=$(awk -v min="$min" -v min_cp="$min_cp" -v min_mcp="$min_mcp" -v min_agents="$min_agents" -v min_sba="$min_sba" -v min_sba_cmd="$min_sba_cmd" -v min_securestore="$min_securestore" -v module="$m" '
        /^mode:/ { next }
        { path = $1; sub(/:.*/, "", path)
          n = split(path, a, "/")
          pkg = (n > 1) ? a[1] : "."
          for (i = 2; i < n; i++) pkg = pkg "/" a[i]
          stmts = $2; count = $3
          t[pkg] += stmts
          c[pkg] += (count > 0) ? stmts : 0
        }
        END {
          for (p in t) {
            pct = (t[p] > 0) ? (100 * c[p] / t[p]) : 0
            pct_rounded = int(pct * 10 + 0.5) / 10
            if (module == "agents" && p ~ /\/cmd\/cynode-sba$/) req = min_sba_cmd + 0
            else if (module == "agents" && p ~ /\/internal\/sba$/) req = min_sba + 0
            else if (module == "agents") req = min_agents + 0
            else if (module == "worker_node" && p ~ /\/internal\/securestore$/) req = min_securestore + 0
            else if (p ~ /\/cmd\/control-plane$/) req = min_cp + 0
            else if (p ~ /\/internal\/database$/) req = min_cp + 0
            else if (p ~ /\/internal\/testutil$/) req = 0
            else if (p ~ /\/cmd\/mcp-gateway$/) req = min_mcp + 0
            else req = min + 0
            if (pct_rounded < req) { printf "  %s %.1f%%\n", p, pct; e = 1 }
          }
          exit e + 0
        }
      ' "$out") || r=$?
      if [ "$r" -ne 0 ]; then
        echo "  [FAIL] packages below ${min}%:"
        echo "$below"
        fail=1
        failed="$failed${failed:+$'\n'}[$m]"$'\n'"$below"
      else
        echo "  [PASS] all packages ≥ ${min}%"
      fi
      echo ""
    done

    if [ "$fail" -ne 0 ]; then
      echo "--- Summary ---"
      echo "Packages below ${min}%:"
      echo "$failed"
      echo ""
      exit 1
    fi
    echo "--- Summary ---"
    echo "All packages meet coverage threshold (≥ ${min}%)."
    echo ""

# Run Go tests with race detector
test-go-race: install-go
    @for m in {{ go_modules }}; do \
      echo "==> go test -race ./... ($m)"; \
      (pushd "$m" >/dev/null && go test -race ./...); \
    done

# All linting (shell, Go, Python, Markdown, containers, doc links, feature files, Gherkin).
lint: lint-sh lint-go lint-go-ci lint-python lint-md validate-doc-links validate-feature-files lint-gherkin lint-containerfiles
    @:

# All tests
test: test-go
    @:

# BDD: run Godog suites for each go_modules module that has _bdd (from repo root).
# Orchestrator steps that need a DB are skipped unless POSTGRES_TEST_DSN is set.
# Optional timeout cancels tests if they exceed the duration (e.g. timeout="5m").
# Run with DB: POSTGRES_TEST_DSN="postgres://..." just test-bdd
# Run with timeout: just test-bdd timeout="10m"
# Run Godog BDD for modules with _bdd; set POSTGRES_TEST_DSN for DB steps; optional timeout=.
test-bdd timeout="": install-go
    #!/usr/bin/env bash
    set -e
    pushd "{{ root_dir }}" >/dev/null
    trap 'popd >/dev/null 2>/dev/null || true' EXIT
    extra=()
    [ -n "{{ timeout }}" ] && extra=(-timeout "{{ timeout }}")
    for m in {{ go_modules }}; do
      [ -d "./$m/_bdd" ] || continue
      go test -v "${extra[@]}" "./$m/_bdd" -count=1
    done


# Python dev setup (scripts/setup_dev.py). Usage: just setup-dev <command> [OPTIONS]
# Commands: start-db, stop-db, clean-db, migrate, build, build-e2e-images, start, stop, test-e2e, full-demo, help.
# Example: just setup-dev full-demo --stop-on-success
# Run setup_dev.py <command> (e.g. full-demo, build, start-db); just setup-dev full-demo --stop-on-success.
setup-dev CMD *ARGS:
    #!/usr/bin/env bash
    set -e
    pushd "{{ root_dir }}" >/dev/null
    trap 'popd >/dev/null 2>/dev/null || true' EXIT
    export PYTHONPATH="$PWD"
    python3 scripts/setup_dev.py {{ CMD }} {{ ARGS }}

# Python E2E test suite (scripts/test_scripts/run_e2e.py). Pass options through.
# Options: --no-build, --skip-ollama, --list; unittest: -v, -k PATTERN, -f.
# Example: just e2e --no-build -v
# Run Python E2E suite (run_e2e.py); options: --no-build, -v, etc.
e2e *ARGS:
    #!/usr/bin/env bash
    set -e
    pushd "{{ root_dir }}" >/dev/null
    trap 'popd >/dev/null 2>/dev/null || true' EXIT
    export PYTHONPATH="$PWD"
    python3 scripts/test_scripts/run_e2e.py {{ ARGS }}

# Create .venv and install Python lint tooling (scripts/requirements-lint.txt). Use with lint-python.
venv:
    #!/usr/bin/env bash
    set -e
    pushd "{{ root_dir }}" >/dev/null
    trap 'popd >/dev/null 2>/dev/null || true' EXIT
    command -v python3 >/dev/null 2>&1 || { echo "Error: python3 not found. Install Python 3 to create the venv."; exit 1; }
    python3 -m venv .venv
    .venv/bin/pip install -q --upgrade pip
    .venv/bin/pip install -q -r scripts/requirements-lint.txt
    echo "Created .venv with lint tooling. Use 'just lint-python' (it will use .venv when present)."

# Python linting (flake8, pylint, radon, xenon, vulture, bandit). Optional: just lint-python paths="scripts,other"
lint-python paths="scripts,.ci_scripts":
    #!/usr/bin/env bash
    # No set -e: we capture each linter exit code and fail at the end if any gating check failed
    pushd "{{ root_dir }}" >/dev/null
    trap 'popd >/dev/null 2>/dev/null || true' EXIT
    LINT_PATHS=$(echo "{{ paths }}" | tr ',' ' ')
    command -v python3 >/dev/null 2>&1 || { echo "Error: python3 not found. Install Python 3 to run Python linting."; exit 1; }
    # need: require tool in PATH or in .venv/bin (so "just venv" is enough)
    need() { command -v "$1" >/dev/null 2>&1 || [ -x .venv/bin/"$1" ] || { echo "Error: $1 not found. Install with: pip install $1 or run 'just venv'"; exit 1; }; }
    need flake8; need pylint; need radon; need xenon; need vulture; need bandit
    if [ -d .venv ]; then export PATH="$PWD/.venv/bin:$PATH"; fi
    export PYTHONPATH="$PWD/scripts"
    echo "Running flake8 on Python scripts..."
    flake8 $LINT_PATHS --jobs=1; FLAKE8_RESULT=$?
    echo "Running pylint on Python scripts..."
    pylint --rcfile=.pylintrc $LINT_PATHS; PYLINT_RESULT=$?
    echo "Running radon complexity (non-gating)..."
    radon cc -s -a $LINT_PATHS || true
    echo "Running xenon cyclomatic complexity check (fail if any block > C)..."
    xenon -b C $LINT_PATHS; XENON_RESULT=$?
    echo "Running radon maintainability metrics (non-gating)..."
    radon mi -s $LINT_PATHS || true
    echo "Running radon maintainability check (fail if any module MI rank C)..."
    # radon mi -j outputs JSON; we parse it and fail if any file has rank C
    TMP_MI=$(mktemp); radon mi -j $LINT_PATHS -O "$TMP_MI"
    python3 -c "import sys, json; d=json.load(open(sys.argv[1])); bad=[k for k,v in d.items() if v.get('rank')=='C']; [print('MI rank C (low maintainability):', f) for f in bad]; sys.exit(1 if bad else 0)" "$TMP_MI"; MI_RESULT=$?
    rm -f "$TMP_MI"
    echo "Running vulture unused code detection (non-gating)..."
    vulture $LINT_PATHS --min-confidence 80 || true
    echo "Running bandit security scan (non-gating)..."
    bandit -r $LINT_PATHS -c bandit.yaml; BANDIT_RESULT=$?
    echo ""; echo "Lint exit codes: flake8=$FLAKE8_RESULT pylint=$PYLINT_RESULT xenon=$XENON_RESULT radon_mi=$MI_RESULT bandit=$BANDIT_RESULT"
    # Fail if any gating linter reported errors (radon cc/mi -s and vulture are non-gating)
    [ "$FLAKE8_RESULT" -ne 0 ] || [ "$PYLINT_RESULT" -ne 0 ] || [ "$XENON_RESULT" -ne 0 ] || [ "$MI_RESULT" -ne 0 ] || [ "$BANDIT_RESULT" -ne 0 ] && exit 1; exit 0
