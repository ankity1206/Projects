// cmd/pdfmaster/cmd_compress.go
package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yourname/pdfmaster/internal/bridge"
	"github.com/yourname/pdfmaster/internal/ops"
	"github.com/yourname/pdfmaster/internal/progress"
)

var (
	compressOutput     string
	compressLevel      int
	compressMeta       bool
	compressJS         bool
	compressAnnot      bool
	compressJpegQ      int
	compressMaxDPI     int
)

var compressCmd = &cobra.Command{
	Use:   "compress [flags] <input.pdf>",
	Short: "Reduce PDF file size",
	Long: `Compresses a PDF using structural QPDF optimisation.

Levels:
  1 / light   Object-stream compaction only.             (~10-25% reduction)
  2 / medium  + recompress all streams at zlib level 9.  (~25-45% reduction)
  3 / heavy   + content normalisation, image downscaling (~40-70% reduction)

Heavy mode resamples raster images above --max-image-dpi to that DPI
then re-encodes as JPEG at --jpeg-quality. Vector/text is always lossless.`,
	Args: cobra.ExactArgs(1),
	Example: `  pdfmaster compress -o small.pdf input.pdf
  pdfmaster compress -l 3 --jpeg-quality 60 -o small.pdf input.pdf
  pdfmaster compress -l light --remove-metadata -o out.pdf in.pdf`,
	RunE: func(cmd *cobra.Command, args []string) error {
		level := bridge.CompressLevel(compressLevel)
		switch {
		case compressLevel == 1:
			level = bridge.CompressLight
		case compressLevel == 3:
			level = bridge.CompressHeavy
		default:
			level = bridge.CompressMedium
		}

		opts := ops.CompressOptions{
			InputPath:         args[0],
			OutputPath:        compressOutput,
			Level:             level,
			RemoveMetadata:    compressMeta,
			RemoveJavaScript:  compressJS,
			RemoveAnnotations: compressAnnot,
			JpegQuality:       compressJpegQ,
			MaxImageDPI:       compressMaxDPI,
			Verbose:           flagVerbose,
		}

		result, err := ops.Compress(opts)
		if err != nil {
			return err
		}

		fmt.Printf("\n  Input  : %s (%s)\n",
			result.InputPath, progress.HumanBytes(result.OriginalBytes))
		fmt.Printf("  Output : %s (%s)\n",
			result.OutputPath, progress.HumanBytes(result.OutputBytes))
		fmt.Printf("  Saved  : %s (%.1f%%)\n",
			progress.HumanBytes(result.OriginalBytes-result.OutputBytes),
			result.SavingsPct)
		fmt.Printf("  Pages  : %d\n", result.PagesProcessed)
		fmt.Printf("  Time   : %s\n\n", progress.HumanMs(result.ElapsedMs))
		return nil
	},
}

func init() {
	compressCmd.Flags().StringVarP(&compressOutput, "output", "o", "",
		"Output PDF path (default: <input>_compressed.pdf)")
	compressCmd.Flags().IntVarP(&compressLevel, "level", "l", 2,
		"Compression level: 1=light, 2=medium, 3=heavy")
	compressCmd.Flags().BoolVar(&compressMeta, "remove-metadata", false,
		"Strip document metadata (title, author, dates)")
	compressCmd.Flags().BoolVar(&compressJS, "remove-js", true,
		"Remove embedded JavaScript (recommended)")
	compressCmd.Flags().BoolVar(&compressAnnot, "remove-annotations", false,
		"Remove all annotations (comments, highlights)")
	compressCmd.Flags().IntVar(&compressJpegQ, "jpeg-quality", 72,
		"JPEG re-encode quality for heavy mode (1-100)")
	compressCmd.Flags().IntVar(&compressMaxDPI, "max-image-dpi", 150,
		"Downsample images above this DPI (heavy mode only)")
	rootCmd.AddCommand(compressCmd)
}
