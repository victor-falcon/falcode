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
	// ShowWorkspaceNumbers prefixes each workspace tab label with its 1-based
	// index number (e.g. "1 main", "2 feature-x"). Matches the default 1-9
	// go_to_workspace keybinds. Defaults to true.
	ShowWorkspaceNumbers *bool `json:"show_workspace_numbers,omitempty"`
	// ShowTabNumbers prefixes each inner tab label with its keybind letter
	// (e.g. "a editor", "b console"). Matches the default a-z go_to_tab
	// keybinds. Defaults to true.
	ShowTabNumbers *bool `json:"show_tab_numbers,omitempty"`
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

// GetShowWorkspaceNumbers returns whether workspace tabs should display a
// numeric prefix that matches the default 1-9 go_to_workspace keybinds.
// Defaults to true when absent.
func (u *UIConfig) GetShowWorkspaceNumbers() bool {
	if u == nil || u.ShowWorkspaceNumbers == nil {
		return true
	}
	return *u.ShowWorkspaceNumbers
}

// GetShowTabNumbers returns whether inner tabs should display a letter prefix
// that matches the default a-z go_to_tab keybinds. Defaults to true when absent.
func (u *UIConfig) GetShowTabNumbers() bool {
	if u == nil || u.ShowTabNumbers == nil {
		return true
	}
	return *u.ShowTabNumbers
}

// Config is the top-level application configuration.
type Config struct {
	// Tabs are the inner tabs shown inside every workspace. When omitted, the
	// next config level (global or built-in defaults) provides them.
	Tabs []*Tab `json:"tabs,omitempty"`
	// AppendedTabs are additional tabs appended after the resolved Tabs list.
	// Useful in a repo-level falcode.json to add project-specific tabs on top
	// of the globally configured ones.
	AppendedTabs []*Tab          `json:"appended_tabs,omitempty"`
	UI           *UIConfig       `json:"ui,omitempty"`
	Keybinds     *KeybindsConfig `json:"keybinds,omitempty"`
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

// Load reads and merges configs from two sources:
//  1. ~/.config/falcode/config.json  (global)
//  2. <cwd>/falcode.json             (repo-level, layered on top)
//
// Both files are optional. Missing files and files without tabs are valid —
// missing tabs fall back to the next level, ultimately to DefaultConfig().
//
// Merge rules:
//   - tabs:             repo wins if non-empty, otherwise global/default
//   - appended_tabs:    always appended after the resolved tabs
//   - ui:               field-by-field — repo non-zero values override global
//   - keybinds:         repo wins if non-nil, otherwise global/default
//   - worktree_scripts: repo wins if non-empty, otherwise global/default
func Load(cwd string) (*Config, error) {
	globalPath := filepath.Join(os.Getenv("HOME"), ".config", "falcode", "config.json")
	repoPath := filepath.Join(cwd, "falcode.json")

	globalCfg, err := loadFile(globalPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading config %s: %w", globalPath, err)
	}

	repoCfg, err := loadFile(repoPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading config %s: %w", repoPath, err)
	}

	// Determine the base: global config if available, otherwise built-in defaults.
	base := globalCfg
	if base == nil {
		base = DefaultConfig()
	}

	// No repo config — use the base, ensuring keybind defaults are applied.
	if repoCfg == nil {
		if base.Keybinds == nil {
			base.Keybinds = DefaultKeybinds()
		} else if base.Keybinds.Prefix == "" {
			base.Keybinds.Prefix = "ctrl+b"
		}
		return base, nil
	}

	// Merge repo config on top of base.
	result := &Config{}

	// tabs: repo wins when non-empty.
	if len(repoCfg.Tabs) > 0 {
		result.Tabs = repoCfg.Tabs
	} else {
		result.Tabs = base.Tabs
	}

	// appended_tabs: always appended after the resolved tabs.
	result.Tabs = append(result.Tabs, repoCfg.AppendedTabs...)

	// ui: field-by-field merge — repo non-zero values win.
	result.UI = mergeUI(base.UI, repoCfg.UI)

	// keybinds: repo wins when explicitly set.
	if repoCfg.Keybinds != nil {
		result.Keybinds = repoCfg.Keybinds
	} else {
		result.Keybinds = base.Keybinds
	}

	// worktree_scripts: repo wins when non-empty.
	if len(repoCfg.WorktreeScripts) > 0 {
		result.WorktreeScripts = repoCfg.WorktreeScripts
	} else {
		result.WorktreeScripts = base.WorktreeScripts
	}

	// Ensure keybind prefix default on the final resolved config.
	if result.Keybinds == nil {
		result.Keybinds = DefaultKeybinds()
	} else if result.Keybinds.Prefix == "" {
		result.Keybinds.Prefix = "ctrl+b"
	}

	return result, nil
}

// loadFile reads and parses a single config file. Returns os.ErrNotExist when
// the file is absent. Any parseable file is valid regardless of whether it
// defines tabs.
func loadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &cfg, nil
}

// mergeUI merges two UIConfig values field-by-field. Repo (override) non-zero
// values take precedence over the base. Either argument may be nil.
func mergeUI(base, override *UIConfig) *UIConfig {
	// Start from a copy of the base (or an empty config if base is nil).
	out := &UIConfig{}
	if base != nil {
		*out = *base
	}
	if override == nil {
		return out
	}

	if override.Theme != "" {
		out.Theme = override.Theme
	}
	if override.ThemeScheme != "" {
		out.ThemeScheme = override.ThemeScheme
	}
	if override.HideFooter {
		out.HideFooter = true
	}
	if override.NewTabButton != nil {
		out.NewTabButton = override.NewTabButton
	}
	if override.NewWorkspaceButton != nil {
		out.NewWorkspaceButton = override.NewWorkspaceButton
	}
	if override.CloseTabButton != "" {
		out.CloseTabButton = override.CloseTabButton
	}
	if override.CloseWorkspaceButton != "" {
		out.CloseWorkspaceButton = override.CloseWorkspaceButton
	}
	if override.CompactTabs != nil {
		out.CompactTabs = override.CompactTabs
	}
	if override.ShowWorkspaceNumbers != nil {
		out.ShowWorkspaceNumbers = override.ShowWorkspaceNumbers
	}
	if override.ShowTabNumbers != nil {
		out.ShowTabNumbers = override.ShowTabNumbers
	}

	return out
}
