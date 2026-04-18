# PDFMaster Hybrid — Go + C++ Engine

> **Engine & Wrapper pattern**: Go orchestrates, C++ computes.
> View · Merge · Compress · Split — one native binary, zero runtime deps.

```
┌─────────────────────────────────────────────────────────────────┐
│                        Go Orchestrator                          │
│                                                                 │
│  cmd/pdfmaster/         ← Cobra CLI entry point                │
│  internal/ops/          ← merge.go, compress.go, split.go      │
│  internal/progress/     ← terminal progress bars (bubbletea)   │
│  internal/config/       ← flags, profiles, user config         │
│  internal/bridge/       ← cgo bridge package                   │
│       │                                                         │
│       │  cgo  (C ABI only — no C++ types cross this line)      │
│       ▼                                                         │
├─────────────────────────────────────────────────────────────────┤
│                    C Bridge Layer                               │
│  engine/src/bridge/pdfengine_c.cpp   ← extern "C" exports      │
│  engine/include/pdfengine.h          ← C ABI header            │
├─────────────────────────────────────────────────────────────────┤
│                    C++ Engine                                   │
│  engine/src/core/PdfDocument.cpp     ← MuPDF wrapper           │
│  engine/src/core/PdfMerger.cpp       ← QPDF merge              │
│  engine/src/core/PdfCompressor.cpp   ← QPDF + zlib compress    │
│  engine/src/core/PdfSplitter.cpp     ← QPDF split              │
│                                                                 │
│        MuPDF (fz_context)    QPDF (QPDFWriter)                 │
└─────────────────────────────────────────────────────────────────┘
```

## Quick Build

```bash
# 1. Install deps (Ubuntu/Debian)
sudo apt install g++ cmake ninja-build libmupdf-dev libqpdf-dev zlib1g-dev

# 2. Install Go 1.22+
wget https://go.dev/dl/go1.22.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.22.0.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# 3. Build everything
make all

# 4. Run
./dist/pdfmaster --help
```

## Usage

```bash
pdfmaster view    document.pdf
pdfmaster merge   -o merged.pdf a.pdf b.pdf c.pdf
pdfmaster compress -l medium -o out.pdf input.pdf
pdfmaster split   -o ./pages/ --mode pages input.pdf
pdfmaster info    document.pdf
```

## Architecture Details

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) and
[docs/USER_MANUAL.md](docs/USER_MANUAL.md).
