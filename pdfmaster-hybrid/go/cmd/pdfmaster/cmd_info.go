// cmd/pdfmaster/cmd_info.go
package main

import (
	"github.com/spf13/cobra"
	"github.com/yourname/pdfmaster/internal/ops"
)

var infoCmd = &cobra.Command{
	Use:   "info <file.pdf>",
	Short: "Show PDF document metadata",
	Long: `Display metadata for a PDF file:
title, author, page count, PDF version, file size, encryption status, and more.`,
	Args:    cobra.ExactArgs(1),
	Example: "  pdfmaster info document.pdf",
	RunE: func(cmd *cobra.Command, args []string) error {
		return ops.PrintInfo(args[0])
	},
}

func init() {
	rootCmd.AddCommand(infoCmd)
}
