// cmd/pdfmaster/cmd_split.go
package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yourname/pdfmaster/internal/ops"
	"github.com/yourname/pdfmaster/internal/progress"
)

var (
	splitOutputDir   string
	splitMode        string
	splitFrom        int
	splitTo          int
	splitChunkSize   int
	splitTemplate    string
)

var splitCmd = &cobra.Command{
	Use:   "split [flags] <input.pdf>",
	Short: "Split a PDF into separate files",
	Long: `Splits a PDF using QPDF structural extraction.
Each output file is a complete, valid PDF — no re-rendering.

Modes:
  pages   One file per page (default). Output: name_0001-0001.pdf ...
  range   Extract from-page to to-page into a single file.
  chunks  Split into sequential groups of N pages each.

Name template tokens: {name} {from} {to} {n} {from:04d} {to:04d} {n:04d}`,
	Args: cobra.ExactArgs(1),
	Example: `  pdfmaster split input.pdf
  pdfmaster split --mode range --from 3 --to 7 -o ./out/ input.pdf
  pdfmaster split --mode chunks --chunk 10 -o ./chunks/ input.pdf
  pdfmaster split --template "{name}_p{n:04d}.pdf" input.pdf`,
	RunE: func(cmd *cobra.Command, args []string) error {
		mode := ops.SplitModePages
		switch splitMode {
		case "range":
			mode = ops.SplitModeRange
		case "chunks", "chunk":
			mode = ops.SplitModeChunks
		}

		opts := ops.SplitOptions{
			InputPath:    args[0],
			OutputDir:    splitOutputDir,
			NameTemplate: splitTemplate,
			Mode:         mode,
			FromPage:     splitFrom,
			ToPage:       splitTo,
			ChunkSize:    splitChunkSize,
			Verbose:      flagVerbose,
		}

		result, err := ops.Split(opts)
		if err != nil {
			return err
		}

		fmt.Printf("\n  Input  : %s\n", result.InputPath)
		fmt.Printf("  Output : %s\n", result.OutputDir)
		fmt.Printf("  Files  : %d\n", result.FilesWritten)
		fmt.Printf("  Pages  : %d processed\n", result.TotalPages)
		fmt.Printf("  Time   : %s\n\n", progress.HumanMs(result.ElapsedMs))
		return nil
	},
}

func init() {
	splitCmd.Flags().StringVarP(&splitOutputDir, "output", "o", "",
		"Output directory (default: same as input file)")
	splitCmd.Flags().StringVar(&splitMode, "mode", "pages",
		"Split mode: pages | range | chunks")
	splitCmd.Flags().IntVar(&splitFrom, "from", 1,
		"Start page (1-based, range mode)")
	splitCmd.Flags().IntVar(&splitTo, "to", -1,
		"End page (1-based, -1=last, range mode)")
	splitCmd.Flags().IntVar(&splitChunkSize, "chunk", 1,
		"Pages per chunk (chunks mode)")
	splitCmd.Flags().StringVar(&splitTemplate, "template", "",
		"Output filename template, e.g. \"{name}_{from:04d}-{to:04d}.pdf\"")
	rootCmd.AddCommand(splitCmd)
}
