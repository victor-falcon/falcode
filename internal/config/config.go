package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Tab represents a single inner tab configuration.
type Tab struct {
	Name    string `json:"name"`
	Command string `json:"command,omitempty"`
}

// Shell returns the command to run for this tab.
// If Command is empty, falls back to $SHELL or /bin/sh.
func (t *Tab) Shell() string {
	if t.Command != "" {
		return t.Command
	}
	if sh := os.Getenv("SHELL"); sh != "" {
		return sh
	}
	return "/bin/sh"
}

// IsInteractive returns true when this tab has no configured command
// and should run an interactive shell directly.
func (t *Tab) IsInteractive() bool {
	return t.Command == ""
}

// CloseTabButton controls when the × close button is shown on inner tabs.
type CloseTabButton string

const (
	CloseTabButtonAll   CloseTabButton = "all"
	CloseTabButtonFocus CloseTabButton = "focus"
	CloseTabButtonNone  CloseTabButton = "none"
)

// CloseWorkspaceButton controls when the × close button is shown on workspace tabs.
type CloseWorkspaceButton string

const (
	CloseWorkspaceButtonAll   CloseWorkspaceButton = "all"
	CloseWorkspaceButtonFocus CloseWorkspaceButton = "focus"
	CloseWorkspaceButtonNone  CloseWorkspaceButton = "none"
)

// UIConfig holds all UI display options.
type UIConfig struct {
	// Theme is the name of the color theme to use. Defaults to "default".
	Theme string `json:"theme,omitempty"`
	// ThemeScheme selects the light/dark variant: "system", "dark", or "light".
	ThemeScheme string `json:"theme_scheme,omitempty"`
	// HideFooter removes the hint footer row when true.
	HideFooter bool `json:"hide_footer,omitempty"`
	// NewTabButton shows a clickable + at the end of the inner tab bar.
	// Defaults to true when omitted.
	NewTabButton *bool `json:"new_tab_button,omitempty"`
	// NewWorkspaceButton shows a clickable + at the end of the workspace tab bar.
	// Defaults to true when omitted.
	NewWorkspaceButton *bool `json:"new_workspace_button,omitempty"`
	// CloseTabButton controls which extra tabs show a clickable × button.
	// Valid values: "all", "focus", "none". Defaults to "focus".
	CloseTabButton CloseTabButton `json:"close_tab_button,omitempty"`
	// CloseWorkspaceButton controls which workspace tabs show a clickable × button.
	// Valid values: "all", "focus", "none". Defaults to "none".
	CloseWorkspaceButton CloseWorkspaceButton `json:"close_workspace_button,omitempty"`
	// CompactTabs merges the workspace row and inner tab row into a single row.
	// When true, inactive workspaces flank the active workspace's inner tabs in one line.
	// Defaults to false.
	CompactTabs *bool `json:"compact_tabs,omitempty"`
}

// GetTheme returns the configured theme name, falling back to "default".
func (u *UIConfig) GetTheme() string {
	if u == nil || u.Theme == "" {
		return "default"
	}
	return u.Theme
}

// GetThemeScheme returns the configured scheme string (may be empty or "system",
// both of which are treated identically by the caller).
func (u *UIConfig) GetThemeScheme() string {
	if u == nil {
		return "system"
	}
	return u.ThemeScheme
}

// GetHideFooter returns whether the footer should be hidden.
func (u *UIConfig) GetHideFooter() bool {
	if u == nil {
		return false
	}
	return u.HideFooter
}

// GetNewTabButton returns whether the + new-tab button should be rendered.
// Defaults to true when the field is absent.
func (u *UIConfig) GetNewTabButton() bool {
	if u == nil || u.NewTabButton == nil {
		return true
	}
	return *u.NewTabButton
}

// GetNewWorkspaceButton returns whether the + new-workspace button should be
// rendered on the workspace tab bar. Defaults to true when the field is absent.
func (u *UIConfig) GetNewWorkspaceButton() bool {
	if u == nil || u.NewWorkspaceButton == nil {
		return true
	}
	return *u.NewWorkspaceButton
}

// GetCloseTabButton returns the resolved CloseTabButton value.
// Defaults to CloseTabButtonFocus when absent.
func (u *UIConfig) GetCloseTabButton() CloseTabButton {
	if u == nil || u.CloseTabButton == "" {
		return CloseTabButtonFocus
	}
	return u.CloseTabButton
}

// GetCloseWorkspaceButton returns the resolved CloseWorkspaceButton value.
// Defaults to CloseWorkspaceButtonNone when absent.
func (u *UIConfig) GetCloseWorkspaceButton() CloseWorkspaceButton {
	if u == nil || u.CloseWorkspaceButton == "" {
		return CloseWorkspaceButtonNone
	}
	return u.CloseWorkspaceButton
}

// GetCompactTabs returns whether the workspace and inner tab rows should be
// merged into a single row. Defaults to false when absent.
func (u *UIConfig) GetCompactTabs() bool {
	if u == nil || u.CompactTabs == nil {
		return false
	}
	return *u.CompactTabs
}

// Config is the top-level application configuration.
type Config struct {
	Tabs     []*Tab          `json:"tabs"`
	UI       *UIConfig       `json:"ui,omitempty"`
	Keybinds *KeybindsConfig `json:"keybinds,omitempty"`
	// WorktreeScripts is an ordered list of paths relative to the newly created
	// worktree to search for a setup script. The first existing file is
	// executed after a new worktree is created. Defaults to
	// ["falcode.sh", "worktree.sh"].
	WorktreeScripts []string `json:"worktree_scripts,omitempty"`
}

// GetWorktreeScripts returns the configured worktree script search paths.
// Falls back to ["falcode.sh", "worktree.sh"] when the field is not set.
func (c *Config) GetWorktreeScripts() []string {
	if c != nil && len(c.WorktreeScripts) > 0 {
		return c.WorktreeScripts
	}
	return []string{"falcode.sh", "worktree.sh"}
}

// Load returns the Config using a 3-priority search:
//  1. <cwd>/falcode.json
//  2. ~/.config/falcode/config.json
//  3. Built-in defaults
func Load(cwd string) (*Config, error) {
	paths := []string{
		filepath.Join(cwd, "falcode.json"),
		filepath.Join(os.Getenv("HOME"), ".config", "falcode", "config.json"),
	}

	for _, p := range paths {
		cfg, err := loadFile(p)
		if err == nil {
			return cfg, nil
		}
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("reading config %s: %w", p, err)
		}
	}

	return DefaultConfig(), nil
}

func loadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if len(cfg.Tabs) == 0 {
		return nil, os.ErrNotExist
	}
	// Fill keybind defaults for any fields not specified in the file.
	if cfg.Keybinds == nil {
		cfg.Keybinds = DefaultKeybinds()
	} else if cfg.Keybinds.Prefix == "" {
		cfg.Keybinds.Prefix = "ctrl+b"
	}
	return &cfg, nil
}
