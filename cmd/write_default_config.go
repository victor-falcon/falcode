package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/victor-falcon/falcode/internal/config"
)

var forceFlag bool

var writeDefaultConfigCmd = &cobra.Command{
	Use:   "write-default-config",
	Short: "Write the default config.json to ~/.config/falcode/",
	Long: `Writes the built-in default configuration to ~/.config/falcode/config.json
so you can inspect and customize it. Use --force to overwrite an existing file.`,
	RunE: runWriteDefaultConfig,
}

func init() {
	writeDefaultConfigCmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "overwrite existing config file")
}

func runWriteDefaultConfig(_ *cobra.Command, _ []string) error {
	dir := filepath.Join(os.Getenv("HOME"), ".config", "falcode")
	dest := filepath.Join(dir, "config.json")

	if _, err := os.Stat(dest); err == nil && !forceFlag {
		return fmt.Errorf("%s already exists; use --force to overwrite", dest)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(config.DefaultConfig(), "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling default config: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", dest, err)
	}

	fmt.Printf("wrote %s\n", dest)
	return nil
}
