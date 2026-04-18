#!/usr/bin/env bash
# setup.sh — PDFMaster one-command installer
# Usage: bash setup.sh
# Works on Ubuntu/Debian, Arch/Manjaro, Fedora/RHEL
set -euo pipefail

# ── Colours ───────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
BLUE='\033[0;34m'; BOLD='\033[1m'; NC='\033[0m'

step()  { echo -e "\n${BLUE}${BOLD}▸ $*${NC}"; }
ok()    { echo -e "  ${GREEN}✓${NC}  $*"; }
warn()  { echo -e "  ${YELLOW}⚠${NC}  $*"; }
fail()  { echo -e "  ${RED}✗${NC}  $*" >&2; exit 1; }
info()  { echo -e "      $*"; }

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo ""
echo -e "${BOLD}  PDFMaster — Automatic Setup${NC}"
echo    "  ══════════════════════════════"
echo ""

# ═══════════════════════════════════════════════════════════════════════
# STEP 1: Detect distro and install system dependencies
# ═══════════════════════════════════════════════════════════════════════
step "Detecting Linux distribution"

detect_distro() {
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        echo "${ID:-unknown}"
    elif command -v apt-get >/dev/null 2>&1; then echo "ubuntu"
    elif command -v pacman  >/dev/null 2>&1; then echo "arch"
    elif command -v dnf     >/dev/null 2>&1; then echo "fedora"
    else echo "unknown"
    fi
}

DISTRO=$(detect_distro)
ok "Distro: $DISTRO"

install_deps() {
    case "$DISTRO" in
    ubuntu|debian|linuxmint|pop|elementary|zorin)
        ok "Using apt"
        sudo apt-get update -qq
        sudo apt-get install -y \
            build-essential cmake pkg-config git \
            libmupdf-dev libqpdf-dev \
            zlib1g-dev libpng-dev libjpeg-turbo8-dev \
            libfreetype6-dev libharfbuzz-dev \
            libmujs-dev libgumbo-dev \
            libjbig2dec0-dev libopenjp2-7-dev \
            libbrotli-dev libbz2-dev
        ;;
    arch|manjaro|endeavouros|garuda)
        ok "Using pacman"
        sudo pacman -S --needed --noconfirm \
            base-devel cmake pkgconf git \
            mupdf-tools qpdf zlib libpng libjpeg-turbo \
            freetype2 harfbuzz mujs gumbo jbig2dec openjpeg2 brotli
        ;;
    fedora|rhel|centos|rocky|almalinux)
        ok "Using dnf"
        sudo dnf install -y \
            gcc-c++ cmake pkg-config git \
            mupdf-devel qpdf-devel zlib-devel \
            libpng-devel libjpeg-turbo-devel \
            freetype-devel harfbuzz-devel \
            jbig2dec-devel openjpeg2-devel brotli-devel
        ;;
    *)
        warn "Unknown distro '$DISTRO' — skipping automatic dep install"
        warn "Please install manually: cmake g++ libmupdf-dev libqpdf-dev"
        warn "and all their static dependencies, then re-run this script."
        ;;
    esac
}

step "Installing system build dependencies"
install_deps
ok "System dependencies installed"

# ═══════════════════════════════════════════════════════════════════════
# STEP 2: Install Go if missing or too old
# ═══════════════════════════════════════════════════════════════════════
step "Checking Go toolchain"

GO_REQUIRED="1.22"
GO_INSTALL_VERSION="1.22.3"

need_go=false

if ! command -v go >/dev/null 2>&1; then
    ok "Go not found — will install"
    need_go=true
else
    GO_CURRENT=$(go version | grep -oP '\d+\.\d+' | head -1)
    MAJOR=$(echo "$GO_CURRENT" | cut -d. -f1)
    MINOR=$(echo "$GO_CURRENT" | cut -d. -f2)
    REQ_MINOR=$(echo "$GO_REQUIRED" | cut -d. -f2)
    if [ "$MAJOR" -lt 1 ] || { [ "$MAJOR" -eq 1 ] && [ "$MINOR" -lt "$REQ_MINOR" ]; }; then
        warn "Go $GO_CURRENT found but $GO_REQUIRED+ required — upgrading"
        need_go=true
    else
        ok "Go $GO_CURRENT ✓"
    fi
fi

if $need_go; then
    step "Installing Go $GO_INSTALL_VERSION"
    ARCH=$(uname -m)
    case "$ARCH" in
        x86_64)  GO_ARCH="amd64" ;;
        aarch64) GO_ARCH="arm64" ;;
        armv7l)  GO_ARCH="armv6l" ;;
        *)       fail "Unsupported CPU architecture: $ARCH" ;;
    esac

    GO_URL="https://go.dev/dl/go${GO_INSTALL_VERSION}.linux-${GO_ARCH}.tar.gz"
    GO_TAR="/tmp/go${GO_INSTALL_VERSION}.tar.gz"

    info "Downloading Go from $GO_URL"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$GO_URL" -o "$GO_TAR"
    else
        wget -q "$GO_URL" -O "$GO_TAR"
    fi

    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf "$GO_TAR"
    rm -f "$GO_TAR"

    # Add to PATH for this session
    export PATH=$PATH:/usr/local/go/bin

    # Add to shell profile persistently
    SHELL_RC="$HOME/.bashrc"
    [ -n "${ZSH_VERSION:-}" ] && SHELL_RC="$HOME/.zshrc"
    if ! grep -q '/usr/local/go/bin' "$SHELL_RC" 2>/dev/null; then
        echo '' >> "$SHELL_RC"
        echo '# Go toolchain' >> "$SHELL_RC"
        echo 'export PATH=$PATH:/usr/local/go/bin' >> "$SHELL_RC"
        echo 'export PATH=$PATH:$HOME/go/bin' >> "$SHELL_RC"
    fi

    ok "Go $(go version | grep -oP 'go\d+\.\d+\.\d+') installed at /usr/local/go"
fi

# ═══════════════════════════════════════════════════════════════════════
# STEP 3: Fetch Go module dependencies
# ═══════════════════════════════════════════════════════════════════════
step "Fetching Go module dependencies"
cd "$SCRIPT_DIR/go"
go mod tidy
ok "Go modules ready"
cd "$SCRIPT_DIR"

# ═══════════════════════════════════════════════════════════════════════
# STEP 4: Build
# ═══════════════════════════════════════════════════════════════════════
step "Building PDFMaster"
make all
ok "Build complete"

# ═══════════════════════════════════════════════════════════════════════
# STEP 5: Install
# ═══════════════════════════════════════════════════════════════════════
step "Installing to ~/.local/bin"

mkdir -p "$HOME/.local/bin"
cp "$SCRIPT_DIR/build/pdfmaster" "$HOME/.local/bin/pdfmaster"
chmod +x "$HOME/.local/bin/pdfmaster"
ok "Installed: ~/.local/bin/pdfmaster"

# ── .desktop entry ────────────────────────────────────────────────────
DESKTOP_DIR="$HOME/.local/share/applications"
mkdir -p "$DESKTOP_DIR"
cat > "$DESKTOP_DIR/pdfmaster.desktop" << DESKTOP
[Desktop Entry]
Version=1.0
Type=Application
Name=PDFMaster
GenericName=PDF Tool
Comment=View, Merge, Compress and Split PDF files
Exec=$HOME/.local/bin/pdfmaster %F
Icon=pdfmaster
Terminal=true
Categories=Office;Viewer;
MimeType=application/pdf;
StartupNotify=true
Keywords=PDF;merge;compress;split;
DESKTOP
ok ".desktop entry created"

# Update desktop database if available
update-desktop-database "$DESKTOP_DIR" 2>/dev/null || true

# ── PATH check ────────────────────────────────────────────────────────
SHELL_RC="$HOME/.bashrc"
[ -n "${ZSH_VERSION:-}" ] && SHELL_RC="$HOME/.zshrc"

if ! echo "$PATH" | grep -q "$HOME/.local/bin"; then
    echo '' >> "$SHELL_RC"
    echo '# PDFMaster / user-local binaries' >> "$SHELL_RC"
    echo 'export PATH=$PATH:$HOME/.local/bin' >> "$SHELL_RC"
    warn "Added ~/.local/bin to PATH in $SHELL_RC"
    info "Run: source $SHELL_RC   (or open a new terminal)"
fi

# ═══════════════════════════════════════════════════════════════════════
# DONE
# ═══════════════════════════════════════════════════════════════════════
echo ""
echo -e "${GREEN}${BOLD}╔═══════════════════════════════════════════╗${NC}"
echo -e "${GREEN}${BOLD}║   PDFMaster installed successfully! ✓     ║${NC}"
echo -e "${GREEN}${BOLD}╚═══════════════════════════════════════════╝${NC}"
echo ""
echo "  Usage:"
echo "    pdfmaster --help"
echo "    pdfmaster info    document.pdf"
echo "    pdfmaster merge   -o out.pdf a.pdf b.pdf"
echo "    pdfmaster compress -l medium -o out.pdf in.pdf"
echo "    pdfmaster split   --mode pages -o ./pages/ in.pdf"
echo ""
echo "  If 'pdfmaster' is not found, run:"
echo "    source $SHELL_RC"
echo "  or open a new terminal."
echo ""
