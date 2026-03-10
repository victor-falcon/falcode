package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	_ "embed"
)

//go:embed themes/default.json
var defaultThemeData []byte

// ThemeColors holds all resolved colour tokens for the UI.
// All values are hex strings like "#7B61FF".
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

// ThemeColorPair holds a color for each color scheme.
type ThemeColorPair struct {
	Dark  string `json:"dark"`
	Light string `json:"light"`
}

// themeFile is the on-disk (and embedded) JSON format.
// Each token carries both a dark and light value; values may reference defs aliases.
type themeFile struct {
	Defs  map[string]string         `json:"defs"`
	Theme map[string]ThemeColorPair `json:"theme"`
}

// DetectSystemScheme queries macOS for the current appearance setting.
// Returns "dark" when dark mode is active, "light" otherwise.
func DetectSystemScheme() string {
	out, err := exec.Command("defaults", "read", "-g", "AppleInterfaceStyle").Output()
	if err != nil {
		// Command errors (exit 1) when the key is absent, which means light mode.
		return "light"
	}
	if strings.TrimSpace(string(out)) == "Dark" {
		return "dark"
	}
	return "light"
}

// DefaultTheme returns a hardcoded fallback theme for the given scheme.
// This is only used when the embedded default.json cannot be parsed.
func DefaultTheme(scheme string) *ThemeColors {
	if scheme == "light" {
		return &ThemeColors{
			WorkspaceActive:     "#5B41DF",
			WorkspaceInactive:   "#F0F0F8",
			WorkspaceActiveFg:   "#FFFFFF",
			WorkspaceInactiveFg: "#666666",
			WorkspaceBarBg:      "#E2E2F0",
			WorkspaceBorder:     "#5B41DF",
			PrefixIndicatorFg:   "#A07800",
			StatusFg:            "#CC2222",
			InnerActive:         "#5B41DF",
			InnerInactive:       "#F0F0F8",
			InnerActiveFg:       "#FFFFFF",
			InnerInactiveFg:     "#666666",
			InnerBarBg:          "#ECECF8",
			InnerSeparator:      "#C8C8E8",
			SheetBg:             "#F8F8FF",
			SheetBorder:         "#5B41DF",
			SheetTitle:          "#A07800",
			SheetKey:            "#5B41DF",
			SheetDescription:    "#444444",
			SheetGroup:          "#A07800",
			SheetSeparator:      "#C8C8E8",
		}
	}
	// dark (default)
	return &ThemeColors{
		WorkspaceActive:     "#7B61FF",
		WorkspaceInactive:   "#1E1E2E",
		WorkspaceActiveFg:   "#FFFFFF",
		WorkspaceInactiveFg: "#888888",
		WorkspaceBarBg:      "#13131F",
		WorkspaceBorder:     "#7B61FF",
		PrefixIndicatorFg:   "#FFD700",
		StatusFg:            "#FF6B6B",
		InnerActive:         "#7B61FF",
		InnerInactive:       "#1E1E2E",
		InnerActiveFg:       "#FFFFFF",
		InnerInactiveFg:     "#888888",
		InnerBarBg:          "#0D0D1A",
		InnerSeparator:      "#333355",
		SheetBg:             "#1A1A2E",
		SheetBorder:         "#7B61FF",
		SheetTitle:          "#FFD700",
		SheetKey:            "#7B61FF",
		SheetDescription:    "#CCCCCC",
		SheetGroup:          "#FFD700",
		SheetSeparator:      "#333355",
	}
}

// LoadTheme loads a theme by name for the given color scheme ("dark" or "light").
//
// Resolution order:
//  1. name == "" || "default"  →  embedded themes/default.json
//  2. custom name              →  ~/.config/falcode/themes/<name>.json
//  3. file missing or invalid  →  embedded themes/default.json
//  4. embedded invalid         →  hardcoded DefaultTheme(scheme)
func LoadTheme(name, scheme string) (*ThemeColors, error) {
	var data []byte

	if name == "" || name == "default" {
		data = defaultThemeData
	} else {
		p := filepath.Join(os.Getenv("HOME"), ".config", "falcode", "themes", name+".json")
		raw, err := os.ReadFile(p)
		if err != nil {
			if os.IsNotExist(err) {
				// Fall back to the embedded default.
				data = defaultThemeData
			} else {
				return nil, fmt.Errorf("reading theme %s: %w", p, err)
			}
		} else {
			data = raw
		}
	}

	colors, err := parseThemeFile(data, scheme)
	if err != nil {
		// Embedded JSON should never be invalid; hardcoded fallback is the last resort.
		return DefaultTheme(scheme), nil
	}
	return colors, nil
}

// EmbeddedDefaultThemeData returns the raw bytes of the embedded default theme
// JSON. Used by the write-default-theme command.
func EmbeddedDefaultThemeData() []byte {
	cp := make([]byte, len(defaultThemeData))
	copy(cp, defaultThemeData)
	return cp
}

// parseThemeFile unmarshals a theme JSON file and resolves it into ThemeColors
// for the requested scheme ("dark" or "light").
func parseThemeFile(data []byte, scheme string) (*ThemeColors, error) {
	var tf themeFile
	if err := json.Unmarshal(data, &tf); err != nil {
		return nil, fmt.Errorf("parsing theme JSON: %w", err)
	}

	// resolve returns the hex value for a token, expanding defs aliases.
	resolve := func(pair ThemeColorPair) string {
		var raw string
		if scheme == "light" {
			raw = pair.Light
		} else {
			raw = pair.Dark
		}
		if v, ok := tf.Defs[raw]; ok {
			return v
		}
		return raw
	}

	get := func(key string) string {
		pair, ok := tf.Theme[key]
		if !ok {
			return ""
		}
		return resolve(pair)
	}

	// Fill any missing fields from the hardcoded fallback so partial theme
	// files still produce a complete ThemeColors.
	def := DefaultTheme(scheme)
	fill := func(v, fallback string) string {
		if v == "" {
			return fallback
		}
		return v
	}

	return &ThemeColors{
		WorkspaceActive:     fill(get("workspace_active"), def.WorkspaceActive),
		WorkspaceInactive:   fill(get("workspace_inactive"), def.WorkspaceInactive),
		WorkspaceActiveFg:   fill(get("workspace_active_fg"), def.WorkspaceActiveFg),
		WorkspaceInactiveFg: fill(get("workspace_inactive_fg"), def.WorkspaceInactiveFg),
		WorkspaceBarBg:      fill(get("workspace_bar_bg"), def.WorkspaceBarBg),
		WorkspaceBorder:     fill(get("workspace_border"), def.WorkspaceBorder),
		PrefixIndicatorFg:   fill(get("prefix_indicator_fg"), def.PrefixIndicatorFg),
		StatusFg:            fill(get("status_fg"), def.StatusFg),
		InnerActive:         fill(get("inner_active"), def.InnerActive),
		InnerInactive:       fill(get("inner_inactive"), def.InnerInactive),
		InnerActiveFg:       fill(get("inner_active_fg"), def.InnerActiveFg),
		InnerInactiveFg:     fill(get("inner_inactive_fg"), def.InnerInactiveFg),
		InnerBarBg:          fill(get("inner_bar_bg"), def.InnerBarBg),
		InnerSeparator:      fill(get("inner_separator"), def.InnerSeparator),
		SheetBg:             fill(get("sheet_bg"), def.SheetBg),
		SheetBorder:         fill(get("sheet_border"), def.SheetBorder),
		SheetTitle:          fill(get("sheet_title"), def.SheetTitle),
		SheetKey:            fill(get("sheet_key"), def.SheetKey),
		SheetDescription:    fill(get("sheet_description"), def.SheetDescription),
		SheetGroup:          fill(get("sheet_group"), def.SheetGroup),
		SheetSeparator:      fill(get("sheet_separator"), def.SheetSeparator),
	}, nil
}
