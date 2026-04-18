#!/usr/bin/env bash
# packaging/linux/build_appimage.sh
#
# Builds PDFMaster-1.0.0-x86_64.AppImage
#
# What this script does:
#   1. Builds the C++ engine (libpdfengine.a) with CMake
#   2. Builds the main pdfmaster binary (Go + cgo)
#   3. Builds the launcher binary (pure Go, no cgo)
#   4. Assembles an AppDir with all bundled .so files
#   5. Runs appimagetool to create the final .AppImage
#
# The resulting file is completely self-contained:
#   - Bundles libmupdf, libqpdf, libz, libpng, libjpeg-turbo
#   - Bundles Qt6 runtime libs
#   - No apt, no sudo, no internet required on the end-user machine
#
# Requirements (build machine only):
#   apt install build-essential cmake ninja-build libmupdf-dev libqpdf-dev
#   apt install qt6-base-dev zlib1g-dev libpng-dev libjpeg-turbo8-dev
#   go 1.22+ in PATH
#   curl or wget (to download appimagetool if not present)

set -euo pipefail

# ── Configuration ─────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
VERSION="1.0.0"
ARCH="$(uname -m)"
OUTPUT_DIR="$ROOT/dist"
APPDIR="$ROOT/build/AppDir"
BUILD_DIR="$ROOT/build"

APPIMAGE_TOOL="$BUILD_DIR/appimagetool-x86_64.AppImage"
APPIMAGE_TOOL_URL="https://github.com/AppImage/AppImageKit/releases/download/continuous/appimagetool-x86_64.AppImage"

OUTPUT_APPIMAGE="$OUTPUT_DIR/PDFMaster-${VERSION}-${ARCH}.AppImage"

# ── Colours ───────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
BLUE='\033[0;34m'; BOLD='\033[1m'; NC='\033[0m'

step()  { echo -e "\n${BLUE}${BOLD}▸ $*${NC}"; }
ok()    { echo -e "${GREEN}  ✓ $*${NC}"; }
warn()  { echo -e "${YELLOW}  ⚠ $*${NC}"; }
error() { echo -e "${RED}  ✗ $*${NC}" >&2; exit 1; }

NPROC=$(nproc 2>/dev/null || echo 4)

# ── Preflight checks ─────────────────────────────────────────────────
step "Preflight checks"

check_cmd() { command -v "$1" >/dev/null || error "Required: $1 (install with: $2)"; }
check_cmd cmake  "apt install cmake"
check_cmd g++    "apt install build-essential"
check_cmd go     "see https://go.dev/dl/"
check_cmd pkg-config "apt install pkg-config"

# Check PDF libs
pkg-config --exists mupdf   || error "libmupdf-dev not found (apt install libmupdf-dev)"
pkg-config --exists libqpdf || error "libqpdf-dev not found (apt install libqpdf-dev)"

ok "All build dependencies present"
ok "Go: $(go version)"
ok "MuPDF: $(pkg-config --modversion mupdf 2>/dev/null || echo 'found')"
ok "QPDF:  $(pkg-config --modversion libqpdf 2>/dev/null || echo 'found')"

# ── Build C++ engine ──────────────────────────────────────────────────
step "Building C++ engine (libpdfengine.a)"

ENGINE_BUILD="$BUILD_DIR/engine-release"
mkdir -p "$ENGINE_BUILD"

cmake -B "$ENGINE_BUILD" -S "$ROOT/engine" \
    -G Ninja \
    -DCMAKE_BUILD_TYPE=Release \
    -DCMAKE_POSITION_INDEPENDENT_CODE=ON \
    -DENGINE_USE_SYSTEM_MUPDF=ON \
    -DENGINE_USE_SYSTEM_QPDF=ON \
    -DENGINE_ENABLE_LTO=ON \
    --log-level=WARNING

cmake --build "$ENGINE_BUILD" --parallel "$NPROC"
ok "Engine built: $ENGINE_BUILD/lib/libpdfengine.a"

# ── Build main pdfmaster binary ───────────────────────────────────────
step "Building pdfmaster (Go + CGo)"

export CGO_ENABLED=1
export CGO_CFLAGS="-I$ROOT/engine/include"
export CGO_LDFLAGS="-L$ENGINE_BUILD/lib -lpdfengine -lstdc++ -lm -lpthread -ldl -lz"

cd "$ROOT/go"
go build \
    -ldflags="-s -w \
        -X main.buildVersion=$VERSION \
        -X main.buildDate=$(date -u +%Y-%m-%d) \
        -X main.buildCommit=$(git rev-parse --short HEAD 2>/dev/null || echo unknown)" \
    -o "$BUILD_DIR/pdfmaster" \
    ./cmd/pdfmaster
ok "pdfmaster binary: $(du -sh "$BUILD_DIR/pdfmaster" | cut -f1)"

# ── Build launcher ────────────────────────────────────────────────────
step "Building launcher (pure Go)"

cd "$ROOT/launcher"
CGO_ENABLED=0 go build \
    -ldflags="-s -w -X main.AppVersion=$VERSION" \
    -o "$BUILD_DIR/pdfmaster-launcher" \
    ./cmd/launcher
ok "Launcher binary: $(du -sh "$BUILD_DIR/pdfmaster-launcher" | cut -f1)"

# ── Assemble AppDir ───────────────────────────────────────────────────
step "Assembling AppDir structure"

rm -rf "$APPDIR"
mkdir -p \
    "$APPDIR/usr/bin" \
    "$APPDIR/usr/lib" \
    "$APPDIR/usr/share/applications" \
    "$APPDIR/usr/share/icons/hicolor/256x256/apps" \
    "$APPDIR/usr/share/icons/hicolor/scalable/apps"

# Binaries
cp "$BUILD_DIR/pdfmaster"          "$APPDIR/usr/bin/"
cp "$BUILD_DIR/pdfmaster-launcher" "$APPDIR/usr/bin/"
chmod +x "$APPDIR/usr/bin/"*

# AppRun
cp "$SCRIPT_DIR/appimage/AppRun"   "$APPDIR/AppRun"
chmod +x "$APPDIR/AppRun"

# Desktop entry
cp "$SCRIPT_DIR/appimage/pdfmaster.desktop" "$APPDIR/"
cp "$SCRIPT_DIR/appimage/pdfmaster.desktop" "$APPDIR/usr/share/applications/"

# Icons (use SVG from resources if PNG not available)
SVG_ICON="$ROOT/engine/resources/icons/pdfmaster.svg"
if [ -f "$SVG_ICON" ]; then
    cp "$SVG_ICON" "$APPDIR/usr/share/icons/hicolor/scalable/apps/pdfmaster.svg"
    cp "$SVG_ICON" "$APPDIR/pdfmaster.svg"
    # Convert to PNG if rsvg-convert or inkscape is available
    if command -v rsvg-convert >/dev/null 2>&1; then
        rsvg-convert -w 256 -h 256 "$SVG_ICON" \
            -o "$APPDIR/usr/share/icons/hicolor/256x256/apps/pdfmaster.png"
        cp "$APPDIR/usr/share/icons/hicolor/256x256/apps/pdfmaster.png" \
            "$APPDIR/pdfmaster.png"
        ok "Icon converted to PNG"
    else
        warn "rsvg-convert not found — using SVG icon only"
        cp "$SVG_ICON" "$APPDIR/pdfmaster.png"  # AppImage fallback
    fi
fi

ok "AppDir skeleton assembled"

# ── Bundle shared libraries ───────────────────────────────────────────
step "Bundling shared libraries"

bundle_lib() {
    local libname="$1"
    local found
    # Search common paths
    for dir in /usr/lib/x86_64-linux-gnu /usr/lib /usr/local/lib /lib/x86_64-linux-gnu /lib64; do
        if [ -f "$dir/$libname" ]; then
            found="$dir/$libname"
            break
        fi
        # Also check for versioned symlinks
        for f in "$dir"/${libname}*; do
            if [ -f "$f" ]; then found="$f"; break 2; fi
        done
    done
    if [ -n "$found" ]; then
        cp -L "$found" "$APPDIR/usr/lib/"
        ok "Bundled: $libname"
    else
        warn "Not found (will rely on system): $libname"
    fi
}

# Core PDF engine libraries
bundle_lib "libmupdf.so.3"
bundle_lib "libmupdf.so"
bundle_lib "libqpdf.so.29"
bundle_lib "libqpdf.so"
bundle_lib "libz.so.1"
bundle_lib "libpng16.so.16"
bundle_lib "libjpeg.so.8"
bundle_lib "libjpeg-turbo.so"
bundle_lib "libopenjp2.so.7"
bundle_lib "libfreetype.so.6"
bundle_lib "libharfbuzz.so.0"
bundle_lib "libglib-2.0.so.0"

# Use linuxdeploy or ldd to auto-collect remaining deps if available
PDFMASTER_BIN="$APPDIR/usr/bin/pdfmaster"
if command -v ldd >/dev/null 2>&1; then
    step "Auto-collecting remaining dynamic dependencies via ldd"
    ldd "$PDFMASTER_BIN" 2>/dev/null | grep "=>" | awk '{print $3}' | while read -r lib; do
        [ -f "$lib" ] || continue
        libname=$(basename "$lib")
        # Skip glibc / libstdc++ (we rely on system compatibility)
        case "$libname" in
            libc.so*|libm.so*|libdl.so*|libpthread.so*|\
            ld-linux*|libstdc++*|libgcc_s*|librt.so*)
                continue ;;
        esac
        if [ ! -f "$APPDIR/usr/lib/$libname" ]; then
            cp -L "$lib" "$APPDIR/usr/lib/" 2>/dev/null && \
                echo "  auto: $libname" || true
        fi
    done
fi

# ── Compute checksums ──────────────────────────────────────────────────
step "Computing integrity checksums"

CHECKSUMS_JSON="$APPDIR/usr/share/pdfmaster/checksums.json"
mkdir -p "$(dirname "$CHECKSUMS_JSON")"

python3 - <<'PYEOF'
import os, hashlib, json

appdir = os.environ.get("APPDIR_PATH")
if not appdir:
    import sys; sys.exit(0)

files = {}
for root, dirs, filenames in os.walk(os.path.join(appdir, "usr", "bin")):
    for fn in filenames:
        path = os.path.join(root, fn)
        rel  = os.path.relpath(path, appdir)
        h = hashlib.sha256()
        with open(path, "rb") as f:
            for chunk in iter(lambda: f.read(65536), b""):
                h.update(chunk)
        files[rel] = h.hexdigest()

with open(os.path.join(appdir, "usr", "share", "pdfmaster", "checksums.json"), "w") as f:
    json.dump({"files": files}, f, indent=2)
print(f"  Checksums written for {len(files)} files")
PYEOF

export APPDIR_PATH="$APPDIR"
python3 - <<'PYEOF'
import os, hashlib, json

appdir = os.environ["APPDIR_PATH"]
files = {}
for root, dirs, filenames in os.walk(os.path.join(appdir, "usr", "bin")):
    for fn in filenames:
        path = os.path.join(root, fn)
        rel  = os.path.relpath(path, appdir)
        h = hashlib.sha256()
        with open(path, "rb") as f:
            for chunk in iter(lambda: f.read(65536), b""): h.update(chunk)
        files[rel] = h.hexdigest()
out = os.path.join(appdir, "usr", "share", "pdfmaster", "checksums.json")
os.makedirs(os.path.dirname(out), exist_ok=True)
with open(out, "w") as f: json.dump({"files": files}, f, indent=2)
print(f"  Written checksums for {len(files)} files")
PYEOF
ok "Checksums computed"

# ── Download appimagetool ─────────────────────────────────────────────
step "Preparing appimagetool"

if [ ! -x "$APPIMAGE_TOOL" ]; then
    echo "  Downloading appimagetool..."
    curl -fsSL "$APPIMAGE_TOOL_URL" -o "$APPIMAGE_TOOL"
    chmod +x "$APPIMAGE_TOOL"
    ok "appimagetool downloaded"
else
    ok "appimagetool already present"
fi

# ── Create AppImage ───────────────────────────────────────────────────
step "Creating AppImage"

mkdir -p "$OUTPUT_DIR"

# ARCH must be set for appimagetool
export ARCH="$ARCH"

"$APPIMAGE_TOOL" \
    --no-appstream \
    "$APPDIR" \
    "$OUTPUT_APPIMAGE" \
    2>&1 | grep -v "^$" | sed 's/^/  /'

chmod +x "$OUTPUT_APPIMAGE"

# ── Final report ─────────────────────────────────────────────────────
SIZE=$(du -sh "$OUTPUT_APPIMAGE" | cut -f1)

echo ""
echo -e "${GREEN}${BOLD}╔══════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}${BOLD}║  AppImage built successfully                     ║${NC}"
echo -e "${GREEN}${BOLD}╚══════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "  File   : ${BOLD}$OUTPUT_APPIMAGE${NC}"
echo -e "  Size   : $SIZE"
echo ""
echo "  To install for this user:"
echo "    cp $OUTPUT_APPIMAGE ~/bin/PDFMaster"
echo "    chmod +x ~/bin/PDFMaster"
echo ""
echo "  To run directly:"
echo "    $OUTPUT_APPIMAGE --help"
echo ""
echo "  Or double-click in your file manager."
echo ""
