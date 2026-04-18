// cmd/pdfmaster/cmd_text.go
package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yourname/pdfmaster/internal/ops"
	"github.com/yourname/pdfmaster/internal/progress"
)

var (
	textOutput string
	textPage   int
)

var textCmd = &cobra.Command{
	Use:   "text [flags] <input.pdf>",
	Short: "Extract text from a PDF",
	Long: `Extracts readable text from PDF pages using MuPDF's text engine.
Output is UTF-8. Pages are separated by form-feed characters (\f).
Scanned PDFs (image-only) will produce empty output — use an OCR tool first.`,
	Args: cobra.ExactArgs(1),
	Example: `  pdfmaster text document.pdf
  pdfmaster text -o out.txt document.pdf
  pdfmaster text --page 3 document.pdf`,
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := ops.TextExtractOptions{
			InputPath:  args[0],
			OutputPath: textOutput,
			PageIndex:  textPage - 1, // convert 1-based flag to 0-based
			Verbose:    flagVerbose,
		}
		if textPage == 0 {
			opts.PageIndex = -1 // all pages
		}

		result, err := ops.ExtractText(opts)
		if err != nil {
			return err
		}

		if flagVerbose && result.OutputPath != "<stdout>" {
			fmt.Printf("\n  Output : %s\n", result.OutputPath)
			fmt.Printf("  Bytes  : %s\n", progress.HumanBytes(int64(result.BytesOut)))
			fmt.Printf("  Pages  : %d\n", result.Pages)
			fmt.Printf("  Time   : %s\n\n", progress.HumanMs(result.ElapsedMs))
		}
		return nil
	},
}

func init() {
	textCmd.Flags().StringVarP(&textOutput, "output", "o", "",
		"Output text file path (default: stdout)")
	textCmd.Flags().IntVar(&textPage, "page", 0,
		"Extract single page (1-based); 0 = all pages")
	rootCmd.AddCommand(textCmd)
}
