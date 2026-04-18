// internal/ops/merge.go
// Go orchestration for the merge operation.
// Validates inputs, dispatches to C++ engine, reports progress.

package ops

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yourname/pdfmaster/internal/bridge"
	"github.com/yourname/pdfmaster/internal/progress"
)

// MergeOptions holds all parameters for a merge operation.
type MergeOptions struct {
	InputPaths []string
	OutputPath string
	Linearize  bool
	PageRanges []PageRange // per-file ranges; nil = all pages
	Verbose    bool
}

// PageRange is a 1-based inclusive page range.
type PageRange struct {
	From int
	To   int // -1 = last page
}

// MergeResult holds the outcome of a merge.
type MergeResult struct {
	OutputPath  string
	OutputBytes int64
	TotalPages  int
	ElapsedMs   float64
}

// Merge validates inputs and calls the C++ merge engine.
func Merge(opts MergeOptions) (*MergeResult, error) {
	t0 := time.Now()

	// ── Validate ────────────────────────────────────────────────
	if len(opts.InputPaths) < 2 {
		return nil, fmt.Errorf("merge requires at least 2 input files (got %d)",
			len(opts.InputPaths))
	}
	if opts.OutputPath == "" {
		return nil, fmt.Errorf("output path is required")
	}

	// Check all inputs exist and are PDFs
	for _, p := range opts.InputPaths {
		if err := validatePDFInput(p); err != nil {
			return nil, fmt.Errorf("invalid input %q: %w", p, err)
		}
	}

	// Guard against overwriting an input
	absOut, _ := filepath.Abs(opts.OutputPath)
	for _, p := range opts.InputPaths {
		absIn, _ := filepath.Abs(p)
		if absIn == absOut {
			return nil, fmt.Errorf("output path %q is the same as input — would overwrite", p)
		}
	}

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(absOut), 0o755); err != nil {
		return nil, fmt.Errorf("cannot create output directory: %w", err)
	}

	// ── Build bridge inputs ─────────────────────────────────────
	inputs := make([]bridge.MergeInput, len(opts.InputPaths))
	for i, p := range opts.InputPaths {
		inputs[i].Path = p
		if i < len(opts.PageRanges) {
			inputs[i].FromPage = opts.PageRanges[i].From
			inputs[i].ToPage   = opts.PageRanges[i].To
		}
	}

	// ── Progress bar ────────────────────────────────────────────
	bar := progress.NewBar("Merging")
	bar.Update(0, len(inputs), "starting")

	cb := progress.MakeBridgeCb(bar)

	// ── Call engine ─────────────────────────────────────────────
	stats, err := bridge.Merge(inputs, opts.OutputPath, opts.Linearize, cb)
	if err != nil {
		bar.Fail(err)
		return nil, fmt.Errorf("merge failed: %w", err)
	}

	result := &MergeResult{
		OutputPath:  opts.OutputPath,
		OutputBytes: stats.OutputBytes,
		TotalPages:  stats.TotalPages,
		ElapsedMs:   float64(time.Since(t0).Milliseconds()),
	}

	bar.Done(fmt.Sprintf("%d pages → %s",
		result.TotalPages, progress.HumanBytes(result.OutputBytes)))
	return result, nil
}

// ── Info operation ────────────────────────────────────────────────────

// InfoResult holds the document information fields.
type InfoResult struct {
	FilePath      string
	FileSizeBytes int64
	PageCount     int
	PdfVersion    string
	Title         string
	Author        string
	Subject       string
	Creator       string
	Producer      string
	CreationDate  string
	ModDate       string
	Encrypted     bool
}

// GetInfo opens a PDF and returns its metadata.
func GetInfo(path string) (*InfoResult, error) {
	if err := validatePDFInput(path); err != nil {
		return nil, err
	}

	doc, err := bridge.OpenDoc(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open %q: %w", path, err)
	}
	defer doc.Close()

	info, err := doc.Info()
	if err != nil {
		return nil, fmt.Errorf("cannot read info from %q: %w", path, err)
	}

	return &InfoResult{
		FilePath:      path,
		FileSizeBytes: info.FileSizeBytes,
		PageCount:     info.PageCount,
		PdfVersion:    info.PdfVersion,
		Title:         info.Title,
		Author:        info.Author,
		Subject:       info.Subject,
		Creator:       info.Creator,
		Producer:      info.Producer,
		CreationDate:  info.CreationDate,
		ModDate:       info.ModDate,
		Encrypted:     info.Encrypted,
	}, nil
}

// ── Shared helpers ────────────────────────────────────────────────────

func validatePDFInput(path string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("file not found")
	}
	if err != nil {
		return fmt.Errorf("cannot stat file: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("path is a directory, not a file")
	}
	if !strings.HasSuffix(strings.ToLower(path), ".pdf") {
		return fmt.Errorf("file does not have .pdf extension")
	}
	return nil
}
