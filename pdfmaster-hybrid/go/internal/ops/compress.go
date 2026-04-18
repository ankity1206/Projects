// internal/ops/compress.go
package ops

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/yourname/pdfmaster/internal/bridge"
	"github.com/yourname/pdfmaster/internal/progress"
)

// CompressOptions holds all parameters for a compress operation.
type CompressOptions struct {
	InputPath         string
	OutputPath        string
	Level             bridge.CompressLevel
	RemoveMetadata    bool
	RemoveJavaScript  bool
	RemoveAnnotations bool
	JpegQuality       int // 1–100, used in Heavy mode
	MaxImageDPI       int // downsample images above this DPI
	Verbose           bool
}

// CompressResult holds the outcome of a compress operation.
type CompressResult struct {
	InputPath      string
	OutputPath     string
	OriginalBytes  int64
	OutputBytes    int64
	SavingsPct     float64
	PagesProcessed int
	ElapsedMs      float64
}

// Compress validates inputs and calls the C++ compress engine.
func Compress(opts CompressOptions) (*CompressResult, error) {
	t0 := time.Now()

	// ── Validate ────────────────────────────────────────────────
	if err := validatePDFInput(opts.InputPath); err != nil {
		return nil, fmt.Errorf("invalid input %q: %w", opts.InputPath, err)
	}
	if opts.OutputPath == "" {
		// Default: <name>_compressed.pdf beside input
		base := filepath.Base(opts.InputPath)
		ext  := filepath.Ext(base)
		name := base[:len(base)-len(ext)]
		opts.OutputPath = filepath.Join(
			filepath.Dir(opts.InputPath),
			name+"_compressed.pdf",
		)
	}

	absIn,  _ := filepath.Abs(opts.InputPath)
	absOut, _ := filepath.Abs(opts.OutputPath)
	if absIn == absOut {
		return nil, fmt.Errorf("output path must differ from input path")
	}
	if err := os.MkdirAll(filepath.Dir(absOut), 0o755); err != nil {
		return nil, fmt.Errorf("cannot create output directory: %w", err)
	}

	// Level defaults
	if opts.Level == 0 {
		opts.Level = bridge.CompressMedium
	}
	if opts.JpegQuality == 0 {
		opts.JpegQuality = 72
	}
	if opts.MaxImageDPI == 0 {
		opts.MaxImageDPI = 150
	}

	// ── Progress bar ─────────────────────────────────────────────
	bar := progress.NewBar("Compressing")
	bar.Update(0, 10, "opening document")

	bridgeOpts := bridge.CompressOpts{
		Level:             opts.Level,
		RemoveMetadata:    opts.RemoveMetadata,
		RemoveJavaScript:  opts.RemoveJavaScript,
		RemoveAnnotations: opts.RemoveAnnotations,
		JpegQuality:       opts.JpegQuality,
		MaxImageDPI:       opts.MaxImageDPI,
	}

	cb := progress.MakeBridgeCb(bar)

	// ── Call engine ──────────────────────────────────────────────
	stats, err := bridge.Compress(opts.InputPath, opts.OutputPath, bridgeOpts, cb)
	if err != nil {
		bar.Fail(err)
		return nil, fmt.Errorf("compress failed: %w", err)
	}

	result := &CompressResult{
		InputPath:      opts.InputPath,
		OutputPath:     opts.OutputPath,
		OriginalBytes:  stats.OriginalBytes,
		OutputBytes:    stats.OutputBytes,
		SavingsPct:     stats.SavingsPct,
		PagesProcessed: stats.PagesProcessed,
		ElapsedMs:      float64(time.Since(t0).Milliseconds()),
	}

	savedBytes := result.OriginalBytes - result.OutputBytes
	bar.Done(fmt.Sprintf(
		"%s → %s (saved %s, %.1f%%)",
		progress.HumanBytes(result.OriginalBytes),
		progress.HumanBytes(result.OutputBytes),
		progress.HumanBytes(savedBytes),
		result.SavingsPct,
	))
	return result, nil
}
