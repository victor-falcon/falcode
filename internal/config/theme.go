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

// LoadTheme loads a theme by name for the given color scheme ("dark" or "light").
//
// Resolution order:
//  1. name == "" || "default"  →  embedded themes/default.json
//  2. custom name              →  ~/.config/falcode/themes/<name>.json
//     with any missing tokens falling back to the embedded default values
//  3. file not found           →  embedded themes/default.json
//  4. file invalid             →  error returned to the caller
func LoadTheme(name, scheme string) (*ThemeColors, error) {
	// Always parse the embedded default first — it is the single source of truth
	// for default values and the fallback for partial custom themes.
	defColors, err := parseThemeFile(defaultThemeData, scheme, nil)
	if err != nil {
		return nil, fmt.Errorf("parsing embedded default theme: %w", err)
	}

	if name == "" || name == "default" {
		return defColors, nil
	}

	p := filepath.Join(os.Getenv("HOME"), ".config", "falcode", "themes", name+".json")
	raw, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			// Custom theme file not found; use the embedded default.
			return defColors, nil
		}
		return nil, fmt.Errorf("reading theme %s: %w", p, err)
	}

	colors, err := parseThemeFile(raw, scheme, defColors)
	if err != nil {
		return nil, fmt.Errorf("parsing theme %s: %w", p, err)
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
//
// fallback, when non-nil, provides values for any tokens absent from data.
// Pass nil when parsing the embedded default (no fallback needed).
func parseThemeFile(data []byte, scheme string, fallback *ThemeColors) (*ThemeColors, error) {
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

	// Fill any missing tokens from the fallback (the parsed embedded default).
	// When fallback is nil (i.e. we are parsing the embedded default itself),
	// fb is a zero-value struct and missing tokens resolve to "".
	var fb ThemeColors
	if fallback != nil {
		fb = *fallback
	}
	fill := func(v, fallback string) string {
		if v == "" {
			return fallback
		}
		return v
	}

	return &ThemeColors{
		WorkspaceActive:     fill(get("workspace_active"), fb.WorkspaceActive),
		WorkspaceInactive:   fill(get("workspace_inactive"), fb.WorkspaceInactive),
		WorkspaceActiveFg:   fill(get("workspace_active_fg"), fb.WorkspaceActiveFg),
		WorkspaceInactiveFg: fill(get("workspace_inactive_fg"), fb.WorkspaceInactiveFg),
		WorkspaceBarBg:      fill(get("workspace_bar_bg"), fb.WorkspaceBarBg),
		WorkspaceBorder:     fill(get("workspace_border"), fb.WorkspaceBorder),
		PrefixIndicatorFg:   fill(get("prefix_indicator_fg"), fb.PrefixIndicatorFg),
		StatusFg:            fill(get("status_fg"), fb.StatusFg),
		InnerActive:         fill(get("inner_active"), fb.InnerActive),
		InnerInactive:       fill(get("inner_inactive"), fb.InnerInactive),
		InnerActiveFg:       fill(get("inner_active_fg"), fb.InnerActiveFg),
		InnerInactiveFg:     fill(get("inner_inactive_fg"), fb.InnerInactiveFg),
		InnerBarBg:          fill(get("inner_bar_bg"), fb.InnerBarBg),
		InnerSeparator:      fill(get("inner_separator"), fb.InnerSeparator),
		NewTabBtnBg:         fill(get("new_tab_btn_bg"), fb.NewTabBtnBg),
		NewTabBtnFg:         fill(get("new_tab_btn_fg"), fb.NewTabBtnFg),
		NewWorkspaceBtnBg:   fill(get("new_workspace_btn_bg"), fb.NewWorkspaceBtnBg),
		NewWorkspaceBtnFg:   fill(get("new_workspace_btn_fg"), fb.NewWorkspaceBtnFg),
		SheetBg:             fill(get("sheet_bg"), fb.SheetBg),
		SheetBorder:         fill(get("sheet_border"), fb.SheetBorder),
		SheetTitle:          fill(get("sheet_title"), fb.SheetTitle),
		SheetKey:            fill(get("sheet_key"), fb.SheetKey),
		SheetDescription:    fill(get("sheet_description"), fb.SheetDescription),
		SheetGroup:          fill(get("sheet_group"), fb.SheetGroup),
		SheetSeparator:      fill(get("sheet_separator"), fb.SheetSeparator),
		FooterBg:            fill(get("footer_bg"), fb.FooterBg),
		AgentWorkingFg:      fill(get("agent_working_fg"), fb.AgentWorkingFg),
		AgentPermissionFg:   fill(get("agent_permission_fg"), fb.AgentPermissionFg),
		AgentQuestionFg:     fill(get("agent_question_fg"), fb.AgentQuestionFg),
		AgentDoneFg:         fill(get("agent_done_fg"), fb.AgentDoneFg),
	}, nil
}
