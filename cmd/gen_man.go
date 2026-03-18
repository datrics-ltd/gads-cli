package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var genManCmd = &cobra.Command{
	Use:    "gen-man <dir>",
	Short:  "Generate man pages for all gads commands",
	Long:   `Generate man pages for all gads commands and write them to the specified directory.`,
	Args:   cobra.ExactArgs(1),
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := args[0]
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating output directory: %w", err)
		}

		absDir, err := filepath.Abs(dir)
		if err != nil {
			return fmt.Errorf("resolving directory path: %w", err)
		}

		header := &doc.GenManHeader{
			Title:   "GADS",
			Section: "1",
		}

		if err := doc.GenManTree(rootCmd, header, absDir); err != nil {
			return fmt.Errorf("generating man pages: %w", err)
		}

		fmt.Fprintf(os.Stderr, "Man pages written to: %s\n", absDir)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(genManCmd)
}
