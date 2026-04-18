// internal/ops/view.go
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

// ViewOptions holds parameters for the view/info command.
type ViewOptions struct {
	InputPath string
	PageIndex int    // 0-based; -1 = show all info only
	DPI       float32
	Rotation  int
	OutputPNG string // if set, render to PNG file
}

// TextExtractOptions holds parameters for text extraction.
type TextExtractOptions struct {
	InputPath   string
	OutputPath  string // "-" = stdout
	PageIndex   int    // -1 = all pages
	Verbose     bool
}

// TextExtractResult holds text extraction results.
type TextExtractResult struct {
	InputPath  string
	OutputPath string
	BytesOut   int
	Pages      int
	ElapsedMs  float64
}

// ExtractText extracts text from a PDF page or all pages.
func ExtractText(opts TextExtractOptions) (*TextExtractResult, error) {
	t0 := time.Now()

	if err := validatePDFInput(opts.InputPath); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	doc, err := bridge.OpenDoc(opts.InputPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open %q: %w", opts.InputPath, err)
	}
	defer doc.Close()

	var text string
	pages := doc.PageCount()

	if opts.PageIndex >= 0 {
		// Single page
		bar := progress.NewBar("Extracting text")
		bar.Update(0, 1, fmt.Sprintf("page %d", opts.PageIndex+1))
		text, err = doc.ExtractTextPage(opts.PageIndex)
		if err != nil {
			bar.Fail(err)
			return nil, fmt.Errorf("text extraction failed: %w", err)
		}
		bar.Done("page extracted")
		pages = 1
	} else {
		// All pages
		bar := progress.NewBar("Extracting text")
		cb := func(cur, total int, msg string) {
			bar.Update(cur, total, msg)
		}
		text, err = doc.ExtractAllText(cb)
		if err != nil {
			bar.Fail(err)
			return nil, fmt.Errorf("text extraction failed: %w", err)
		}
		bar.Done(fmt.Sprintf("%d pages extracted", pages))
	}

	// Write output
	outPath := opts.OutputPath
	if outPath == "" || outPath == "-" {
		fmt.Print(text)
		return &TextExtractResult{
			InputPath:  opts.InputPath,
			OutputPath: "<stdout>",
			BytesOut:   len(text),
			Pages:      pages,
			ElapsedMs:  float64(time.Since(t0).Milliseconds()),
		}, nil
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return nil, fmt.Errorf("cannot create output directory: %w", err)
	}
	if err := os.WriteFile(outPath, []byte(text), 0o644); err != nil {
		return nil, fmt.Errorf("cannot write output file: %w", err)
	}

	return &TextExtractResult{
		InputPath:  opts.InputPath,
		OutputPath: outPath,
		BytesOut:   len(text),
		Pages:      pages,
		ElapsedMs:  float64(time.Since(t0).Milliseconds()),
	}, nil
}

// PrintInfo prints human-readable document metadata to stdout.
func PrintInfo(path string) error {
	info, err := GetInfo(path)
	if err != nil {
		return err
	}

	// Colour palette via lipgloss-compatible ANSI
	key  := func(s string) string { return "\033[38;5;111m" + s + "\033[0m" }
	val  := func(s string) string { return "\033[38;5;252m" + s + "\033[0m" }
	head := func(s string) string { return "\033[1;38;5;63m"  + s + "\033[0m" }
	dim  := func(s string) string { return "\033[38;5;240m"   + s + "\033[0m" }

	fmt.Println()
	fmt.Println(head("  ▸ Document Information"))
	fmt.Println(dim("  " + strings.Repeat("─", 46)))

	row := func(k, v string) {
		if v != "" {
			fmt.Printf("  %s  %s\n", key(fmt.Sprintf("%-18s", k)), val(v))
		}
	}

	row("File",          filepath.Base(path))
	row("Size",          progress.HumanBytes(info.FileSizeBytes))
	row("PDF Version",   info.PdfVersion)
	row("Pages",         fmt.Sprintf("%d", info.PageCount))
	if info.Encrypted {
		row("Encrypted", "\033[38;5;196mYes\033[0m")
	} else {
		row("Encrypted", "No")
	}
	fmt.Println(dim("  " + strings.Repeat("─", 46)))
	row("Title",         info.Title)
	row("Author",        info.Author)
	row("Subject",       info.Subject)
	row("Creator",       info.Creator)
	row("Producer",      info.Producer)
	row("Created",       info.CreationDate)
	row("Modified",      info.ModDate)
	fmt.Println()
	return nil
}
