// cmd/pdfmaster/cmd_merge.go
package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yourname/pdfmaster/internal/ops"
	"github.com/yourname/pdfmaster/internal/progress"
)

var (
	mergeOutput    string
	mergeLinearize bool
	mergeRanges    []string // e.g. "1-5" "2-" "-10"
)

var mergeCmd = &cobra.Command{
	Use:   "merge -o <output.pdf> <file1.pdf> <file2.pdf> [...]",
	Short: "Merge multiple PDFs into one",
	Long: `Structurally merges PDF files using QPDF.
Pages are copied at the object-graph level — no re-rendering,
no quality loss, and no pixel decoding.

Optional per-file page ranges can be specified with --range (one per input file).
Range format: "N-M" (1-based, inclusive). Use "-" to mean first or last.
  Examples:  "1-5"   pages 1 to 5
             "3-"    page 3 to end
             "-10"   first page to page 10
             "-"     all pages (default)`,
	Args: cobra.MinimumNArgs(2),
	Example: `  pdfmaster merge -o merged.pdf a.pdf b.pdf c.pdf
  pdfmaster merge -o out.pdf --range 1-10 --range 2-5 a.pdf b.pdf
  pdfmaster merge -o out.pdf --linearize a.pdf b.pdf`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Parse page ranges
		var ranges []ops.PageRange
		for i, r := range mergeRanges {
			if i >= len(args) {
				break
			}
			pr, err := parsePageRange(r)
			if err != nil {
				return fmt.Errorf("invalid range %q for file %d: %w", r, i+1, err)
			}
			ranges = append(ranges, pr)
		}

		opts := ops.MergeOptions{
			InputPaths: args,
			OutputPath: mergeOutput,
			Linearize:  mergeLinearize,
			PageRanges: ranges,
			Verbose:    flagVerbose,
		}

		result, err := ops.Merge(opts)
		if err != nil {
			return err
		}

		fmt.Printf("\n  Output : %s\n", result.OutputPath)
		fmt.Printf("  Size   : %s\n", progress.HumanBytes(result.OutputBytes))
		fmt.Printf("  Pages  : %d\n", result.TotalPages)
		fmt.Printf("  Time   : %s\n\n", progress.HumanMs(result.ElapsedMs))
		return nil
	},
}

func init() {
	mergeCmd.Flags().StringVarP(&mergeOutput, "output", "o", "",
		"Output PDF path (required)")
	mergeCmd.Flags().BoolVar(&mergeLinearize, "linearize", false,
		"Linearize (web-optimise) the output PDF")
	mergeCmd.Flags().StringArrayVar(&mergeRanges, "range", nil,
		"Page range per input file, e.g. \"1-5\" (repeat for each file)")
	_ = mergeCmd.MarkFlagRequired("output")
	rootCmd.AddCommand(mergeCmd)
}

// parsePageRange parses "N-M", "N-", "-M", or "-".
func parsePageRange(s string) (ops.PageRange, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return ops.PageRange{From: 1, To: -1}, nil
	}
	parts := strings.SplitN(s, "-", 2)
	if len(parts) == 1 {
		// single page number
		n := 0
		if _, err := fmt.Sscanf(parts[0], "%d", &n); err != nil || n < 1 {
			return ops.PageRange{}, fmt.Errorf("expected page number, got %q", s)
		}
		return ops.PageRange{From: n, To: n}, nil
	}
	pr := ops.PageRange{From: 1, To: -1}
	if parts[0] != "" {
		if _, err := fmt.Sscanf(parts[0], "%d", &pr.From); err != nil {
			return pr, fmt.Errorf("invalid from-page %q", parts[0])
		}
	}
	if parts[1] != "" {
		if _, err := fmt.Sscanf(parts[1], "%d", &pr.To); err != nil {
			return pr, fmt.Errorf("invalid to-page %q", parts[1])
		}
	}
	return pr, nil
}
