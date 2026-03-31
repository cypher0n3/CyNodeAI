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

# Local CI: all lint, all tests (with 90% coverage), Go vuln check, BDD suites (Godog strict: pending/undefined/ambiguous steps fail).
ci: build-dev lint vulncheck-go test-go-cover bdd-ci
    @:

# Lint delegation (implemented in .ci_scripts/justfile).
lint:
    @just .ci_scripts/lint
# Shell script lint (shellcheck).
lint-sh:
    @just .ci_scripts/lint-sh
# Go vet and staticcheck in all workspace modules.
lint-go:
    @just .ci_scripts/lint-go
# Golangci-lint in all workspace modules.
lint-go-ci:
    @just .ci_scripts/lint-go-ci
# Go vulnerability check (govulncheck) in all workspace modules.
vulncheck-go:
    @just .ci_scripts/vulncheck-go
# Install markdownlint-cli2 and custom rules (for lint-md).
install-markdownlint:
    @just .ci_scripts/install-markdownlint
# Install gherkin-lint (for lint-gherkin).
install-gherkin-lint:
    @just .ci_scripts/install-gherkin-lint
# Validate internal doc links and anchors.
validate-doc-links:
    @just .ci_scripts/validate-doc-links
# Validate requirements docs (IDs, ordering, spec refs).
validate-requirements:
    @just .ci_scripts/validate-requirements
# Validate feature files (narrative, tags, traceability).
validate-feature-files:
    @just .ci_scripts/validate-feature-files
# Check E2E test tags and class layout.
check-e2e-tags:
    @just .ci_scripts/check-e2e-tags
# Check E2E scripts reference requirements correctly.
check-e2e-requirements-traces:
    @just .ci_scripts/check-e2e-requirements-traces
# Lint Gherkin feature files.
lint-gherkin:
    @just .ci_scripts/lint-gherkin
# Check tech specs for duplicated blocks (optional args).
check-tech-spec-duplication *ARGS:
    @just .ci_scripts/check-tech-spec-duplication {{ ARGS }}
# Lint Containerfiles/Dockerfiles (hadolint).
lint-containerfiles:
    @just .ci_scripts/lint-containerfiles
# Lint Markdown (markdownlint-cli2). Pass paths or omit for all.
lint-md *PATHS:
    @just .ci_scripts/lint-md {{ PATHS }}
# Format Go code and run go mod tidy in all modules.
go-fmt:
    @just .ci_scripts/go-fmt
# Lint Python (flake8, pylint, etc.). Default paths: scripts, .ci_scripts.
lint-python paths="scripts,.ci_scripts":
    @just .ci_scripts/lint-python paths="{{ paths }}"

# Local docs check: lint Markdown, validate doc links, validate feature files.
docs-check *PATHS:
    @just fix-cynode {{ PATHS }}
    @just .ci_scripts/lint-md {{ PATHS }}
    @just .ci_scripts/validate-doc-links
    # TODO: remove --no-duplicate-check once tech_spec duplicates are cleaned up
    @just .ci_scripts/check-tech-specs --no-duplicate-check
    @just .ci_scripts/validate-requirements
    @just .ci_scripts/validate-feature-files

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
            case "$f" in
              *.[Mm][Dd]) ;;
              *) continue ;;
            esac
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

# Build all modules (prod binaries).
build:
    @just orchestrator/build
    @just worker_node/build
    @just cynork/build
    @just agents/build
# Build all modules (dev binaries, no strip).
build-dev:
    @just orchestrator/build-dev
    @just worker_node/build-dev
    @just cynork/build-dev
    @just agents/build-dev
# Build cynork CLI (prod).
build-cynork:
    @just cynork/build
# Build cynork CLI (dev).
build-cynork-dev:
    @just cynork/build-dev
# Build control-plane (orchestrator).
build-control-plane:
    @just orchestrator/build-control-plane
# Build control-plane dev binary.
build-control-plane-dev:
    @just orchestrator/build-control-plane-dev
# Build user-gateway (orchestrator).
build-user-gateway:
    @just orchestrator/build-user-gateway
# Build user-gateway dev binary.
build-user-gateway-dev:
    @just orchestrator/build-user-gateway-dev
# Build api-egress (orchestrator).
build-api-egress:
    @just orchestrator/build-api-egress
# Build api-egress dev binary.
build-api-egress-dev:
    @just orchestrator/build-api-egress-dev
# Build mcp-gateway (orchestrator).
build-mcp-gateway:
    @just orchestrator/build-mcp-gateway
# Build mcp-gateway dev binary.
build-mcp-gateway-dev:
    @just orchestrator/build-mcp-gateway-dev
# Build worker-api (worker_node).
build-worker-api:
    @just worker_node/build-worker-api
# Build worker-api dev binary.
build-worker-api-dev:
    @just worker_node/build-worker-api-dev
# Build node-manager (worker_node).
build-node-manager:
    @just worker_node/build-node-manager
# Build node-manager dev binary.
build-node-manager-dev:
    @just worker_node/build-node-manager-dev
# Build inference-proxy (worker_node).
build-inference-proxy:
    @just worker_node/build-inference-proxy
# Build inference-proxy dev binary.
build-inference-proxy-dev:
    @just worker_node/build-inference-proxy-dev
# Build cynode-pma (agents).
build-cynode-pma:
    @just agents/build-cynode-pma
# Build cynode-pma dev binary.
build-cynode-pma-dev:
    @just agents/build-cynode-pma-dev
# Build cynode-sba (agents).
build-cynode-sba:
    @just agents/build-cynode-sba
# Build cynode-sba dev binary.
build-cynode-sba-dev:
    @just agents/build-cynode-sba-dev
# Build all container images. Pass ARGS (e.g. --no-cache) to docker/podman build.
build-images *ARGS:
    @just orchestrator/build-control-plane-image {{ ARGS }}
    @just orchestrator/build-user-gateway-image {{ ARGS }}
    @just orchestrator/build-mcp-gateway-image {{ ARGS }}
    @just orchestrator/build-api-egress-image {{ ARGS }}
    @just worker_node/build-node-manager-image {{ ARGS }}
    @just worker_node/build-inference-proxy-image {{ ARGS }}
    @just agents/build-cynode-pma-image {{ ARGS }}
    @just agents/build-cynode-sba-image {{ ARGS }}
# Build control-plane container image.
build-control-plane-image *ARGS:
    @just orchestrator/build-control-plane-image {{ ARGS }}
# Build user-gateway container image.
build-user-gateway-image *ARGS:
    @just orchestrator/build-user-gateway-image {{ ARGS }}
# Build mcp-gateway container image.
build-mcp-gateway-image *ARGS:
    @just orchestrator/build-mcp-gateway-image {{ ARGS }}
# Build api-egress container image.
build-api-egress-image *ARGS:
    @just orchestrator/build-api-egress-image {{ ARGS }}
# Build cynode-pma container image.
build-cynode-pma-image *ARGS:
    @just agents/build-cynode-pma-image {{ ARGS }}
# Build cynode-sba container image.
build-cynode-sba-image *ARGS:
    @just agents/build-cynode-sba-image {{ ARGS }}
# Build worker-api container image.
build-worker-api-image *ARGS:
    @just worker_node/build-worker-api-image {{ ARGS }}
# Build node-manager container image.
build-node-manager-image *ARGS:
    @just worker_node/build-node-manager-image {{ ARGS }}
# Build inference-proxy container image.
build-inference-proxy-image *ARGS:
    @just worker_node/build-inference-proxy-image {{ ARGS }}

# Test (repo-wide; runs across all go_modules).
test-go: test-go-cover test-go-race test-go-e2e
    @:

go_coverage_min := "90"
go_coverage_min_control_plane := "90"
# cmd/control-plane only (internal/database still uses go_coverage_min_control_plane above).
go_coverage_min_control_plane_cmd := "89"
go_coverage_min_mcp_gateway := "89"
go_coverage_min_artifacts := "89"
go_coverage_min_agents := "90"
go_coverage_min_sba := "90"
go_coverage_min_sba_cmd := "90"
go_coverage_min_securestore := "90"

# Run Python E2E suite (skipped unless RUN_E2E=1).
test-go-e2e:
    #!/usr/bin/env bash
    set -e
    if [ "${RUN_E2E:-}" != "1" ]; then echo "Skipping e2e (set RUN_E2E=1 to run)"; exit 0; fi
    just e2e

# Go tests with coverage; fails if any package below threshold.
test-go-cover: install-go podman-setup
    #!/usr/bin/env bash
    set -euo pipefail
    # Disable Ryuk reaper for Podman/crun compatibility (orchestrator testcontainers).
    export TESTCONTAINERS_RYUK_DISABLED=true
    root="{{ root_dir }}"
    min="{{ go_coverage_min }}"
    fail=0
    failed=""
    mkdir -p "$root/tmp/go/coverage"
    echo ""; echo "--- Go coverage (min ${min}% per package) ---"; echo ""
    for m in {{ go_modules }}; do
      echo "==> $m: go test -coverprofile"
      out="$root/tmp/go/coverage/$m.coverage.out"
      if [ "$m" = "orchestrator" ]; then
        (pushd "$root/$m" >/dev/null && go test -p 1 -count=1 ./... -coverprofile="$out" -covermode=atomic)
      else
        (pushd "$root/$m" >/dev/null && go test ./... -coverprofile="$out" -covermode=atomic)
      fi
      r=0
      min_cp="{{ go_coverage_min_control_plane }}"
      min_cp_cmd="{{ go_coverage_min_control_plane_cmd }}"
      min_mcp="{{ go_coverage_min_mcp_gateway }}"
      min_art="{{ go_coverage_min_artifacts }}"
      min_agents="{{ go_coverage_min_agents }}"
      min_sba="{{ go_coverage_min_sba }}"
      min_sba_cmd="{{ go_coverage_min_sba_cmd }}"
      min_securestore="{{ go_coverage_min_securestore }}"
      below=$(awk -v min="$min" -v min_cp="$min_cp" -v min_cp_cmd="$min_cp_cmd" -v min_mcp="$min_mcp" -v min_art="$min_art" -v min_agents="$min_agents" -v min_sba="$min_sba" -v min_sba_cmd="$min_sba_cmd" -v min_securestore="$min_securestore" -v module="$m" '
        /^mode:/ { next }
        { path = $1; sub(/:.*/, "", path); n = split(path, a, "/"); pkg = (n > 1) ? a[1] : "."; for (i = 2; i < n; i++) pkg = pkg "/" a[i]; stmts = $2; count = $3; t[pkg] += stmts; c[pkg] += (count > 0) ? stmts : 0 }
        END {
          for (p in t) {
            pct = (t[p] > 0) ? (100 * c[p] / t[p]) : 0; pct_rounded = int(pct * 10 + 0.5) / 10
            if (module == "agents" && p ~ /\/cmd\/cynode-sba$/) req = min_sba_cmd + 0
            else if (module == "agents" && p ~ /\/internal\/sba$/) req = min_sba + 0
            else if (module == "agents") req = min_agents + 0
            else if (module == "worker_node" && p ~ /\/internal\/securestore$/) req = min_securestore + 0
            else if (p ~ /\/cmd\/control-plane$/) req = min_cp_cmd + 0
            else if (p ~ /\/internal\/database$/) req = min_cp + 0
            else if (module == "orchestrator" && p ~ /\/internal\/artifacts$/) req = min_art + 0
            else if (p ~ /\/internal\/testutil$/) req = 0
            else if (p ~ /\/cmd\/mcp-gateway$/) req = min_mcp + 0
            else req = min + 0
            if (pct_rounded < req) { printf "  %s %.1f%%\n", p, pct; e = 1 }
          }
          exit e + 0
        }
      ' "$out") || r=$?
      if [ "$r" -ne 0 ]; then echo "  [FAIL] packages below ${min}%:"; echo "$below"; fail=1; failed="$failed${failed:+$'\n'}[$m]"$'\n'"$below"; else echo "  [PASS] all packages ≥ ${min}%"; fi
      echo ""
    done
    if [ "$fail" -ne 0 ]; then echo "--- Summary ---"; echo "Packages below ${min}%:"; echo "$failed"; echo ""; exit 1; fi
    echo "--- Summary ---"; echo "All packages meet coverage threshold (≥ ${min}%)."; echo ""

# Go tests with -race detector in all modules.
test-go-race: install-go
    @for m in {{ go_modules }}; do echo "==> go test -race ./... ($m)"; (pushd "$m" >/dev/null && go test -race ./...); done

# Alias for test-go (cover + race + e2e when RUN_E2E=1).
test: test-go
    @:

# Run BDD tests (_bdd packages) in all modules.
# Go's default `go test` timeout is 10m per package; full Godog suites usually exceed that, so we pass
# -timeout explicitly (default 30m per module). Override: `just test-bdd timeout=45m` or `timeout=0` (no limit).
test-bdd timeout="30m": install-go podman-setup
    #!/usr/bin/env bash
    set -e
    # Disable Ryuk reaper: incompatible with Podman/crun (executable /bin/ryuk not found). Containers are still terminated by TestMain.
    export TESTCONTAINERS_RYUK_DISABLED=true
    # Ensure rootless Podman socket is used when available (same as test code expects).
    if [ -z "${DOCKER_HOST:-}" ] && [ -n "${XDG_RUNTIME_DIR:-}" ] && [ -S "${XDG_RUNTIME_DIR}/podman/podman.sock" ]; then
      export DOCKER_HOST="unix://${XDG_RUNTIME_DIR}/podman/podman.sock"
    fi
    # Remove leftover testcontainers from a previous crashed run so ports are free.
    if command -v podman >/dev/null 2>&1; then
      ids=$(podman ps -aq --filter "label=org.testcontainers.golang.sessionId" 2>/dev/null) || true
      if [ -n "$ids" ]; then
        podman rm -f $ids 2>/dev/null || true
      fi
    fi
    pushd "{{ root_dir }}" >/dev/null
    trap 'popd >/dev/null 2>/dev/null || true' EXIT
    for m in {{ go_modules }}; do
      [ -d "./$m/_bdd" ] || continue
      go test -v -timeout "{{ timeout }}" "./$m/_bdd" -count=1
    done

# BDD with github.com/cucumber/godog strict mode for agents/_bdd (GODOG_STRICT=1): fails on pending, undefined, or ambiguous steps.
# Other _bdd modules run the same tests without Strict until their step bindings are complete (run with GODOG_STRICT=1 per module locally to verify).
bdd-ci:
    #!/usr/bin/env bash
    set -e
    export GODOG_STRICT=1
    just test-bdd

# Explain how BDD coverage relates to `just test-go-cover`. Godog suites run as separate `go test`
# packages under each module's `_bdd/`; they do not merge into the unit-test coverage profiles.
# For BDD-only numbers: `cd <module> && go test -coverprofile=/tmp/bdd.cov -coverpkg=./... ./_bdd`.
test-coverage-bdd-vs-unit:
    @printf '%s\n' "Unit tests (per-package thresholds): just test-go-cover" "BDD: just test-bdd — separate metric; optional BDD coverprofile example above."

# Dev setup (scripts/justfile). Usage: just setup-dev <command> [ARGS]. Run just setup-dev help.
setup-dev CMD *ARGS:
    @just scripts/{{ CMD }} {{ ARGS }}

# E2E test suite (scripts/justfile; runs scripts/test_scripts/run_e2e.py).
e2e *ARGS:
    @just scripts/e2e {{ ARGS }}

# Create .venv and install Python deps: lint (scripts/requirements-lint.txt) and E2E (scripts/requirements-e2e.txt). Use with lint-python and just e2e.
venv:
    #!/usr/bin/env bash
    set -e
    pushd "{{ root_dir }}" >/dev/null
    trap 'popd >/dev/null 2>/dev/null || true' EXIT
    command -v python3 >/dev/null 2>&1 || { echo "Error: python3 not found. Install Python 3 to create the venv."; exit 1; }
    python3 -m venv .venv
    .venv/bin/pip install -q --upgrade pip
    .venv/bin/pip install -q -r scripts/requirements-lint.txt
    .venv/bin/pip install -q -r scripts/requirements-e2e.txt
    echo "Created .venv with lint and E2E tooling. Use 'just lint-python' and 'just e2e' (they use .venv when present)."
