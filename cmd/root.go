package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
	"github.com/victor-falcon/falcode/internal/config"
	"github.com/victor-falcon/falcode/internal/git"
	"github.com/victor-falcon/falcode/internal/ui"
)

// Version is the current build version. It is overridden at build time with:
//
//	go build -ldflags "-X 'github.com/victor-falcon/falcode/cmd.Version=1.2.3'"
var Version = "dev"

var rootCmd = &cobra.Command{
	Use:   "falcode",
	Short: "Terminal multiplexer for multi-agent git worktree workflows",
	Long: `falcode opens a tab-based terminal UI where each tab is a git worktree.
Within each workspace, configurable inner tabs run tools like opencode, lazygit,
or an interactive shell side-by-side.`,
	RunE: run,
}

func Execute() {
	rootCmd.AddCommand(writeDefaultConfigCmd)
	rootCmd.AddCommand(writeDefaultThemeCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(_ *cobra.Command, _ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	// --- Load configuration ---
	cfg, err := config.Load(cwd)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Resolve the color scheme before loading the theme.
	// "system" (and empty) means detect the OS appearance at boot time.
	scheme := cfg.UI.GetThemeScheme()
	if scheme == "" || scheme == "system" {
		scheme = config.DetectSystemScheme()
	}

	theme, err := config.LoadTheme(cfg.UI.GetTheme(), scheme)
	if err != nil {
		return fmt.Errorf("loading theme: %w", err)
	}

	// --- Discover git worktrees ---
	worktrees, err := git.Discover(cwd)
	if err != nil {
		return fmt.Errorf(
			"discovering worktrees (is this a git repo?): %w", err,
		)
	}

	// --- Query terminal size ---
	cols, rows, err := term.GetSize(os.Stdout.Fd())
	if err != nil {
		cols, rows = 80, 24 // safe fallback
	}

	// --- Build model and program ---
	model := ui.New(cfg, cfg.Keybinds, theme, worktrees, cols, rows, Version)

	prog := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseAllMotion(),
		// Don't intercept Ctrl+C so it can pass through to the active PTY.
		tea.WithoutSignalHandler(),
	)

	// Give the model a reference to prog.Send so that background goroutines
	// (PTY readers) can dispatch messages into the event loop.
	model.SetSend(prog.Send)

	// Eagerly start the initial (first) pane.
	model.StartAll()

	// Restore the previous keyboard mode when the program exits.
	// The Kitty keyboard protocol enable (\x1b[>1u) is sent from Init() after
	// bubbletea has finished its own terminal setup, so it is not reset by the
	// alt-screen or mouse initialisation sequences.
	defer os.Stdout.WriteString("\x1b[<u") //nolint:errcheck

	// --- Run ---
	if _, err := prog.Run(); err != nil {
		return fmt.Errorf("program error: %w", err)
	}

	// Gracefully terminate all PTY processes.
	model.StopAll()
	return nil
}
