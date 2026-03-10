package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ThemeColors holds all colour tokens for the UI.
// Colours are hex strings like "#7B61FF".
type ThemeColors struct {
	// Workspace (outer) tab bar
	WorkspaceActive     string `json:"workspace_active"`
	WorkspaceInactive   string `json:"workspace_inactive"`
	WorkspaceActiveFg   string `json:"workspace_active_fg"`
	WorkspaceInactiveFg string `json:"workspace_inactive_fg"`
	WorkspaceBarBg      string `json:"workspace_bar_bg"`
	WorkspaceBorder     string `json:"workspace_border"`
	PrefixIndicatorFg   string `json:"prefix_indicator_fg"`
	StatusFg            string `json:"status_fg"`

	// Inner tab bar
	InnerActive     string `json:"inner_active"`
	InnerInactive   string `json:"inner_inactive"`
	InnerActiveFg   string `json:"inner_active_fg"`
	InnerInactiveFg string `json:"inner_inactive_fg"`
	InnerBarBg      string `json:"inner_bar_bg"`
	InnerSeparator  string `json:"inner_separator"`

	// Which-key sheet
	SheetBg          string `json:"sheet_bg"`
	SheetBorder      string `json:"sheet_border"`
	SheetTitle       string `json:"sheet_title"`
	SheetKey         string `json:"sheet_key"`
	SheetDescription string `json:"sheet_description"`
	SheetGroup       string `json:"sheet_group"`
	SheetSeparator   string `json:"sheet_separator"`
}

// themeFile is the on-disk format — allows named colour aliases.
type themeFile struct {
	Defs   map[string]string `json:"defs"`
	Colors ThemeColors       `json:"colors"`
}

// DefaultTheme returns the built-in deep-purple dark theme.
func DefaultTheme() *ThemeColors {
	return &ThemeColors{
		WorkspaceActive:     "#7B61FF",
		WorkspaceInactive:   "#1E1E2E",
		WorkspaceActiveFg:   "#FFFFFF",
		WorkspaceInactiveFg: "#888888",
		WorkspaceBarBg:      "#13131F",
		WorkspaceBorder:     "#7B61FF",
		PrefixIndicatorFg:   "#FFD700",
		StatusFg:            "#FF6B6B",

		InnerActive:     "#7B61FF",
		InnerInactive:   "#1E1E2E",
		InnerActiveFg:   "#FFFFFF",
		InnerInactiveFg: "#888888",
		InnerBarBg:      "#0D0D1A",
		InnerSeparator:  "#333355",

		SheetBg:          "#1A1A2E",
		SheetBorder:      "#7B61FF",
		SheetTitle:       "#FFD700",
		SheetKey:         "#7B61FF",
		SheetDescription: "#CCCCCC",
		SheetGroup:       "#FFD700",
		SheetSeparator:   "#333355",
	}
}

// LoadTheme loads ~/.config/falcode/themes/<name>.json.
// Falls back to DefaultTheme() if the file does not exist.
func LoadTheme(name string) (*ThemeColors, error) {
	if name == "" || name == "default" {
		return DefaultTheme(), nil
	}
	p := filepath.Join(os.Getenv("HOME"), ".config", "falcode", "themes", name+".json")
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultTheme(), nil
		}
		return nil, fmt.Errorf("reading theme %s: %w", p, err)
	}
	var tf themeFile
	if err := json.Unmarshal(data, &tf); err != nil {
		return nil, fmt.Errorf("parsing theme %s: %w", p, err)
	}

	// Resolve named colour aliases.
	resolve := func(c string) string {
		if v, ok := tf.Defs[c]; ok {
			return v
		}
		return c
	}

	t := tf.Colors
	t.WorkspaceActive = resolve(t.WorkspaceActive)
	t.WorkspaceInactive = resolve(t.WorkspaceInactive)
	t.WorkspaceActiveFg = resolve(t.WorkspaceActiveFg)
	t.WorkspaceInactiveFg = resolve(t.WorkspaceInactiveFg)
	t.WorkspaceBarBg = resolve(t.WorkspaceBarBg)
	t.WorkspaceBorder = resolve(t.WorkspaceBorder)
	t.PrefixIndicatorFg = resolve(t.PrefixIndicatorFg)
	t.StatusFg = resolve(t.StatusFg)
	t.InnerActive = resolve(t.InnerActive)
	t.InnerInactive = resolve(t.InnerInactive)
	t.InnerActiveFg = resolve(t.InnerActiveFg)
	t.InnerInactiveFg = resolve(t.InnerInactiveFg)
	t.InnerBarBg = resolve(t.InnerBarBg)
	t.InnerSeparator = resolve(t.InnerSeparator)
	t.SheetBg = resolve(t.SheetBg)
	t.SheetBorder = resolve(t.SheetBorder)
	t.SheetTitle = resolve(t.SheetTitle)
	t.SheetKey = resolve(t.SheetKey)
	t.SheetDescription = resolve(t.SheetDescription)
	t.SheetGroup = resolve(t.SheetGroup)
	t.SheetSeparator = resolve(t.SheetSeparator)

	// Fill any missing fields from defaults.
	def := DefaultTheme()
	fill := func(s, d string) string {
		if s == "" {
			return d
		}
		return s
	}
	t.WorkspaceActive = fill(t.WorkspaceActive, def.WorkspaceActive)
	t.WorkspaceInactive = fill(t.WorkspaceInactive, def.WorkspaceInactive)
	t.WorkspaceActiveFg = fill(t.WorkspaceActiveFg, def.WorkspaceActiveFg)
	t.WorkspaceInactiveFg = fill(t.WorkspaceInactiveFg, def.WorkspaceInactiveFg)
	t.WorkspaceBarBg = fill(t.WorkspaceBarBg, def.WorkspaceBarBg)
	t.WorkspaceBorder = fill(t.WorkspaceBorder, def.WorkspaceBorder)
	t.PrefixIndicatorFg = fill(t.PrefixIndicatorFg, def.PrefixIndicatorFg)
	t.StatusFg = fill(t.StatusFg, def.StatusFg)
	t.InnerActive = fill(t.InnerActive, def.InnerActive)
	t.InnerInactive = fill(t.InnerInactive, def.InnerInactive)
	t.InnerActiveFg = fill(t.InnerActiveFg, def.InnerActiveFg)
	t.InnerInactiveFg = fill(t.InnerInactiveFg, def.InnerInactiveFg)
	t.InnerBarBg = fill(t.InnerBarBg, def.InnerBarBg)
	t.InnerSeparator = fill(t.InnerSeparator, def.InnerSeparator)
	t.SheetBg = fill(t.SheetBg, def.SheetBg)
	t.SheetBorder = fill(t.SheetBorder, def.SheetBorder)
	t.SheetTitle = fill(t.SheetTitle, def.SheetTitle)
	t.SheetKey = fill(t.SheetKey, def.SheetKey)
	t.SheetDescription = fill(t.SheetDescription, def.SheetDescription)
	t.SheetGroup = fill(t.SheetGroup, def.SheetGroup)
	t.SheetSeparator = fill(t.SheetSeparator, def.SheetSeparator)

	return &t, nil
}
