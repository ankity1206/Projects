// internal/bridge/bridge.go
//
// CGo bridge — the ONLY package in the Go codebase that imports "C".
// All other packages call through this package's pure-Go API.
// Keeps cgo contamination isolated to a single compilation unit.

package bridge

/*
#cgo CFLAGS:  -I${SRCDIR}/../../engine/include
#cgo LDFLAGS: -L${SRCDIR}/../../engine/build/lib -lpdfengine
#cgo LDFLAGS: -lstdc++ -lm -lpthread -ldl -lz
#cgo linux  LDFLAGS: -Wl,--allow-multiple-definition
#cgo windows LDFLAGS: -static-libgcc -static-libstdc++

#include "pdfengine.h"
#include <stdlib.h>
#include <string.h>

// Go cannot call C variadic functions or pass Go function pointers
// directly as C callbacks.  We use a trampoline:
// a static C function that calls back into a registered Go func.

extern void goProgressCallback(int current, int total, const char* msg, void* userdata);

static pm_progress_cb_t get_progress_trampoline(void) {
    return (pm_progress_cb_t)goProgressCallback;
}
*/
import "C"
import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"
)

// ── Progress callback registry ────────────────────────────────────────
// cgo cannot store Go func values in C memory, so we use an integer
// handle that maps to a Go callback stored in a sync.Map.

type ProgressFunc func(current, total int, message string)

var (
	cbMu      sync.Mutex
	cbCounter uintptr
	cbMap     = make(map[uintptr]ProgressFunc)
)

func registerCb(fn ProgressFunc) uintptr {
	if fn == nil {
		return 0
	}
	cbMu.Lock()
	defer cbMu.Unlock()
	cbCounter++
	id := cbCounter
	cbMap[id] = fn
	return id
}

func unregisterCb(id uintptr) {
	if id == 0 {
		return
	}
	cbMu.Lock()
	defer cbMu.Unlock()
	delete(cbMap, id)
}

//export goProgressCallback
func goProgressCallback(current, total C.int, msg *C.char, userdata unsafe.Pointer) {
	id := uintptr(userdata)
	cbMu.Lock()
	fn, ok := cbMap[id]
	cbMu.Unlock()
	if ok && fn != nil {
		fn(int(current), int(total), C.GoString(msg))
	}
}

// ── Error type ────────────────────────────────────────────────────────

type EngineError struct {
	Code    int
	Message string
}

func (e *EngineError) Error() string {
	return fmt.Sprintf("engine error %d: %s", e.Code, e.Message)
}

func checkErr(code C.int) error {
	if code == C.PM_OK {
		return nil
	}
	return &EngineError{
		Code:    int(code),
		Message: C.GoString(C.pm_strerror(code)),
	}
}

// ── Lifecycle ─────────────────────────────────────────────────────────

// Init initialises the C++ engine. Call once at program startup.
func Init() error {
	return checkErr(C.pm_engine_init())
}

// Shutdown tears down the engine. Call at program exit.
func Shutdown() {
	C.pm_engine_shutdown()
}

// Version returns the engine version string.
func Version() string {
	return C.GoString(C.pm_engine_version())
}

// ── Document ──────────────────────────────────────────────────────────

// Doc is an opaque handle to an open PDF document.
// Must be closed with Close() when done.
type Doc struct {
	handle C.pm_doc_t
}

// OpenDoc opens a PDF file and returns a Doc handle.
func OpenDoc(path string) (*Doc, error) {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))

	var errCode C.int
	var errMsg *C.char
	h := C.pm_doc_open(cpath, &errCode, &errMsg)
	if h == nil {
		msg := ""
		if errMsg != nil {
			msg = C.GoString(errMsg)
		}
		return nil, &EngineError{Code: int(errCode), Message: msg}
	}
	d := &Doc{handle: h}
	runtime.SetFinalizer(d, (*Doc).Close) // safety net
	return d, nil
}

// Close releases the document handle.
func (d *Doc) Close() {
	if d.handle != nil {
		C.pm_doc_close(d.handle)
		d.handle = nil
		runtime.SetFinalizer(d, nil)
	}
}

// PageCount returns the number of pages.
func (d *Doc) PageCount() int {
	return int(C.pm_doc_page_count(d.handle))
}

// DocInfo holds document metadata.
type DocInfo struct {
	Title, Author, Subject, Creator, Producer string
	CreationDate, ModDate, PdfVersion         string
	PageCount                                  int
	Encrypted                                  bool
	FileSizeBytes                              int64
}

// Info returns document metadata.
func (d *Doc) Info() (*DocInfo, error) {
	var ci C.pm_doc_info_t
	if rc := C.pm_doc_get_info(d.handle, &ci); rc != C.PM_OK {
		return nil, checkErr(rc)
	}
	return &DocInfo{
		Title:         C.GoString(&ci.title[0]),
		Author:        C.GoString(&ci.author[0]),
		Subject:       C.GoString(&ci.subject[0]),
		Creator:       C.GoString(&ci.creator[0]),
		Producer:      C.GoString(&ci.producer[0]),
		CreationDate:  C.GoString(&ci.creation_date[0]),
		ModDate:       C.GoString(&ci.mod_date[0]),
		PdfVersion:    C.GoString(&ci.pdf_version[0]),
		PageCount:     int(ci.page_count),
		Encrypted:     ci.encrypted != 0,
		FileSizeBytes: int64(ci.file_size_bytes),
	}, nil
}

// PageInfo holds per-page metadata.
type PageInfo struct {
	Index       int
	WidthPt     float64
	HeightPt    float64
	Rotation    int
	StreamBytes uint64
}

// PageInfo returns metadata for a single page (0-based index).
func (d *Doc) PageInfo(index int) (*PageInfo, error) {
	var cp C.pm_page_info_t
	if rc := C.pm_doc_get_page_info(d.handle, C.int(index), &cp); rc != C.PM_OK {
		return nil, checkErr(rc)
	}
	return &PageInfo{
		Index:       int(cp.page_index),
		WidthPt:     float64(cp.width_pt),
		HeightPt:    float64(cp.height_pt),
		Rotation:    int(cp.rotation),
		StreamBytes: uint64(cp.stream_bytes),
	}, nil
}

// ── Rendering ─────────────────────────────────────────────────────────

// RenderedPage is a raw RGB24 pixel buffer.
type RenderedPage struct {
	Pixels []byte
	Width  int
	Height int
	Stride int
}

// RenderPage renders page at the given DPI and optional extra rotation.
func (d *Doc) RenderPage(pageIndex int, dpi float32, rotation int) (*RenderedPage, error) {
	var result C.pm_render_result_t
	rc := C.pm_render_page(d.handle, C.int(pageIndex),
		C.float(dpi), C.int(rotation), &result)
	if rc != C.PM_OK {
		return nil, checkErr(rc)
	}
	defer C.pm_free_render(&result)

	sz := int(result.stride) * int(result.height)
	pixels := make([]byte, sz)
	C.memcpy(unsafe.Pointer(&pixels[0]),
		unsafe.Pointer(result.pixels), C.size_t(sz))

	return &RenderedPage{
		Pixels: pixels,
		Width:  int(result.width),
		Height: int(result.height),
		Stride: int(result.stride),
	}, nil
}

// ── Text extraction ───────────────────────────────────────────────────

// ExtractTextPage returns the text content of one page.
func (d *Doc) ExtractTextPage(pageIndex int) (string, error) {
	var ctext *C.char
	var clen C.size_t
	rc := C.pm_extract_text_page(d.handle, C.int(pageIndex), &ctext, &clen)
	if rc != C.PM_OK {
		return "", checkErr(rc)
	}
	defer C.pm_free_string(ctext)
	return C.GoStringN(ctext, C.int(clen)), nil
}

// ExtractAllText returns text from all pages, separated by \f.
func (d *Doc) ExtractAllText(progress ProgressFunc) (string, error) {
	cbID := registerCb(progress)
	defer unregisterCb(cbID)

	var ctext *C.char
	var clen C.size_t
	trampoline := C.get_progress_trampoline()
	rc := C.pm_extract_text_all(d.handle, &ctext, &clen,
		trampoline, unsafe.Pointer(cbID))
	if rc != C.PM_OK {
		return "", checkErr(rc)
	}
	defer C.pm_free_string(ctext)
	return C.GoStringN(ctext, C.int(clen)), nil
}

// ── Merge ─────────────────────────────────────────────────────────────

// MergeInput describes one input file with an optional page range.
type MergeInput struct {
	Path     string
	FromPage int // 1-based; 0 = use default (1)
	ToPage   int // 1-based; 0 = use default (last)
}

// MergeStats holds result statistics.
type MergeStats struct {
	OutputBytes int64
	TotalPages  int
	ElapsedMs   float64
}

// Merge combines input PDFs into outputPath.
func Merge(inputs []MergeInput, outputPath string, linearize bool,
	progress ProgressFunc) (*MergeStats, error) {

	n := len(inputs)
	if n == 0 {
		return nil, fmt.Errorf("no input files")
	}

	// Build C arrays
	cPaths := make([]*C.char, n)
	froms  := make([]C.int, n)
	tos    := make([]C.int, n)

	for i, inp := range inputs {
		cPaths[i] = C.CString(inp.Path)
		froms[i]  = C.int(inp.FromPage)
		tos[i]    = C.int(inp.ToPage)
	}
	defer func() {
		for _, p := range cPaths { C.free(unsafe.Pointer(p)) }
	}()

	cOut := C.CString(outputPath)
	defer C.free(unsafe.Pointer(cOut))

	cbID := registerCb(progress)
	defer unregisterCb(cbID)

	var stats C.pm_merge_stats_t
	trampoline := C.get_progress_trampoline()
	rc := C.pm_merge(
		(**C.char)(unsafe.Pointer(&cPaths[0])),
		C.int(n),
		(*C.int)(unsafe.Pointer(&froms[0])),
		(*C.int)(unsafe.Pointer(&tos[0])),
		cOut,
		boolToInt(linearize),
		&stats,
		trampoline,
		unsafe.Pointer(cbID),
	)
	if rc != C.PM_OK {
		return nil, checkErr(rc)
	}
	return &MergeStats{
		OutputBytes: int64(stats.output_bytes),
		TotalPages:  int(stats.total_pages),
		ElapsedMs:   float64(stats.elapsed_ms),
	}, nil
}

// ── Compress ──────────────────────────────────────────────────────────

// CompressLevel mirrors PM_COMPRESS_* constants.
type CompressLevel int

const (
	CompressLight  CompressLevel = 1
	CompressMedium CompressLevel = 2
	CompressHeavy  CompressLevel = 3
)

// CompressOpts holds compress options.
type CompressOpts struct {
	Level              CompressLevel
	RemoveMetadata     bool
	RemoveJavaScript   bool
	RemoveAnnotations  bool
	JpegQuality        int
	MaxImageDPI        int
}

// CompressStats holds compress result statistics.
type CompressStats struct {
	OriginalBytes  int64
	OutputBytes    int64
	SavingsPct     float64
	PagesProcessed int
	ElapsedMs      float64
}

// Compress compresses inputPath into outputPath.
func Compress(inputPath, outputPath string, opts CompressOpts,
	progress ProgressFunc) (*CompressStats, error) {

	cIn  := C.CString(inputPath)
	cOut := C.CString(outputPath)
	defer C.free(unsafe.Pointer(cIn))
	defer C.free(unsafe.Pointer(cOut))

	copts := C.pm_compress_opts_t{
		level:               C.int(opts.Level),
		remove_metadata:     boolToInt(opts.RemoveMetadata),
		remove_javascript:   boolToInt(opts.RemoveJavaScript),
		remove_annotations:  boolToInt(opts.RemoveAnnotations),
		jpeg_quality:        C.int(opts.JpegQuality),
		max_image_dpi:       C.int(opts.MaxImageDPI),
	}

	cbID := registerCb(progress)
	defer unregisterCb(cbID)

	var stats C.pm_compress_stats_t
	trampoline := C.get_progress_trampoline()
	rc := C.pm_compress(cIn, cOut, &copts, &stats,
		trampoline, unsafe.Pointer(cbID))
	if rc != C.PM_OK {
		return nil, checkErr(rc)
	}
	return &CompressStats{
		OriginalBytes:  int64(stats.original_bytes),
		OutputBytes:    int64(stats.output_bytes),
		SavingsPct:     float64(stats.savings_pct),
		PagesProcessed: int(stats.pages_processed),
		ElapsedMs:      float64(stats.elapsed_ms),
	}, nil
}

// ── Split ─────────────────────────────────────────────────────────────

// SplitMode mirrors PM_SPLIT_* constants.
type SplitMode int

const (
	SplitPages  SplitMode = 0
	SplitRange  SplitMode = 1
	SplitChunks SplitMode = 2
)

// SplitStats holds split result statistics.
type SplitStats struct {
	FilesWritten int
	TotalPages   int
	ElapsedMs    float64
}

// Split splits inputPath into outputDir.
func Split(inputPath, outputDir, nameTemplate string,
	mode SplitMode, fromPage, toPage, chunkSize int,
	progress ProgressFunc) (*SplitStats, error) {

	cIn   := C.CString(inputPath)
	cDir  := C.CString(outputDir)
	cTmpl := C.CString(nameTemplate)
	defer C.free(unsafe.Pointer(cIn))
	defer C.free(unsafe.Pointer(cDir))
	defer C.free(unsafe.Pointer(cTmpl))

	cbID := registerCb(progress)
	defer unregisterCb(cbID)

	var stats C.pm_split_stats_t
	trampoline := C.get_progress_trampoline()
	rc := C.pm_split(cIn, cDir, cTmpl,
		C.int(mode),
		C.int(fromPage), C.int(toPage), C.int(chunkSize),
		&stats, trampoline, unsafe.Pointer(cbID))
	if rc != C.PM_OK {
		return nil, checkErr(rc)
	}
	return &SplitStats{
		FilesWritten: int(stats.files_written),
		TotalPages:   int(stats.total_pages),
		ElapsedMs:    float64(stats.elapsed_ms),
	}, nil
}

// ── Utilities ─────────────────────────────────────────────────────────

// QuickPageCount returns the page count of a PDF without a full open.
func QuickPageCount(path string) (int, error) {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	n := C.pm_quick_page_count(cpath)
	if n < 0 {
		return 0, &EngineError{Code: int(n), Message: "could not count pages"}
	}
	return int(n), nil
}

// ValidatePDF checks that a file is a valid readable PDF.
func ValidatePDF(path string) error {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	var cerr *C.char
	rc := C.pm_validate_pdf(cpath, &cerr)
	if rc != C.PM_OK {
		msg := ""
		if cerr != nil { msg = C.GoString(cerr); C.pm_free_string(cerr) }
		return &EngineError{Code: int(rc), Message: msg}
	}
	return nil
}

// ── helpers ────────────────────────────────────────────────────────────
func boolToInt(b bool) C.int {
	if b { return 1 }
	return 0
}
