package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseThemeFileResolvesAliasesAndFallbacks(t *testing.T) {
	data := []byte(`{
		"defs": {"accent": "#111111"},
		"theme": {
			"workspace_active": {"dark": "accent", "light": "#222222"},
			"workspace_inactive": {"dark": "#333333", "light": "#444444"}
		}
	}`)
	fallback := &ThemeColors{WorkspaceInactiveFg: "#abcdef"}

	dark, err := parseThemeFile(data, "dark", fallback)
	if err != nil {
		t.Fatalf("parseThemeFile(dark) error = %v", err)
	}
	if dark.WorkspaceActive != "#111111" {
		t.Fatalf("WorkspaceActive = %q, want %q", dark.WorkspaceActive, "#111111")
	}
	if dark.WorkspaceInactive != "#333333" {
		t.Fatalf("WorkspaceInactive = %q, want %q", dark.WorkspaceInactive, "#333333")
	}
	if dark.WorkspaceInactiveFg != "#abcdef" {
		t.Fatalf("WorkspaceInactiveFg = %q, want %q", dark.WorkspaceInactiveFg, "#abcdef")
	}

	light, err := parseThemeFile(data, "light", fallback)
	if err != nil {
		t.Fatalf("parseThemeFile(light) error = %v", err)
	}
	if light.WorkspaceActive != "#222222" {
		t.Fatalf("WorkspaceActive = %q, want %q", light.WorkspaceActive, "#222222")
	}
	if light.WorkspaceInactive != "#444444" {
		t.Fatalf("WorkspaceInactive = %q, want %q", light.WorkspaceInactive, "#444444")
	}
}

func TestParseThemeFileInvalidJSON(t *testing.T) {
	if _, err := parseThemeFile([]byte(`{"theme":`), "dark", nil); err == nil {
		t.Fatalf("parseThemeFile() error = nil, want error")
	}
}

func TestLoadThemeDefaultAndMissingCustomFallback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	def, err := LoadTheme("default", "dark")
	if err != nil {
		t.Fatalf("LoadTheme(default) error = %v", err)
	}
	missing, err := LoadTheme("does-not-exist", "dark")
	if err != nil {
		t.Fatalf("LoadTheme(missing) error = %v", err)
	}

	if def.WorkspaceActive != missing.WorkspaceActive {
		t.Fatalf("WorkspaceActive mismatch: default=%q missing=%q", def.WorkspaceActive, missing.WorkspaceActive)
	}
	if def.FooterBg != missing.FooterBg {
		t.Fatalf("FooterBg mismatch: default=%q missing=%q", def.FooterBg, missing.FooterBg)
	}
}

func TestLoadThemePartialCustomOverridesAndFallsBack(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	defaultTheme, err := LoadTheme("default", "dark")
	if err != nil {
		t.Fatalf("LoadTheme(default) error = %v", err)
	}

	themePath := filepath.Join(home, ".config", "falcode", "themes", "custom.json")
	if err := os.MkdirAll(filepath.Dir(themePath), 0o755); err != nil {
		t.Fatalf("mkdir themes dir: %v", err)
	}
	if err := os.WriteFile(themePath, []byte(`{
		"theme": {
			"workspace_active": {"dark": "#123456", "light": "#654321"}
		}
	}`), 0o644); err != nil {
		t.Fatalf("write custom theme: %v", err)
	}

	custom, err := LoadTheme("custom", "dark")
	if err != nil {
		t.Fatalf("LoadTheme(custom) error = %v", err)
	}

	if custom.WorkspaceActive != "#123456" {
		t.Fatalf("WorkspaceActive = %q, want %q", custom.WorkspaceActive, "#123456")
	}
	if custom.WorkspaceInactive != defaultTheme.WorkspaceInactive {
		t.Fatalf("WorkspaceInactive = %q, want fallback %q", custom.WorkspaceInactive, defaultTheme.WorkspaceInactive)
	}
}
