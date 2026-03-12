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

	// New-tab / new-workspace buttons
	NewTabBtnBg       string `json:"new_tab_btn_bg"`
	NewTabBtnFg       string `json:"new_tab_btn_fg"`
	NewWorkspaceBtnBg string `json:"new_workspace_btn_bg"`
	NewWorkspaceBtnFg string `json:"new_workspace_btn_fg"`

	// Which-key sheet
	SheetBg          string `json:"sheet_bg"`
	SheetBorder      string `json:"sheet_border"`
	SheetTitle       string `json:"sheet_title"`
	SheetKey         string `json:"sheet_key"`
	SheetDescription string `json:"sheet_description"`
	SheetGroup       string `json:"sheet_group"`
	SheetSeparator   string `json:"sheet_separator"`

	// Footer bar
	FooterBg string `json:"footer_bg"`

	// Agent status icons shown in workspace tabs
	AgentWorkingFg    string `json:"agent_working_fg"`
	AgentPermissionFg string `json:"agent_permission_fg"`
	AgentQuestionFg   string `json:"agent_question_fg"`
	AgentDoneFg       string `json:"agent_done_fg"`
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
			WorkspaceInactive:   "#9080D8",
			WorkspaceActiveFg:   "#FFFFFF",
			WorkspaceInactiveFg: "#FFFFFF",
			WorkspaceBarBg:      "transparent",
			WorkspaceBorder:     "#5B41DF",
			PrefixIndicatorFg:   "#A07800",
			StatusFg:            "#CC2222",
			InnerActive:         "#7B61FF",
			InnerInactive:       "#EAE6FF",
			InnerActiveFg:       "#FFFFFF",
			InnerInactiveFg:     "#4A38B8",
			InnerBarBg:          "transparent",
			InnerSeparator:      "#C8C8E8",
			NewTabBtnBg:         "#F0F0F8",
			NewTabBtnFg:         "#666666",
			NewWorkspaceBtnBg:   "#F0F0F8",
			NewWorkspaceBtnFg:   "#666666",
			SheetBg:             "#F8F8FF",
			SheetBorder:         "#5B41DF",
			SheetTitle:          "#A07800",
			SheetKey:            "#5B41DF",
			SheetDescription:    "#444444",
			SheetGroup:          "#A07800",
			SheetSeparator:      "#C8C8E8",
			FooterBg:            "transparent",
			AgentWorkingFg:      "#00AA44",
			AgentPermissionFg:   "#CC2222",
			AgentQuestionFg:     "#C05800",
			AgentDoneFg:         "#00AA44",
		}
	}
	// dark (default)
	return &ThemeColors{
		WorkspaceActive:     "#7B61FF",
		WorkspaceInactive:   "#2E2480",
		WorkspaceActiveFg:   "#FFFFFF",
		WorkspaceInactiveFg: "#B0A8F0",
		WorkspaceBarBg:      "transparent",
		WorkspaceBorder:     "#7B61FF",
		PrefixIndicatorFg:   "#FFD700",
		StatusFg:            "#FF6B6B",
		InnerActive:         "#5B4CC0",
		InnerInactive:       "#CBC5FF",
		InnerActiveFg:       "#FFFFFF",
		InnerInactiveFg:     "#3D2E8C",
		InnerBarBg:          "transparent",
		InnerSeparator:      "#333355",
		NewTabBtnBg:         "#1E1E2E",
		NewTabBtnFg:         "#888888",
		NewWorkspaceBtnBg:   "#1E1E2E",
		NewWorkspaceBtnFg:   "#888888",
		SheetBg:             "#1A1A2E",
		SheetBorder:         "#7B61FF",
		SheetTitle:          "#FFD700",
		SheetKey:            "#7B61FF",
		SheetDescription:    "#CCCCCC",
		SheetGroup:          "#FFD700",
		SheetSeparator:      "#333355",
		FooterBg:            "transparent",
		AgentWorkingFg:      "#5FFF87",
		AgentPermissionFg:   "#FF5F5F",
		AgentQuestionFg:     "#FF8C00",
		AgentDoneFg:         "#5FFF87",
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
		NewTabBtnBg:         fill(get("new_tab_btn_bg"), def.NewTabBtnBg),
		NewTabBtnFg:         fill(get("new_tab_btn_fg"), def.NewTabBtnFg),
		NewWorkspaceBtnBg:   fill(get("new_workspace_btn_bg"), def.NewWorkspaceBtnBg),
		NewWorkspaceBtnFg:   fill(get("new_workspace_btn_fg"), def.NewWorkspaceBtnFg),
		SheetBg:             fill(get("sheet_bg"), def.SheetBg),
		SheetBorder:         fill(get("sheet_border"), def.SheetBorder),
		SheetTitle:          fill(get("sheet_title"), def.SheetTitle),
		SheetKey:            fill(get("sheet_key"), def.SheetKey),
		SheetDescription:    fill(get("sheet_description"), def.SheetDescription),
		SheetGroup:          fill(get("sheet_group"), def.SheetGroup),
		SheetSeparator:      fill(get("sheet_separator"), def.SheetSeparator),
		FooterBg:            fill(get("footer_bg"), def.FooterBg),
		AgentWorkingFg:      fill(get("agent_working_fg"), def.AgentWorkingFg),
		AgentPermissionFg:   fill(get("agent_permission_fg"), def.AgentPermissionFg),
		AgentQuestionFg:     fill(get("agent_question_fg"), def.AgentQuestionFg),
		AgentDoneFg:         fill(get("agent_done_fg"), def.AgentDoneFg),
	}, nil
}
