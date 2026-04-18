# PDFMaster Hybrid — Architecture Reference

## Overview: The Engine & Wrapper Pattern

PDFMaster separates concerns across a hard ABI boundary:

```
┌─────────────────────────────────────────────────────────────────────┐
│  Go Orchestrator  (goroutines · Cobra CLI · progress UI)            │
│                                                                     │
│   cmd/pdfmaster/         Cobra commands (info, view, merge, ...)    │
│   internal/ops/          Operation logic (validate, dispatch, fmt)  │
│   internal/progress/     Bubbletea progress bars                    │
│   internal/config/       User config (~/.config/pdfmaster/)         │
│   internal/bridge/       ← CGO BOUNDARY — the only import "C" pkg  │
│           │                                                         │
│           │  cgo  (C ABI only — pdfengine.h)                       │
│           ▼                                                         │
├─────────────────────────────────────────────────────────────────────┤
│  C Bridge Layer                                                     │
│   engine/src/bridge/pdfengine_c.cpp  extern "C" exports            │
│   engine/include/pdfengine.h         public C ABI header            │
│           │                                                         │
│           │  C++ internal calls                                     │
│           ▼                                                         │
├─────────────────────────────────────────────────────────────────────┤
│  C++ Engine  (PIC static library: libpdfengine.a)                  │
│   engine/src/core/PdfDocument.cpp    MuPDF rendering/text          │
│   engine/src/core/PdfMerger.cpp      QPDF structural merge         │
│   engine/src/core/PdfCompressor.cpp  QPDF + zlib compression       │
│   engine/src/core/PdfSplitter.cpp    QPDF page extraction          │
│           │                                                         │
│      MuPDF (fz_context)   QPDF (QPDFWriter)   zlib                 │
└─────────────────────────────────────────────────────────────────────┘
```

---

## The C ABI Boundary — Hard Rules

`engine/include/pdfengine.h` is the **only** header that `cgo` ever
sees.  Every symbol that crosses this line must follow these rules:

| Rule | Reason |
|------|--------|
| No C++ types (std::string, vectors, exceptions) | cgo is a C compiler |
| No C++ name mangling — all exports use `extern "C"` | Go cannot call mangled symbols |
| Strings are `const char*` UTF-8, null-terminated | Go's `C.GoString()` converts these |
| Heap strings are freed by `pm_free_string()` | Caller owns the buffer |
| Opaque handles are `void*` | Go holds them without knowing the layout |
| Errors returned as `int` code + optional `const char*` | Exceptions must not cross the boundary |
| Progress callbacks are `typedef void (*)(int, int, const char*, void*)` | Simple C function pointer |

### Why not use CGo's `//export` for the engine?

`//export` annotations in Go generate C header stubs, but they cannot
call back into Go from a C++ destructor or from a C++ thread.  Instead,
PDFMaster uses a **trampoline pattern**:

1. A static C function `get_progress_trampoline()` returns a C function pointer.
2. The Go bridge registers a callback ID in a `sync.Map`.
3. The C trampoline calls `goProgressCallback(cur, total, msg, id)` which is
   exported to C via `//export` in `bridge.go`.
4. `goProgressCallback` looks up the ID and calls the Go closure.

This keeps the C++ engine fully decoupled from Go's runtime.

---

## Memory Model

```
C++ engine allocates    →  pm_free_string() / pm_free_render() releases
Go bridge calls malloc  →  Go's GC never sees raw C pointers
C strings copied to Go  →  C.GoString() / C.GoStringN() make heap copies
```

**Critical:** `cgo` does not allow Go pointers to be passed to C
if the Go pointer is stored in C memory (cgo pointer passing rules).
The bridge avoids this by passing integer IDs (`uintptr`) as the
`void* userdata` for callbacks, never actual Go pointers.

---

## Build Pipeline

```
cmake (engine)                      go build (binary)
─────────────────                   ─────────────────────────────
src/core/*.cpp                      cmd/pdfmaster/*.go
src/bridge/pdfengine_c.cpp          internal/bridge/bridge.go
         │                                    │
         ▼                          CGO_CFLAGS=-I engine/include
 libpdfengine.a (PIC)               CGO_LDFLAGS=-L build/lib
         │                                    │
         └──────────────────────────────────→ pdfmaster binary
                                    (static engine + Go runtime)
```

The final binary contains:
- The entire Go runtime + standard library
- All Go packages (cobra, bubbletea, lipgloss, ...)
- `libpdfengine.a` statically linked
- At runtime: dynamically links libmupdf.so and libqpdf.so
  (or links them statically if built with vcpkg `*-static` triplets)

---

## Concurrency Model

```
main goroutine
    │  cobra.Execute()
    ▼
command RunE goroutine
    │  ops.Merge() / Compress() / Split()
    │  validates, prepares, calls bridge.Merge()
    │
    ├──[C++]── pm_merge() runs on the calling goroutine's OS thread
    │          (runtime.LockOSThread is NOT needed — cgo handles this)
    │
    │  progress callback arrives on same OS thread
    │  → goProgressCallback() calls Go closure
    │  → updates progress.Bar safely
    │
    └── returns MergeStats to Go
```

Long operations (merge, compress, split) run on the calling goroutine.
Go's scheduler cooperates correctly because cgo releases the goroutine's
P (processor) when entering C code, allowing other goroutines to run.

For truly concurrent batch operations (e.g. compress 100 files at once),
wrap each `ops.*` call in a goroutine with a `sync.WaitGroup`:

```go
var wg sync.WaitGroup
sem := make(chan struct{}, runtime.NumCPU()) // limit concurrency

for _, f := range files {
    wg.Add(1)
    go func(path string) {
        defer wg.Done()
        sem <- struct{}{}
        defer func() { <-sem }()
        ops.Compress(ops.CompressOptions{InputPath: path, ...})
    }(f)
}
wg.Wait()
```

---

## Adding a New Engine Operation

### Step 1 — C++ header in `engine/src/core/`

```cpp
// PdfWatermark.h
namespace pdfmaster {
struct WatermarkOptions { ... };
struct WatermarkResult  { ... };
class PdfWatermarker {
    static WatermarkResult apply(const std::string& in,
                                  const std::string& out,
                                  const WatermarkOptions& opts);
};
}
```

### Step 2 — Add to C ABI in `engine/include/pdfengine.h`

```c
typedef struct { ... } pm_watermark_opts_t;

int pm_watermark(const char* input, const char* output,
                 const pm_watermark_opts_t* opts,
                 pm_progress_cb_t cb, void* userdata);
```

### Step 3 — Implement bridge in `engine/src/bridge/pdfengine_c.cpp`

```cpp
extern "C" int pm_watermark(const char* in, const char* out,
                              const pm_watermark_opts_t* opts, ...)
{
    PM_TRY({ ... });
    return PM_OK;
}
```

### Step 4 — Go bridge in `internal/bridge/bridge.go`

```go
func Watermark(inputPath, outputPath string, ...) error {
    cIn  := C.CString(inputPath)
    defer C.free(unsafe.Pointer(cIn))
    // ...
    rc := C.pm_watermark(cIn, cOut, &copts, trampoline, unsafe.Pointer(cbID))
    return checkErr(rc)
}
```

### Step 5 — Go ops in `internal/ops/watermark.go`

```go
func Watermark(opts WatermarkOptions) (*WatermarkResult, error) { ... }
```

### Step 6 — Cobra command in `cmd/pdfmaster/cmd_watermark.go`

```go
var watermarkCmd = &cobra.Command{ Use: "watermark ...", RunE: ... }
func init() { rootCmd.AddCommand(watermarkCmd) }
```

---

## File Layout

```
pdfmaster-hybrid/
├── Makefile                    ← unified build (engine + Go)
├── README.md
├── cmake/
│   ├── FindMuPDF.cmake
│   └── FindQPDF.cmake
├── engine/
│   ├── CMakeLists.txt          ← builds libpdfengine.a
│   ├── include/
│   │   └── pdfengine.h         ← PUBLIC C ABI — cgo reads this
│   └── src/
│       ├── bridge/
│       │   └── pdfengine_c.cpp ← extern "C" glue
│       └── core/
│           ├── PdfDocument.h/.cpp
│           ├── PdfMerger.h/.cpp
│           ├── PdfCompressor.h/.cpp
│           └── PdfSplitter.h/.cpp
├── go/
│   ├── go.mod
│   ├── cmd/pdfmaster/
│   │   ├── root.go             ← Cobra root + main()
│   │   ├── cmd_info.go
│   │   ├── cmd_view.go
│   │   ├── cmd_text.go
│   │   ├── cmd_merge.go
│   │   ├── cmd_compress.go
│   │   └── cmd_split.go
│   └── internal/
│       ├── bridge/
│       │   └── bridge.go       ← ONLY file with import "C"
│       ├── ops/
│       │   ├── merge.go
│       │   ├── compress.go
│       │   ├── split.go
│       │   └── view.go
│       ├── progress/
│       │   └── progress.go
│       └── config/
│           └── config.go
└── docs/
    ├── ARCHITECTURE.md         ← this file
    └── USER_MANUAL.md
```
