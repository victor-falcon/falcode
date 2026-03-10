package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/victor-falcon/falcode/internal/config"
)

var writeDefaultThemeForceFlag bool

var writeDefaultThemeCmd = &cobra.Command{
	Use:   "write-default-theme <name>",
	Short: "Write the default theme to ~/.config/falcode/themes/<name>.json",
	Long: `Copies the built-in default theme to ~/.config/falcode/themes/<name>.json
so you can inspect and customize it. The <name> argument becomes both the
filename and the value you set for "theme" in your falcode config.

Example:
  falcode write-default-theme my-purple
  # then in falcode.json: { "theme": "my-purple" }

Use --force to overwrite an existing file.`,
	Args: cobra.ExactArgs(1),
	RunE: runWriteDefaultTheme,
}

func init() {
	writeDefaultThemeCmd.Flags().BoolVarP(&writeDefaultThemeForceFlag, "force", "f", false, "overwrite existing theme file")
}

func runWriteDefaultTheme(_ *cobra.Command, args []string) error {
	name := args[0]

	dir := filepath.Join(os.Getenv("HOME"), ".config", "falcode", "themes")
	dest := filepath.Join(dir, name+".json")

	if _, err := os.Stat(dest); err == nil && !writeDefaultThemeForceFlag {
		return fmt.Errorf("%s already exists; use --force to overwrite", dest)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	data := config.EmbeddedDefaultThemeData()
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", dest, err)
	}

	fmt.Printf("wrote %s\n", dest)
	fmt.Printf("set \"theme\": %q in your falcode config to use it\n", name)
	return nil
}
