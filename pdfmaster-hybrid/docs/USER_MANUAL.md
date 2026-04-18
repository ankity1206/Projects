# PDFMaster Hybrid — User Manual

**Version 1.0.0 · Go Orchestrator + C++17 Engine · MuPDF · QPDF**

---

## Table of Contents

1. [Overview](#1-overview)
2. [System Requirements](#2-system-requirements)
3. [Installation](#3-installation)
   - 3.1 [Linux — Quick Install](#31-linux--quick-install)
   - 3.2 [Windows — Build from Source](#32-windows--build-from-source)
   - 3.3 [Using vcpkg](#33-using-vcpkg)
4. [CLI Reference](#4-cli-reference)
   - 4.1 [info](#41-info)
   - 4.2 [view](#42-view)
   - 4.3 [text](#43-text)
   - 4.4 [merge](#44-merge)
   - 4.5 [compress](#45-compress)
   - 4.6 [split](#46-split)
5. [Configuration File](#5-configuration-file)
6. [Batch Processing](#6-batch-processing)
7. [Performance Reference](#7-performance-reference)
8. [Troubleshooting](#8-troubleshooting)
9. [Architecture Summary](#9-architecture-summary)
10. [License & Credits](#10-license--credits)

---

## 1. Overview

PDFMaster is a **standalone offline CLI** for PDF processing.
It uses a **Go + C++ hybrid architecture**:

| Layer      | Language | Role                                         |
|------------|----------|----------------------------------------------|
| CLI        | Go       | Argument parsing, validation, progress bars  |
| Operations | Go       | File I/O, error handling, goroutine dispatch |
| Engine     | C++17    | MuPDF rendering, QPDF merge/compress/split   |

**Zero runtime dependencies** — no Python, no JVM, no Electron.
One binary. Fully offline. Works on Linux and Windows.

---

## 2. System Requirements

### Runtime

| Platform | Requirements                                     |
|----------|--------------------------------------------------|
| Linux    | glibc 2.31+, libmupdf.so, libqpdf.so, zlib      |
| Windows  | Windows 10 64-bit, MSVC 2019 runtime             |

> If you build with vcpkg static triplets (`x64-linux`, `x64-windows-static`),
> all dependencies are baked into the binary — no .so/.dll files needed.

### Build Tools

| Tool    | Version  |
|---------|----------|
| Go      | 1.22+    |
| GCC/G++ | 10+      |
| CMake   | 3.22+    |
| MuPDF   | 1.23+    |
| QPDF    | 11.0+    |

---

## 3. Installation

### 3.1 Linux — Quick Install

```bash
# 1. Install build dependencies
make deps-ubuntu     # Ubuntu/Debian
make deps-arch       # Arch Linux
make deps-fedora     # Fedora

# 2. Install Go 1.22+
wget https://go.dev/dl/go1.22.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.22.0.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# 3. Clone + build
git clone https://github.com/yourname/pdfmaster.git
cd pdfmaster
make all

# 4. Install to ~/.local/bin
make install

# 5. Verify
pdfmaster --version
```

### 3.2 Windows — Build from Source

```batch
rem Prerequisites: Visual Studio 2022, Go 1.22, CMake 3.22
rem Set Qt6 path if building the Qt GUI as well

set PATH=%PATH%;C:\Go\bin
git clone https://github.com/yourname/pdfmaster.git
cd pdfmaster
scripts\build.bat
```

The batch script auto-detects Visual Studio, runs CMake for the engine,
then calls `go build` with the correct `CGO_CFLAGS`/`CGO_LDFLAGS`.

### 3.3 Using vcpkg

```bash
# Install vcpkg
git clone https://github.com/microsoft/vcpkg.git ~/vcpkg
~/vcpkg/bootstrap-vcpkg.sh
export VCPKG_ROOT=~/vcpkg

# Install PDF libs with static linking
~/vcpkg/vcpkg install mupdf qpdf zlib libjpeg-turbo --triplet x64-linux

# Build (vcpkg toolchain handles all library paths automatically)
make all VCPKG_ROOT=~/vcpkg
```

---

## 4. CLI Reference

### Global flags

| Flag          | Description                         |
|---------------|-------------------------------------|
| `--help`      | Show help for any command           |
| `--version`   | Print version and engine info       |
| `-v, --verbose` | Verbose output                    |

---

### 4.1 `info`

Show metadata for a PDF file.

```
pdfmaster info <file.pdf>
```

**Output includes:**
- File size, PDF version, page count
- Encryption status
- Title, Author, Subject, Creator, Producer
- Creation and modification dates

**Example:**

```
$ pdfmaster info report.pdf

  ▸ Document Information
  ──────────────────────────────────────────────
  File                report.pdf
  Size                4.2 MB
  PDF Version         1.7
  Pages               42
  Encrypted           No
  ──────────────────────────────────────────────
  Title               Q3 Financial Report
  Author              Jane Smith
  Creator             LibreOffice Writer
  Producer            LibreOffice 7.5
  Created             D:20240315120000Z
```

---

### 4.2 `view`

Render a PDF page to a PNG image.

```
pdfmaster view [flags] <input.pdf>
```

| Flag              | Default                       | Description                     |
|-------------------|-------------------------------|---------------------------------|
| `--page N`        | 1                             | Page to render (1-based)        |
| `--dpi F`         | 150                           | Render resolution               |
| `--rotate N`      | 0                             | Extra rotation (0/90/180/270)   |
| `-o, --output`    | `<name>_page<N>.png`          | Output PNG path                 |

**DPI guide:**

| DPI | Use case                         |
|-----|----------------------------------|
| 72  | Screen preview (fast)            |
| 96  | Standard screen                  |
| 150 | Quality preview (default)        |
| 300 | Print-ready output               |

**Examples:**

```bash
pdfmaster view document.pdf                         # page 1 at 150 DPI
pdfmaster view --page 5 --dpi 300 -o p5.png doc.pdf
pdfmaster view --page 2 --rotate 90 doc.pdf
```

---

### 4.3 `text`

Extract text from a PDF.

```
pdfmaster text [flags] <input.pdf>
```

| Flag            | Default   | Description                           |
|-----------------|-----------|---------------------------------------|
| `--page N`      | 0 (all)   | Single page to extract (1-based)      |
| `-o, --output`  | stdout    | Output text file path                 |

Pages are separated by form-feed characters (`\f`) in the output.

> **Note:** Works only on PDFs with embedded text. For scanned
> (image-only) PDFs, you must run an OCR tool (Tesseract, etc.) first.

**Examples:**

```bash
pdfmaster text document.pdf                    # print all text to stdout
pdfmaster text -o content.txt document.pdf     # save to file
pdfmaster text --page 3 document.pdf           # page 3 only
```

---

### 4.4 `merge`

Combine multiple PDFs into one.

```
pdfmaster merge -o <output.pdf> <file1.pdf> <file2.pdf> [...]
```

| Flag            | Default | Description                                  |
|-----------------|---------|----------------------------------------------|
| `-o, --output`  | —       | Output PDF path **(required)**               |
| `--range STR`   | —       | Page range for each file (repeat per file)   |
| `--linearize`   | false   | Web-optimise output (for online viewing)     |

**Page range format:**  `"N-M"`, `"N-"`, `"-M"`, `"-"` (all pages)

**Examples:**

```bash
# Merge three full files
pdfmaster merge -o merged.pdf a.pdf b.pdf c.pdf

# Use pages 1-5 from a.pdf and pages 2-10 from b.pdf
pdfmaster merge -o out.pdf --range 1-5 --range 2-10 a.pdf b.pdf

# Use last section of a.pdf (page 8 onwards)
pdfmaster merge -o out.pdf --range 8- --range - a.pdf b.pdf

# Linearize for web hosting
pdfmaster merge -o web.pdf --linearize a.pdf b.pdf c.pdf
```

**Performance:** Merge is structural — no pixel decoding. Throughput is
~50–200 MB/s depending on object density, typically 1–3 seconds per
100 MB of combined input.

---

### 4.5 `compress`

Reduce PDF file size.

```
pdfmaster compress [flags] <input.pdf>
```

| Flag                   | Default | Description                                    |
|------------------------|---------|------------------------------------------------|
| `-o, --output`         | `<n>_compressed.pdf` | Output path                 |
| `-l, --level N`        | 2       | 1=light, 2=medium, 3=heavy                     |
| `--remove-metadata`    | false   | Strip title, author, dates from /Info          |
| `--remove-js`          | true    | Remove embedded JavaScript                     |
| `--remove-annotations` | false   | Remove all comments and highlights             |
| `--jpeg-quality N`     | 72      | JPEG re-encode quality for Heavy (1-100)       |
| `--max-image-dpi N`    | 150     | Downsample images above this DPI (Heavy only)  |

**Level guide:**

| Level | What happens                          | Typical saving | Speed   |
|-------|---------------------------------------|----------------|---------|
| 1     | Object-stream compaction              | 10–25%         | Fast    |
| 2     | + zlib-9 recompression of all streams | 25–45%         | Medium  |
| 3     | + image downsampling + normalisation  | 40–70%         | Slower  |

**Examples:**

```bash
# Default medium compression
pdfmaster compress -o small.pdf large.pdf

# Aggressive: strip metadata + heavy compress
pdfmaster compress -l 3 --remove-metadata --jpeg-quality 60 \
    -o tiny.pdf large.pdf

# Light pass for already-compressed files
pdfmaster compress -l 1 -o out.pdf input.pdf
```

**Output summary:**
```
✓ Compressing — 4.2 MB → 1.8 MB (saved 2.4 MB, 57.1%) (1 240 ms)

  Input  : large.pdf (4.2 MB)
  Output : large_compressed.pdf (1.8 MB)
  Saved  : 2.4 MB (57.1%)
  Pages  : 42
  Time   : 1.2 s
```

---

### 4.6 `split`

Split a PDF into separate files.

```
pdfmaster split [flags] <input.pdf>
```

| Flag              | Default         | Description                           |
|-------------------|-----------------|---------------------------------------|
| `-o, --output`    | input directory | Output directory                      |
| `--mode MODE`     | pages           | `pages` \| `range` \| `chunks`        |
| `--from N`        | 1               | Start page for range mode (1-based)   |
| `--to N`          | last            | End page for range mode               |
| `--chunk N`       | 1               | Pages per chunk (chunks mode)         |
| `--template STR`  | see below       | Output filename template              |

**Modes:**

| Mode     | Description                                           |
|----------|-------------------------------------------------------|
| `pages`  | One file per page: `doc_0001-0001.pdf`, `0002-0002.pdf`... |
| `range`  | Extract pages `--from` to `--to` into one file        |
| `chunks` | Split into sequential groups of `--chunk` pages       |

**Template tokens:**
`{name}`, `{from}`, `{to}`, `{n}`, `{from:04d}`, `{to:04d}`, `{n:04d}`

**Examples:**

```bash
# One page per file
pdfmaster split input.pdf

# Extract pages 5-12 into one file
pdfmaster split --mode range --from 5 --to 12 -o ./out/ input.pdf

# Split into 10-page chunks
pdfmaster split --mode chunks --chunk 10 -o ./parts/ input.pdf

# Custom naming
pdfmaster split --template "chapter_{n:02d}.pdf" input.pdf

# Split into /tmp with verbose output
pdfmaster split -v --mode pages -o /tmp/pages/ input.pdf
```

---

## 5. Configuration File

PDFMaster reads defaults from `~/.config/pdfmaster/config.toml`.

Create or edit it:

```bash
mkdir -p ~/.config/pdfmaster
cat > ~/.config/pdfmaster/config.toml << 'EOF'
# PDFMaster configuration

default_compress_level = 2      # 1=light 2=medium 3=heavy
default_dpi            = 150    # render DPI
default_jpeg_quality   = 72     # 1-100 (heavy compress mode)
default_max_image_dpi  = 150    # downsample threshold
progress_color         = true   # coloured progress bars
temp_dir               = ""     # scratch directory (empty = system temp)
EOF
```

CLI flags always override config file values.

---

## 6. Batch Processing

### Compress all PDFs in a directory

```bash
# Bash loop
for f in /path/to/docs/*.pdf; do
    pdfmaster compress -l 2 -o "${f%.pdf}_small.pdf" "$f"
done
```

### Merge all PDFs matching a pattern

```bash
pdfmaster merge -o combined.pdf chapter_*.pdf
```

### Extract text from all PDFs (parallel with GNU parallel)

```bash
ls *.pdf | parallel pdfmaster text -o {.}.txt {}
```

### Split + re-merge (extract specific chapters)

```bash
# Extract pages 10-20 from each of 5 files, merge into one
for i in 1 2 3 4 5; do
    pdfmaster split --mode range --from 10 --to 20 \
        -o /tmp/ part${i}.pdf
done
pdfmaster merge -o extracted.pdf /tmp/part*_pages_10-20.pdf
```

### Programmatic use (Go)

If you embed PDFMaster as a library in your own Go program:

```go
import (
    "github.com/yourname/pdfmaster/internal/bridge"
    "github.com/yourname/pdfmaster/internal/ops"
)

func main() {
    bridge.Init()
    defer bridge.Shutdown()

    result, err := ops.Compress(ops.CompressOptions{
        InputPath:  "input.pdf",
        OutputPath: "output.pdf",
        Level:      bridge.CompressMedium,
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Saved %.1f%%\n", result.SavingsPct)
}
```

---

## 7. Performance Reference

### Rendering (MuPDF)

| Page type        | 96 DPI   | 150 DPI  | 300 DPI  |
|------------------|----------|----------|----------|
| Text-heavy       | 5–15 ms  | 10–40 ms | 30–100 ms|
| Mixed content    | 15–50 ms | 40–100ms | 100–300ms|
| Scanned image    | 3–10 ms  | 8–20 ms  | 20–60 ms |

### Merge (QPDF object graph copy)

| Total input size | Time     |
|------------------|----------|
| 10 MB            | < 0.5 s  |
| 100 MB           | 1–3 s    |
| 500 MB           | 5–15 s   |

### Compression (QPDF + zlib-9)

| Level  | 10 MB  | 100 MB |
|--------|--------|--------|
| Light  | < 0.2s | 0.5–1s |
| Medium | 0.5–1s | 3–8 s  |
| Heavy  | 1–3 s  | 8–25 s |

---

## 8. Troubleshooting

### `cgo: C compiler not found`
```bash
sudo apt install build-essential   # Ubuntu
sudo pacman -S base-devel          # Arch
```

### `cannot find -lpdfengine`
The engine library must be built first:
```bash
make engine
# then retry
make go
```

### `mupdf/fitz.h: No such file or directory`
```bash
sudo apt install libmupdf-dev
# or set:
export MUPDF_ROOT=/path/to/mupdf
make engine
```

### `libqpdf not found`
```bash
sudo apt install libqpdf-dev
```

### `merge failed: engine error -9: Encrypted PDF`
```bash
# Decrypt first using the QPDF CLI tool:
qpdf --password=SECRET --decrypt encrypted.pdf decrypted.pdf
pdfmaster merge -o out.pdf decrypted.pdf other.pdf
```

### Progress bar garbled in terminal
Set `progress_color = false` in `~/.config/pdfmaster/config.toml`
or pipe output through `cat`:
```bash
pdfmaster compress input.pdf | cat
```

### Binary too large
The Go binary includes the Go runtime (~5 MB). Build with:
```bash
make go BUILD_TYPE=Release   # strips debug symbols with -ldflags="-s -w"
```

---

## 9. Architecture Summary

```
Go CLI (cobra)  →  ops layer  →  bridge (cgo)  →  C ABI  →  C++ engine
    │                                                           │
    └── progress bars                              MuPDF + QPDF + zlib
        (bubbletea)
```

The C ABI boundary (`pdfengine.h`) contains only:
- `int` error codes
- `const char*` strings (UTF-8)
- `void*` opaque handles
- POD structs (`pm_doc_info_t`, `pm_render_result_t`, ...)
- C function pointer for progress callbacks

No C++ types, no exceptions, no templates ever cross this line.
See [ARCHITECTURE.md](ARCHITECTURE.md) for full details.

---

## 10. License & Credits

PDFMaster is released under the **MIT License**.

| Dependency    | License              | Purpose                |
|---------------|----------------------|------------------------|
| MuPDF         | AGPL v3 / Commercial | PDF rendering, text    |
| QPDF          | Apache 2.0           | Merge, compress, split |
| Go stdlib     | BSD 3-Clause         | Core runtime           |
| Cobra         | Apache 2.0           | CLI framework          |
| Bubble Tea    | MIT                  | Terminal UI / progress |
| Lip Gloss     | MIT                  | Terminal styling       |
| zlib          | zlib License         | Stream compression     |

> **MuPDF note:** For commercial / closed-source distribution you need
> a commercial MuPDF license from Artifex Software, Inc.
> QPDF (Apache 2.0) and all Go dependencies are permissive.
