# CyNodeAI dev environment and scripting
# Run `just` for a list of recipes. Requires: https://github.com/casey/just
# shellcheck disable=SC2148,SC1007
# (justfile: not a shell script; recipe bodies are run by just with set shell)

set shell := ["bash", "-uc"]

# Go version to install when using install-go (match go.mod / toolchain)
go_version := "1.25.7"

default:
    @just --list

# Full dev setup: podman, Go, and Go tools (incl. deps for .golangci.yml and lint-go-ci)
setup:
    just install-podman
    just install-go
    just install-go-tools
    just install-markdownlint

# Install Podman if not already installed (Linux: distro package; macOS: Homebrew)
install-podman:
    #!/usr/bin/env bash
    set -e
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

# Install Go (prefer distro package; fall back to go.dev tarball for {{ go_version }})
install-go:
    #!/usr/bin/env bash
    set -e
    go_version="{{ go_version }}"
    want_minor="${go_version#*.}"
    want_minor="${want_minor%%.*}"
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
    if [ "$did_apt_go" = 1 ]; then
        echo "Removing too-old golang-go package before tarball install"
        sudo apt-get remove -y golang-go || true
    fi
    tarball_version="$go_version"
    case "$tarball_version" in
        *.*.*) ;;
        *) tarball_version="${go_version}.0";;
    esac
    echo "Installing Go $tarball_version from go.dev/dl (distro package missing or too old)"
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
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
            echo "export PATH=\"$go_path:\$PATH\"" >> "$profile"
            echo "Added $go_path to PATH in $profile"
        fi
    else
        echo "Add to PATH: export PATH=\"$go_path:\$PATH\" (and ensure it is in your shell profile)"
    fi
    /usr/local/go/bin/go version

# Install Go linting and analysis tools (required for .golangci.yml, lint-go, lint-go-ci, vulncheck-go)
install-go-tools:
    @go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
    @go install honnef.co/go/tools/cmd/staticcheck@latest
    @go install golang.org/x/vuln/cmd/govulncheck@latest
    @echo "Installed: golangci-lint, staticcheck, govulncheck"

# Run go vet and staticcheck (quick Go lint; use lint-go-ci for full suite)
lint-go:
    go vet ./...
    staticcheck ./...

# Full Go lint suite via golangci-lint (uses .golangci.yml)
lint-go-ci:
    golangci-lint run -c .golangci.yml ./...

# Run vulnerability check on Go dependencies
vulncheck-go:
    govulncheck ./...

# Install markdownlint: create .markdownlint-cli2.jsonc if missing; clone/update
# https://github.com/cypher0n3/docs-as-code-tools markdownlint-rules into .markdownlint-rules/
# when the config points to that dir.
install-markdownlint:
    #!/usr/bin/env bash
    set -e
    CONFIG=".markdownlint-cli2.jsonc"
    RULES_DIR=".markdownlint-rules"
    REPO_DIR=".markdownlint-repo"
    REPO_URL="https://github.com/cypher0n3/docs-as-code-tools.git"
    if [ ! -f "$CONFIG" ]; then
        echo "Creating $CONFIG with customRules in $RULES_DIR/ ..."
        printf '%s\n' '{"config":{"default":true,"extends":".markdownlint.yml"},"customRules":[".markdownlint-rules/allow-custom-anchors.js",".markdownlint-rules/ascii-only.js",".markdownlint-rules/document-length.js",".markdownlint-rules/fenced-code-under-heading.js",".markdownlint-rules/heading-min-words.js",".markdownlint-rules/heading-numbering.js",".markdownlint-rules/heading-title-case.js",".markdownlint-rules/no-duplicate-headings-normalized.js",".markdownlint-rules/no-empty-heading.js",".markdownlint-rules/no-h1-content.js",".markdownlint-rules/no-heading-like-lines.js",".markdownlint-rules/one-sentence-per-line.js"],"ignores":[".github/**","**/node_modules/**","tmp/**","**/*.plan.md"]}' > "$CONFIG"
    fi
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
        if [ ! -e "$RULES_DIR" ]; then
            ln -sfn "$REPO_DIR/markdownlint-rules" "$RULES_DIR"
            echo "Linked $RULES_DIR to $REPO_DIR/markdownlint-rules."
        fi
    fi

# Lint Markdown (markdownlint-cli2; uses .markdownlint-cli2.jsonc)
lint-markdown:
    markdownlint-cli2 '**/*.md'

# Format Go code
fmt-go:
    gofmt -s -w .
    go mod tidy

# Run Go tests
test-go:
    go test ./...

# Run Go tests with race detector
test-go-race:
    go test -race ./...

# All linting (Go quick + Go full + Python + Markdown)
lint:
    just lint-go
    just lint-go-ci
    just lint-python
    just lint-markdown

# All tests
test:
    just test-go

# Local CI: all lint, all tests, Go vuln check
ci:
    just lint
    just test
    just vulncheck-go

# Create .venv and install Python lint tooling (scripts/requirements-lint.txt). Use with lint-python.
venv:
    @command -v python3 >/dev/null 2>&1 || { echo "Error: python3 not found. Install Python 3 to create the venv."; exit 1; }
    @python3 -m venv .venv
    @.venv/bin/pip install -q --upgrade pip
    @.venv/bin/pip install -q -r scripts/requirements-lint.txt
    @echo "Created .venv with lint tooling. Use 'just lint-python' (it will use .venv when present)."

# Alias for venv (matches install-* naming for setup)
install-python-venv:
    just venv

# Python linting (flake8, pylint, radon, xenon, vulture, bandit). Optional: just lint-python paths="scripts,other"
lint-python paths="scripts":
    #!/usr/bin/env bash
    LINT_PATHS=$(echo "{{ paths }}" | tr ',' ' ')
    command -v python3 >/dev/null 2>&1 || { echo "Error: python3 not found. Install Python 3 to run Python linting."; exit 1; }
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
    TMP_MI=$(mktemp); radon mi -j $LINT_PATHS -O "$TMP_MI"
    python3 -c "import sys, json; d=json.load(open(sys.argv[1])); bad=[k for k,v in d.items() if v.get('rank')=='C']; [print('MI rank C (low maintainability):', f) for f in bad]; sys.exit(1 if bad else 0)" "$TMP_MI"; MI_RESULT=$?
    rm -f "$TMP_MI"
    echo "Running vulture unused code detection (non-gating)..."
    vulture $LINT_PATHS --min-confidence 80 || true
    echo "Running bandit security scan (non-gating)..."
    bandit -r $LINT_PATHS; BANDIT_RESULT=$?
    echo ""; echo "Lint exit codes: flake8=$FLAKE8_RESULT pylint=$PYLINT_RESULT xenon=$XENON_RESULT radon_mi=$MI_RESULT bandit=$BANDIT_RESULT"
    [ "$FLAKE8_RESULT" -ne 0 ] || [ "$PYLINT_RESULT" -ne 0 ] || [ "$XENON_RESULT" -ne 0 ] || [ "$MI_RESULT" -ne 0 ] || [ "$BANDIT_RESULT" -ne 0 ] && exit 1; exit 0
