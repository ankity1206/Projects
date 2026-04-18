// cmd/pdfmaster/root.go
package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/yourname/pdfmaster/internal/bridge"
)

var (
	// Global flags
	flagVerbose bool

	// Styles
	styleBanner = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("63"))

	styleVersion = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	styleError = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("196"))
)

var rootCmd = &cobra.Command{
	Use:   "pdfmaster",
	Short: "High-performance offline PDF tool",
	Long: styleBanner.Render("PDFMaster") + "  " +
		styleVersion.Render("v1.0.0  ·  C++17+Go hybrid engine") +
		`

Standalone, offline PDF processing. No Python. No runtime. One binary.

Operations:
  info      Show document metadata
  view      Render a page (outputs PNG)
  text      Extract text from pages
  merge     Combine multiple PDFs into one
  compress  Reduce PDF file size
  split     Split a PDF into separate files

Examples:
  pdfmaster info    document.pdf
  pdfmaster merge   -o merged.pdf a.pdf b.pdf c.pdf
  pdfmaster compress -l medium   -o out.pdf    input.pdf
  pdfmaster split   --mode pages -o ./pages/   input.pdf
  pdfmaster text    -o out.txt   input.pdf`,

	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Initialise the C++ engine once before any subcommand runs
		return bridge.Init()
	},

	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		bridge.Shutdown()
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false,
		"Verbose output")
	rootCmd.Version = "1.0.0 (engine: " + bridge.Version() + ")"
}

func errorf(format string, args ...any) {
	fmt.Fprintln(os.Stderr, styleError.Render("Error: ")+fmt.Sprintf(format, args...))
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
