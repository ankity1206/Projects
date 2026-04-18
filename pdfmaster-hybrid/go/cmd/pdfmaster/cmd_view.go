// cmd/pdfmaster/cmd_view.go
// Renders a PDF page to a PNG file using the C++ engine.
// Writing PNG uses Go's stdlib image/png — no extra C++ needed.

package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yourname/pdfmaster/internal/bridge"
	"github.com/yourname/pdfmaster/internal/progress"
)

var (
	viewPage     int
	viewDPI      float32
	viewRotation int
	viewOutput   string
)

var viewCmd = &cobra.Command{
	Use:   "view [flags] <input.pdf>",
	Short: "Render a PDF page to PNG",
	Long: `Renders one page of a PDF to a PNG image using MuPDF.

If --output is omitted the PNG is written next to the input file as
  <name>_page<N>.png`,
	Args: cobra.ExactArgs(1),
	Example: `  pdfmaster view document.pdf
  pdfmaster view --page 5 --dpi 300 -o page5.png document.pdf
  pdfmaster view --page 1 --dpi 150 --rotate 90 document.pdf`,
	RunE: func(cmd *cobra.Command, args []string) error {
		inputPath := args[0]

		// Default output path
		if viewOutput == "" {
			base := filepath.Base(inputPath)
			ext  := filepath.Ext(base)
			name := base[:len(base)-len(ext)]
			viewOutput = filepath.Join(
				filepath.Dir(inputPath),
				fmt.Sprintf("%s_page%d.png", name, viewPage),
			)
		}

		fmt.Printf("Rendering page %d at %.0f DPI...\n", viewPage, viewDPI)

		doc, err := bridge.OpenDoc(inputPath)
		if err != nil {
			return fmt.Errorf("cannot open %q: %w", inputPath, err)
		}
		defer doc.Close()

		pageIdx := viewPage - 1
		if pageIdx < 0 { pageIdx = 0 }

		rendered, err := doc.RenderPage(pageIdx, viewDPI, viewRotation)
		if err != nil {
			return fmt.Errorf("render failed: %w", err)
		}

		// Build Go image.NRGBA from RGB24 buffer
		img := image.NewNRGBA(image.Rect(0, 0, rendered.Width, rendered.Height))
		for y := 0; y < rendered.Height; y++ {
			for x := 0; x < rendered.Width; x++ {
				srcIdx := y*rendered.Stride + x*3
				dstIdx := y*img.Stride + x*4
				img.Pix[dstIdx+0] = rendered.Pixels[srcIdx+0] // R
				img.Pix[dstIdx+1] = rendered.Pixels[srcIdx+1] // G
				img.Pix[dstIdx+2] = rendered.Pixels[srcIdx+2] // B
				img.Pix[dstIdx+3] = 255                        // A
			}
		}

		// Write PNG
		f, err := os.Create(viewOutput)
		if err != nil {
			return fmt.Errorf("cannot create output file: %w", err)
		}
		defer f.Close()

		if err := png.Encode(f, img); err != nil {
			return fmt.Errorf("PNG encode failed: %w", err)
		}

		stat, _ := f.Stat()
		fmt.Printf("\n  Output : %s\n", viewOutput)
		fmt.Printf("  Size   : %dx%d px\n", rendered.Width, rendered.Height)
		if stat != nil {
			fmt.Printf("  File   : %s\n", progress.HumanBytes(stat.Size()))
		}
		fmt.Printf("  DPI    : %.0f\n\n", viewDPI)

		// Optional: try to open with system viewer
		if !strings.Contains(viewOutput, "/dev/") {
			fmt.Printf("  Tip: open %s\n", viewOutput)
		}
		return nil
	},
}

func init() {
	viewCmd.Flags().IntVar(&viewPage, "page", 1,
		"Page to render (1-based)")
	viewCmd.Flags().Float32Var(&viewDPI, "dpi", 150,
		"Render resolution in DPI (72=screen, 150=quality, 300=print)")
	viewCmd.Flags().IntVar(&viewRotation, "rotate", 0,
		"Extra rotation in degrees (0|90|180|270)")
	viewCmd.Flags().StringVarP(&viewOutput, "output", "o", "",
		"Output PNG path (default: <input>_page<N>.png)")
	rootCmd.AddCommand(viewCmd)
}
