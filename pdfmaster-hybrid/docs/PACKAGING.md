# PDFMaster — Packaging & Distribution Guide

This document explains how the one-click distribution packages are built,
how the self-bootstrapping launcher works, and how to produce release
artifacts for Linux and Windows.

---

## What Users Get

| Platform | File                               | Size   | What happens on double-click          |
|----------|------------------------------------|--------|---------------------------------------|
| Linux    | `PDFMaster-1.0.0-x86_64.AppImage` | ~35 MB | Mounts squashfs, runs AppRun, launches|
| Windows  | `PDFMaster-Setup-1.0.0.exe`        | ~28 MB | NSIS installs to `%LOCALAPPDATA%`, makes shortcuts |
| Windows  | `PDFMaster-Portable-1.0.0.zip`     | ~28 MB | Extract anywhere, run `.exe`          |

**No terminal. No sudo. No apt. No pip. No admin rights.**
The user clicks once and PDFMaster opens.

---

## Architecture: One-Click Boot Sequence

```
USER DOUBLE-CLICKS
        │
        ▼
┌─────────────────────────────────────────────────────────┐
│  pdfmaster-launcher  (pure Go, no cgo, ~4 MB)           │
│                                                         │
│  1. Check ~/.local/share/pdfmaster/.health.json         │
│     └── EXISTS + VALID → skip to step 5 (< 50 ms)      │
│                                                         │
│  2. syscheck.Run()   → probe OS, libs, disk space       │
│                                                         │
│  3. setup.BuildPlan() → decide what to extract          │
│                                                         │
│  4. ui.RunSetupUI()  → Bubble Tea progress screen       │
│     ├── Create directories                              │
│     ├── Extract pdfmaster binary from payload           │
│     ├── Extract bundled .so / .dll files                │
│     ├── Write config defaults                           │
│     └── Create .desktop / Start Menu shortcut           │
│                                                         │
│  5. Set LD_LIBRARY_PATH to bundled lib dir              │
│                                                         │
│  6. syscall.Exec(pdfmaster, args, env)                  │
│     └── REPLACES launcher process — zero overhead       │
└─────────────────────────────────────────────────────────┘
        │
        ▼
  pdfmaster binary (Go + C++ engine)  ← runs as usual
```

**Subsequent launches:** Step 1 finds the health file valid → jumps
directly to step 5+6. Total overhead: ~30 ms.

---

## Linux: AppImage

### How AppImage works

An AppImage is an ISO 9660 / SquashFS image with an ELF runtime stub
prepended. When the user runs it:

1. The ELF stub extracts itself to a temp dir and mounts the SquashFS.
2. `$APPDIR` is set to the mount point.
3. Our `AppRun` script sets `LD_LIBRARY_PATH` to our bundled libs.
4. `AppRun` execs `pdfmaster-launcher`.

### AppDir structure

```
PDFMaster-1.0.0-x86_64.AppImage  (single file, ~35 MB)
└── (SquashFS mount)
    ├── AppRun                     ← entry point
    ├── pdfmaster.desktop
    ├── pdfmaster.png              ← 256×256 icon
    └── usr/
        ├── bin/
        │   ├── pdfmaster          ← main binary (Go + C++ engine)
        │   └── pdfmaster-launcher ← bootstrapper (pure Go)
        ├── lib/
        │   ├── libmupdf.so.3      ← bundled PDF libs
        │   ├── libqpdf.so.29
        │   ├── libz.so.1
        │   └── ...
        └── share/
            ├── applications/pdfmaster.desktop
            ├── icons/hicolor/256x256/apps/pdfmaster.png
            └── pdfmaster/checksums.json
```

### Building

```bash
# Build machine: Ubuntu 22.04+
sudo apt install build-essential cmake ninja-build \
    libmupdf-dev libqpdf-dev zlib1g-dev \
    qt6-base-dev rsvg-convert

chmod +x packaging/linux/build_appimage.sh
./packaging/linux/build_appimage.sh
# Output: dist/PDFMaster-1.0.0-x86_64.AppImage
```

### Installing (end user, no terminal needed)

```bash
# In file manager: right-click → Properties → Allow executing as program
# OR from terminal:
chmod +x PDFMaster-1.0.0-x86_64.AppImage
./PDFMaster-1.0.0-x86_64.AppImage
```

The AppImage also integrates with `appimaged` for automatic .desktop
entry creation and MIME type registration.

### glibc compatibility

We build on Ubuntu 22.04 (glibc 2.35) which runs on any distro with
glibc ≥ 2.35. For broader compatibility (RHEL 7, older LTS distros),
build on Ubuntu 20.04 (glibc 2.31) or use `zig cc` as the C compiler
for cross-compilation.

---

## Windows: NSIS Installer

### What the installer does

1. Extracts files to `%LOCALAPPDATA%\PDFMaster\`
   (no UAC, no `C:\Program Files`, no admin)
2. Creates Start Menu shortcuts
3. Optionally creates a Desktop shortcut
4. Registers "Open with PDFMaster" in the right-click context menu
5. Adds an entry to Add/Remove Programs (Programs & Features)
6. Creates a full uninstaller (`Uninstall.exe`)

### Building

```batch
rem Requirements: VS 2022, Go 1.22, CMake 3.22, NSIS 3.x
rem Using vcpkg for MuPDF + QPDF (recommended):

set VCPKG_ROOT=C:\vcpkg
packaging\windows\build_installer.bat

rem Output:
rem   dist\PDFMaster-Setup-1.0.0.exe
rem   dist\PDFMaster-Portable-1.0.0.zip
```

### Silent install (for enterprise/IT deployment)

```batch
PDFMaster-Setup-1.0.0.exe /S
```

Installs silently to `%LOCALAPPDATA%\PDFMaster\` with no prompts.

### Portable ZIP

Extract anywhere, run `pdfmaster-launcher.exe`. No install needed.
The launcher creates `%LOCALAPPDATA%\PDFMaster\config\` on first run
for storing preferences.

---

## The Self-Extracting Binary (Linux standalone)

For users who want a single file without the AppImage runtime overhead,
we also offer a self-extracting binary:

```
pdfmaster-self-extract  =  launcher ELF  +  [payload.tar.gz]  +  [magic]  +  [size]
```

Built with `ci/payload_packer.sh`:

```bash
# Build payload directory
mkdir -p build/payload/{bin,lib}
cp build/pdfmaster         build/payload/bin/
cp build/libmupdf.so.3     build/payload/lib/
cp build/libqpdf.so.29     build/payload/lib/

# Pack
./ci/payload_packer.sh \
    --launcher build/pdfmaster-launcher \
    --payload  build/payload \
    --output   dist/pdfmaster-self-extract
```

On first run the binary extracts to `~/.local/share/pdfmaster/` and
exec()s itself. Subsequent runs launch in ~30 ms.

---

## CI/CD with GitHub Actions

Trigger a release by pushing a version tag:

```bash
git tag v1.0.1
git push origin v1.0.1
```

The `.github/workflows/release.yml` workflow:
1. Runs on `ubuntu-22.04` → builds Linux AppImage
2. Runs on `windows-2022` → builds Windows installer + portable ZIP
3. Creates a GitHub Release with all artifacts attached
4. Uploads `SHA256SUMS.txt` for integrity verification

---

## Checksums and Signing

Every release includes:
- `SHA256SUMS.txt` — SHA-256 of each distribution file
- `SHA256SUMS.txt.asc` — GPG signature (if signing key configured)

To verify:
```bash
# Linux
sha256sum -c SHA256SUMS.txt

# macOS
shasum -a 256 -c SHA256SUMS.txt
```

---

## Size Budget

| Component                    | Compressed size |
|------------------------------|----------------|
| pdfmaster binary (Go + C++)  | ~12 MB         |
| pdfmaster-launcher (Go)      | ~4 MB          |
| libmupdf.so.3                | ~8 MB          |
| libqpdf.so.29                | ~3 MB          |
| libz + libpng + libjpeg      | ~2 MB          |
| Qt6 runtime (if bundled)     | ~6 MB          |
| **Total AppImage**           | **~35 MB**     |
| **Total Windows installer**  | **~28 MB**     |

---

## Reducing Binary Size Further

```bash
# 1. Strip debug symbols (already done in release build)
strip --strip-unneeded build/pdfmaster

# 2. UPX compression (tradeoff: slower startup)
upx --best --lzma build/pdfmaster

# 3. Use musl instead of glibc for fully static binary
# Build with: CC="zig cc -target x86_64-linux-musl" CGO_ENABLED=1 ...
```
