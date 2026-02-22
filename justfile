# CyNodeAI dev environment and scripting
# Run `just` for a list of recipes. Requires: https://github.com/casey/just
# shellcheck disable=SC2148,SC1007
# (justfile: not a shell script; recipe bodies are run by just with set shell)

# Strict bash for all recipes (explicit errors, safe pipelines).
set shell := ["bash", "-euo", "pipefail", "-c"]

# Go version to install when using install-go (match go.mod / toolchain)
go_version := "1.25.7"

# Directory containing this justfile (repo root when justfile is at root)
root_dir := justfile_directory()

# Workspace modules (explicit, stable).
go_modules := "go_shared_libs orchestrator worker_node cynork"

default:
    @just --list

# Local CI: all lint, all tests (with 90% coverage), Go vuln check, BDD suites.
# test-go-cover runs coverage for all go_modules (orchestrator uses testcontainers for Postgres when POSTGRES_TEST_DSN unset).
# test-bdd runs Godog BDD for orchestrator and worker_node (orchestrator scenarios need POSTGRES_TEST_DSN for DB steps).
ci: lint-go lint-go-ci vulncheck-go lint-python lint-md validate-doc-links validate-feature-files test-go-cover test-bdd lint-containerfiles
    @:

# Local docs check: lint Markdown, validate doc links, validate feature files.
docs-check: fix-cynode lint-md validate-doc-links validate-feature-files
    @:

# Fix all instances of "Cynode" to "CyNode" in all Markdown files.
fix-cynode:
    #!/usr/bin/env bash
    set -e
    if [ "$(uname)" = "Darwin" ]; then
        find "{{ root_dir }}" -type f -iname '*.md' -exec sed -i '' 's/Cynode/CyNode/g' {} \;
    else
        find "{{ root_dir }}" -type f -iname '*.md' -exec sed -i "s/Cynode/CyNode/g" {} \;
    fi

# Full dev setup: podman, Go, and Go tools (incl. deps for .golangci.yml and lint-go-ci)
setup: install-podman install-go install-go-tools install-markdownlint
    @:

# Remove temporary files, .out files, compiled binaries, coverage artifacts, and Go test cache.
clean:
    #!/usr/bin/env bash
    set -e
    root="{{ root_dir }}"
    echo "Cleaning $root ..."
    find "$root" -maxdepth 3 -type f -name '*.out' ! -path '*/.git/*' -delete 2>/dev/null || true
    find "$root" -maxdepth 4 -type f -name 'coverage.*' ! -path '*/.git/*' -delete 2>/dev/null || true
    for m in {{ go_modules }}; do
      (cd "$root/$m" && go clean -testcache 2>/dev/null) || true
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

# Run go vet and staticcheck (quick Go lint; use lint-go-ci for full suite).
# Runs in each workspace module so each go.mod is used correctly.
lint-go: install-go-tools
    @for m in {{ go_modules }}; do \
      echo "==> go vet ./... ($m)"; \
      (cd "$m" && go vet ./...); \
      echo "==> staticcheck ./... ($m)"; \
      (cd "$m" && staticcheck ./...); \
    done

# Full Go lint suite via golangci-lint (uses repo-root .golangci.yml for all modules).
lint-go-ci: install-go-tools
    @for m in {{ go_modules }}; do \
      echo "==> golangci-lint run ./... ($m)"; \
      (cd "$m" && golangci-lint run -c "{{ root_dir }}/.golangci.yml" ./...); \
    done

# Run vulnerability check on Go dependencies
vulncheck-go: install-go-tools
    @for m in {{ go_modules }}; do \
      echo "==> govulncheck ./... ($m)"; \
      (cd "$m" && govulncheck ./...); \
    done

# Install markdownlint: create .markdownlint-cli2.jsonc if missing; clone/update
# https://github.com/cypher0n3/docs-as-code-tools markdownlint-rules into .markdownlint-rules/
# when the config points to that dir.
install-markdownlint:
    #!/usr/bin/env bash
    set -e
    pushd "{{ root_dir }}" >/dev/null
    trap 'popd >/dev/null 2>/dev/null' EXIT
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

# Validate internal file links in docs/ (checks markdown hrefs to other files; in-doc #anchors ignored).
validate-doc-links:
    #!/usr/bin/env bash
    set -e
    pushd "{{ root_dir }}" >/dev/null
    trap 'popd >/dev/null 2>/dev/null' EXIT
    python3 .ci_scripts/validate_doc_links.py

# Validate Gherkin feature file conventions in features/
validate-feature-files:
    #!/usr/bin/env bash
    set -e
    pushd "{{ root_dir }}" >/dev/null
    trap 'popd >/dev/null 2>/dev/null' EXIT
    python3 .ci_scripts/validate_feature_files.py

# Check docs/tech_specs for duplicated spec text (CPD-style); output to stdout only.
# Pass script args as recipe args, e.g. just check-tech-spec-duplication --report dev_docs/tech_spec_duplication_report.txt
check-tech-spec-duplication *ARGS:
    #!/usr/bin/env bash
    set -e
    pushd "{{ root_dir }}" >/dev/null
    trap 'popd >/dev/null 2>/dev/null' EXIT
    python3 .ci_scripts/check_tech_spec_duplication.py --no-fail {{ ARGS }}

# Lint Containerfiles and Dockerfile (hadolint). Uses hadolint from PATH, or podman run hadolint/hadolint.
lint-containerfiles:
    #!/usr/bin/env bash
    set -e
    root="{{ root_dir }}"
    pushd "$root" >/dev/null
    trap 'popd >/dev/null 2>/dev/null' EXIT
    files=()
    while IFS= read -r -d '' p; do files+=("$p"); done < <(find . -type f \( -name 'Containerfile' -o -name 'Dockerfile' \) ! -path '*/.git/*' -print0 | sort -z)
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

# Lint Markdown (markdownlint-cli2; uses .markdownlint-cli2.jsonc)
lint-md target = "'**/*.md'":
    #!/usr/bin/env bash
    set -e
    pushd "{{ root_dir }}" >/dev/null
    trap 'popd >/dev/null 2>/dev/null' EXIT
    markdownlint-cli2 --fix {{ target }}

# Format Go code (each module in go_modules)
fmt-go: install-go
    @for m in {{ go_modules }}; do \
      echo "==> gofmt -s -w ($m)"; \
      (cd "$m" && gofmt -s -w .); \
      echo "==> go mod tidy ($m)"; \
      (cd "$m" && go mod tidy); \
    done

# Tidy and update deps in all Go modules (go mod tidy in each)
go-tidy: install-go
    @for m in {{ go_modules }}; do \
      echo "==> go mod tidy ($m)"; \
      (cd "{{ root_dir }}/$m" && go mod tidy); \
    done

# Upgrade all Go dependencies to latest in each workspace module (go get -u ./... then tidy).
go-upgrade-deps: install-go
    @for m in {{ go_modules }}; do \
      echo "==> go get -u ./... ($m)"; \
      (cd "{{ root_dir }}/$m" && go get -u ./... && go mod tidy); \
    done

# All built binaries go here (production and dev). Production = stripped + upx; dev = debug symbols.
bin_dir := root_dir + "/bin"

# Build one binary: production (stripped, upx'd) or dev (debug). Usage: _build_prod <name> <pkg_path>; _build_dev <name> <pkg_path>.
_build_prod name pkg_path:
    #!/usr/bin/env bash
    set -e
    root="{{ root_dir }}"
    mkdir -p "$root/bin"
    cd "$root"
    echo "Building {{ name }} (production)..."
    CGO_ENABLED=0 go build -ldflags="-s -w" -o "$root/bin/{{ name }}" {{ pkg_path }}
    if command -v upx >/dev/null 2>&1; then
      upx --best "$root/bin/{{ name }}"
    else
      echo "upx not found; install for smaller binary (e.g. pacman -S upx, apt install upx-ucl)"
    fi
    echo "Built: $root/bin/{{ name }}"

_build_dev name pkg_path:
    #!/usr/bin/env bash
    set -e
    root="{{ root_dir }}"
    mkdir -p "$root/bin"
    cd "$root"
    echo "Building {{ name }} (dev, debug)..."
    go build -gcflags="all=-N -l" -o "$root/bin/{{ name }}-dev" {{ pkg_path }}
    echo "Built: $root/bin/{{ name }}-dev"

# --- Cynork ---
build-cynork: install-go
    @just _build_prod cynork ./cynork

build-cynork-dev: install-go
    @just _build_dev cynork ./cynork

# --- Orchestrator ---
build-control-plane: install-go
    @just _build_prod control-plane ./orchestrator/cmd/control-plane

build-control-plane-dev: install-go
    @just _build_dev control-plane ./orchestrator/cmd/control-plane

build-user-gateway: install-go
    @just _build_prod user-gateway ./orchestrator/cmd/user-gateway

build-user-gateway-dev: install-go
    @just _build_dev user-gateway ./orchestrator/cmd/user-gateway

build-api-egress: install-go
    @just _build_prod api-egress ./orchestrator/cmd/api-egress

build-api-egress-dev: install-go
    @just _build_dev api-egress ./orchestrator/cmd/api-egress

build-mcp-gateway: install-go
    @just _build_prod mcp-gateway ./orchestrator/cmd/mcp-gateway

build-mcp-gateway-dev: install-go
    @just _build_dev mcp-gateway ./orchestrator/cmd/mcp-gateway

# --- Worker node ---
build-worker-api: install-go
    @just _build_prod worker-api ./worker_node/cmd/worker-api

build-worker-api-dev: install-go
    @just _build_dev worker-api ./worker_node/cmd/worker-api

build-node-manager: install-go
    @just _build_prod node-manager ./worker_node/cmd/node-manager

build-node-manager-dev: install-go
    @just _build_dev node-manager ./worker_node/cmd/node-manager

build-inference-proxy: install-go
    @just _build_prod inference-proxy ./worker_node/cmd/inference-proxy

build-inference-proxy-dev: install-go
    @just _build_dev inference-proxy ./worker_node/cmd/inference-proxy

# Build all production binaries (stripped + upx) into bin/.
build: install-go
    @just build-cynork build-control-plane build-user-gateway build-api-egress build-mcp-gateway build-worker-api build-node-manager build-inference-proxy

# Build all dev binaries (debug symbols) into bin/.
build-dev: install-go
    @just build-cynork-dev build-control-plane-dev build-user-gateway-dev build-api-egress-dev build-mcp-gateway-dev build-worker-api-dev build-node-manager-dev build-inference-proxy-dev

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
# Exception: control-plane main() cannot be covered by tests; allow ≥89%
go_coverage_min_control_plane := "89"

# Run Go tests with coverage for all go_modules; fail if any package is below go_coverage_min.
# Orchestrator uses testcontainers for Postgres when POSTGRES_TEST_DSN is unset (run just podman-setup first).
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
      (cd "$root/$m" && go test ./... -coverprofile="$out" -covermode=atomic)

      r=0
      min_cp="{{ go_coverage_min_control_plane }}"
      below=$(awk -v min="$min" -v min_cp="$min_cp" '
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
            req = (p ~ /\/cmd\/control-plane$/) ? min_cp + 0 : min + 0
            if (pct < req) { printf "  %s %.1f%%\n", p, pct; e = 1 }
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
      (cd "$m" && go test -race ./...); \
    done

# All linting (Go quick + Go full + Python + Markdown)
lint: lint-go lint-go-ci lint-python lint-md validate-doc-links validate-feature-files
    @:

# All tests
test: test-go
    @:

# BDD: run Godog suites for each go_modules module that has _bdd (from repo root).
# Orchestrator steps that need a DB are skipped unless POSTGRES_TEST_DSN is set.
# Optional timeout cancels tests if they exceed the duration (e.g. timeout="5m").
# Run with DB: POSTGRES_TEST_DSN="postgres://..." just test-bdd
# Run with timeout: just test-bdd timeout="10m"
test-bdd timeout="": install-go
    #!/usr/bin/env bash
    set -e
    cd "{{ root_dir }}"
    extra=()
    [ -n "{{ timeout }}" ] && extra=(-timeout "{{ timeout }}")
    for m in {{ go_modules }}; do
      [ -d "./$m/_bdd" ] || continue
      go test -v "${extra[@]}" "./$m/_bdd" -count=1
    done

# E2E: start Postgres, orchestrator, one worker node; run happy path (login, create task, get result).
# Requires: podman or docker, jq. Stops existing services first; leaves services running after.
# Remove PostgreSQL container and volume (so next start uses fresh DB, e.g. with pgvector image).
clean-db:
    @./scripts/setup-dev.sh clean-db

e2e:
    @./scripts/setup-dev.sh full-demo

e2e-stop:
    @./scripts/setup-dev.sh stop

# Create .venv and install Python lint tooling (scripts/requirements-lint.txt). Use with lint-python.
venv:
    #!/usr/bin/env bash
    set -e
    pushd "{{ root_dir }}" >/dev/null
    trap 'popd >/dev/null 2>/dev/null' EXIT
    command -v python3 >/dev/null 2>&1 || { echo "Error: python3 not found. Install Python 3 to create the venv."; exit 1; }
    python3 -m venv .venv
    .venv/bin/pip install -q --upgrade pip
    .venv/bin/pip install -q -r scripts/requirements-lint.txt
    echo "Created .venv with lint tooling. Use 'just lint-python' (it will use .venv when present)."

# Alias for venv (matches install-* naming for setup)
install-python-venv: venv
    @:

# Python linting (flake8, pylint, radon, xenon, vulture, bandit). Optional: just lint-python paths="scripts,other"
lint-python paths="scripts,.ci_scripts":
    #!/usr/bin/env bash
    # No set -e: we capture each linter exit code and fail at the end if any gating check failed
    pushd "{{ root_dir }}" >/dev/null
    trap 'popd >/dev/null 2>/dev/null' EXIT
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
    bandit -r $LINT_PATHS; BANDIT_RESULT=$?
    echo ""; echo "Lint exit codes: flake8=$FLAKE8_RESULT pylint=$PYLINT_RESULT xenon=$XENON_RESULT radon_mi=$MI_RESULT bandit=$BANDIT_RESULT"
    # Fail if any gating linter reported errors (radon cc/mi -s and vulture are non-gating)
    [ "$FLAKE8_RESULT" -ne 0 ] || [ "$PYLINT_RESULT" -ne 0 ] || [ "$XENON_RESULT" -ne 0 ] || [ "$MI_RESULT" -ne 0 ] || [ "$BANDIT_RESULT" -ne 0 ] && exit 1; exit 0
