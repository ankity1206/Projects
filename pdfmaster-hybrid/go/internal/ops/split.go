// internal/ops/split.go
package ops

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/yourname/pdfmaster/internal/bridge"
	"github.com/yourname/pdfmaster/internal/progress"
)

// SplitMode mirrors bridge.SplitMode for the ops layer.
type SplitMode int

const (
	SplitModePages  SplitMode = SplitMode(bridge.SplitPages)
	SplitModeRange  SplitMode = SplitMode(bridge.SplitRange)
	SplitModeChunks SplitMode = SplitMode(bridge.SplitChunks)
)

// SplitOptions holds all parameters for a split operation.
type SplitOptions struct {
	InputPath    string
	OutputDir    string
	NameTemplate string    // e.g. "{name}_{from:04d}-{to:04d}.pdf"
	Mode         SplitMode
	FromPage     int       // 1-based; used by Range mode
	ToPage       int       // 1-based; -1 = last
	ChunkSize    int       // used by Chunks mode
	Verbose      bool
}

// SplitResult holds the outcome of a split operation.
type SplitResult struct {
	InputPath    string
	OutputDir    string
	FilesWritten int
	TotalPages   int
	ElapsedMs    float64
}

// Split validates inputs and calls the C++ split engine.
func Split(opts SplitOptions) (*SplitResult, error) {
	t0 := time.Now()

	// ── Validate ────────────────────────────────────────────────
	if err := validatePDFInput(opts.InputPath); err != nil {
		return nil, fmt.Errorf("invalid input %q: %w", opts.InputPath, err)
	}

	// Default output dir = directory of input file
	if opts.OutputDir == "" {
		opts.OutputDir = filepath.Dir(opts.InputPath)
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return nil, fmt.Errorf("cannot create output directory %q: %w",
			opts.OutputDir, err)
	}

	// Validate mode-specific args
	switch opts.Mode {
	case SplitModeRange:
		if opts.FromPage < 1 {
			opts.FromPage = 1
		}
		if opts.ToPage == 0 {
			opts.ToPage = -1
		}
	case SplitModeChunks:
		if opts.ChunkSize < 1 {
			return nil, fmt.Errorf("chunk size must be ≥ 1")
		}
	case SplitModePages:
		opts.ChunkSize = 1
		opts.FromPage  = 1
		opts.ToPage    = -1
	}

	// Preflight: quick page count for display
	totalPages, err := bridge.QuickPageCount(opts.InputPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read page count: %w", err)
	}
	if totalPages == 0 {
		return nil, fmt.Errorf("document has no pages")
	}

	// ── Progress bar ─────────────────────────────────────────────
	bar := progress.NewBar("Splitting")
	bar.Update(0, totalPages, fmt.Sprintf("%d pages total", totalPages))

	cb := progress.MakeBridgeCb(bar)

	// ── Call engine ──────────────────────────────────────────────
	stats, err := bridge.Split(
		opts.InputPath,
		opts.OutputDir,
		opts.NameTemplate,
		bridge.SplitMode(opts.Mode),
		opts.FromPage,
		opts.ToPage,
		opts.ChunkSize,
		cb,
	)
	if err != nil {
		bar.Fail(err)
		return nil, fmt.Errorf("split failed: %w", err)
	}

	result := &SplitResult{
		InputPath:    opts.InputPath,
		OutputDir:    opts.OutputDir,
		FilesWritten: stats.FilesWritten,
		TotalPages:   stats.TotalPages,
		ElapsedMs:    float64(time.Since(t0).Milliseconds()),
	}

	bar.Done(fmt.Sprintf(
		"%d files written to %s",
		result.FilesWritten,
		result.OutputDir,
	))
	return result, nil
}
