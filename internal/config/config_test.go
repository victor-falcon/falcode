package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func boolPtr(v bool) *bool {
	return &v
}

func writeJSONFile(t *testing.T, path string, v any) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}

	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestTabShell(t *testing.T) {
	t.Run("uses explicit command first", func(t *testing.T) {
		t.Setenv("SHELL", "/bin/zsh")

		tab := &Tab{Name: "Agent", Command: "opencode"}
		if got := tab.Shell(); got != "opencode" {
			t.Fatalf("Shell() = %q, want %q", got, "opencode")
		}
	})

	t.Run("falls back to SHELL env", func(t *testing.T) {
		t.Setenv("SHELL", "/bin/fish")

		tab := &Tab{Name: "Console"}
		if got := tab.Shell(); got != "/bin/fish" {
			t.Fatalf("Shell() = %q, want %q", got, "/bin/fish")
		}
	})

	t.Run("falls back to bin sh", func(t *testing.T) {
		t.Setenv("SHELL", "")

		tab := &Tab{Name: "Console"}
		if got := tab.Shell(); got != "/bin/sh" {
			t.Fatalf("Shell() = %q, want %q", got, "/bin/sh")
		}
	})
}

func TestTabShouldAutoRun(t *testing.T) {
	tests := []struct {
		name string
		tab  *Tab
		want bool
	}{
		{name: "interactive defaults true", tab: &Tab{Name: "Console"}, want: true},
		{name: "command defaults true", tab: &Tab{Name: "Agent", Command: "opencode"}, want: true},
		{name: "explicit true", tab: &Tab{Name: "Agent", Command: "opencode", AutoRun: boolPtr(true)}, want: true},
		{name: "explicit false", tab: &Tab{Name: "Agent", Command: "opencode", AutoRun: boolPtr(false)}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tab.ShouldAutoRun(); got != tt.want {
				t.Fatalf("ShouldAutoRun() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUIConfigGettersDefaultValues(t *testing.T) {
	var ui *UIConfig

	if got := ui.GetTheme(); got != "default" {
		t.Fatalf("GetTheme() = %q, want %q", got, "default")
	}
	if got := ui.GetThemeScheme(); got != "system" {
		t.Fatalf("GetThemeScheme() = %q, want %q", got, "system")
	}
	if got := ui.GetNewTabButton(); !got {
		t.Fatalf("GetNewTabButton() = %v, want true", got)
	}
	if got := ui.GetNewWorkspaceButton(); !got {
		t.Fatalf("GetNewWorkspaceButton() = %v, want true", got)
	}
	if got := ui.GetCloseTabButton(); got != CloseTabButtonFocus {
		t.Fatalf("GetCloseTabButton() = %q, want %q", got, CloseTabButtonFocus)
	}
	if got := ui.GetCloseWorkspaceButton(); got != CloseWorkspaceButtonNone {
		t.Fatalf("GetCloseWorkspaceButton() = %q, want %q", got, CloseWorkspaceButtonNone)
	}
	if got := ui.GetCompactTabs(); got {
		t.Fatalf("GetCompactTabs() = %v, want false", got)
	}
	if got := ui.GetShowWorkspaceNumbers(); !got {
		t.Fatalf("GetShowWorkspaceNumbers() = %v, want true", got)
	}
	if got := ui.GetShowTabNumbers(); !got {
		t.Fatalf("GetShowTabNumbers() = %v, want true", got)
	}
}

func TestNotificationsConfigGettersDefaultValues(t *testing.T) {
	var cfg *NotificationsConfig

	if got := cfg.GetSoundOnIdle(); !got {
		t.Fatalf("GetSoundOnIdle() = %v, want true", got)
	}
	if got := cfg.GetSoundOnPermission(); !got {
		t.Fatalf("GetSoundOnPermission() = %v, want true", got)
	}
	if got := cfg.GetNotifyOnIdle(); !got {
		t.Fatalf("GetNotifyOnIdle() = %v, want true", got)
	}
	if got := cfg.GetNotifyOnPermission(); !got {
		t.Fatalf("GetNotifyOnPermission() = %v, want true", got)
	}
	if got := cfg.GetNotifyOnQuestion(); !got {
		t.Fatalf("GetNotifyOnQuestion() = %v, want true", got)
	}
	if got := cfg.GetProvider(); got != "osascript" {
		t.Fatalf("GetProvider() = %q, want %q", got, "osascript")
	}
	if got := cfg.GetActivateApp(); got != "" {
		t.Fatalf("GetActivateApp() = %q, want empty string", got)
	}
}

func TestMergeUI(t *testing.T) {
	base := &UIConfig{
		Theme:                "default",
		ThemeScheme:          "dark",
		HideFooter:           false,
		NewTabButton:         boolPtr(true),
		NewWorkspaceButton:   boolPtr(true),
		CloseTabButton:       CloseTabButtonFocus,
		CloseWorkspaceButton: CloseWorkspaceButtonNone,
		CompactTabs:          boolPtr(false),
		ShowWorkspaceNumbers: boolPtr(true),
		ShowTabNumbers:       boolPtr(true),
	}
	override := &UIConfig{
		Theme:                "custom",
		HideFooter:           true,
		NewTabButton:         boolPtr(false),
		CloseWorkspaceButton: CloseWorkspaceButtonAll,
		CompactTabs:          boolPtr(true),
		ShowWorkspaceNumbers: boolPtr(false),
	}

	got := mergeUI(base, override)

	if got.Theme != "custom" {
		t.Fatalf("Theme = %q, want %q", got.Theme, "custom")
	}
	if got.ThemeScheme != "dark" {
		t.Fatalf("ThemeScheme = %q, want %q", got.ThemeScheme, "dark")
	}
	if !got.HideFooter {
		t.Fatalf("HideFooter = %v, want true", got.HideFooter)
	}
	if got.NewTabButton == nil || *got.NewTabButton {
		t.Fatalf("NewTabButton = %v, want false", got.NewTabButton)
	}
	if got.NewWorkspaceButton == nil || !*got.NewWorkspaceButton {
		t.Fatalf("NewWorkspaceButton = %v, want true", got.NewWorkspaceButton)
	}
	if got.CloseTabButton != CloseTabButtonFocus {
		t.Fatalf("CloseTabButton = %q, want %q", got.CloseTabButton, CloseTabButtonFocus)
	}
	if got.CloseWorkspaceButton != CloseWorkspaceButtonAll {
		t.Fatalf("CloseWorkspaceButton = %q, want %q", got.CloseWorkspaceButton, CloseWorkspaceButtonAll)
	}
	if got.CompactTabs == nil || !*got.CompactTabs {
		t.Fatalf("CompactTabs = %v, want true", got.CompactTabs)
	}
	if got.ShowWorkspaceNumbers == nil || *got.ShowWorkspaceNumbers {
		t.Fatalf("ShowWorkspaceNumbers = %v, want false", got.ShowWorkspaceNumbers)
	}
	if got.ShowTabNumbers == nil || !*got.ShowTabNumbers {
		t.Fatalf("ShowTabNumbers = %v, want true", got.ShowTabNumbers)
	}
}

func TestMergeNotifications(t *testing.T) {
	base := &NotificationsConfig{
		SoundOnIdle:        boolPtr(true),
		SoundOnPermission:  boolPtr(true),
		NotifyOnIdle:       boolPtr(true),
		NotifyOnPermission: boolPtr(true),
		NotifyOnQuestion:   boolPtr(true),
		Provider:           "osascript",
		ActivateApp:        "com.example.base",
	}
	override := &NotificationsConfig{
		SoundOnPermission: boolPtr(false),
		NotifyOnQuestion:  boolPtr(false),
		Provider:          "terminal-notifier",
	}

	got := mergeNotifications(base, override)

	if got.SoundOnIdle == nil || !*got.SoundOnIdle {
		t.Fatalf("SoundOnIdle = %v, want true", got.SoundOnIdle)
	}
	if got.SoundOnPermission == nil || *got.SoundOnPermission {
		t.Fatalf("SoundOnPermission = %v, want false", got.SoundOnPermission)
	}
	if got.NotifyOnQuestion == nil || *got.NotifyOnQuestion {
		t.Fatalf("NotifyOnQuestion = %v, want false", got.NotifyOnQuestion)
	}
	if got.Provider != "terminal-notifier" {
		t.Fatalf("Provider = %q, want %q", got.Provider, "terminal-notifier")
	}
	if got.ActivateApp != "com.example.base" {
		t.Fatalf("ActivateApp = %q, want %q", got.ActivateApp, "com.example.base")
	}
}

func TestLoadDefaultsWithoutFiles(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	repo := filepath.Join(t.TempDir(), "repo")

	t.Setenv("HOME", home)
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}

	got, err := Load(repo)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(got.Tabs) != 3 {
		t.Fatalf("len(Tabs) = %d, want 3", len(got.Tabs))
	}
	if got.Tabs[0].Name != "Agent" || got.Tabs[0].Command != "opencode" {
		t.Fatalf("first tab = %+v, want Agent/opencode", got.Tabs[0])
	}
	if got.Keybinds == nil || got.Keybinds.Prefix != "ctrl+b" {
		t.Fatalf("Keybinds.Prefix = %q, want %q", got.Keybinds.Prefix, "ctrl+b")
	}
	if got.UI == nil || got.UI.GetTheme() != "default" {
		t.Fatalf("UI theme = %q, want %q", got.UI.GetTheme(), "default")
	}
	if scripts := got.GetWorktreeScripts(); len(scripts) != 2 || scripts[0] != "falcode.sh" || scripts[1] != "worktree.sh" {
		t.Fatalf("GetWorktreeScripts() = %v, want default script list", scripts)
	}
}

func TestLoadGlobalConfigAppliesPrefixDefault(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	repo := filepath.Join(root, "repo")
	t.Setenv("HOME", home)

	writeJSONFile(t, filepath.Join(home, ".config", "falcode", "config.json"), map[string]any{
		"tabs": []map[string]any{{"name": "Tests", "command": "go test ./..."}},
		"ui": map[string]any{
			"theme": "global-theme",
		},
		"keybinds": map[string]any{
			"prefix":   "",
			"bindings": []map[string]any{{"key": "q", "description": "Quit", "action": "quit"}},
		},
	})

	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}

	got, err := Load(repo)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(got.Tabs) != 1 || got.Tabs[0].Name != "Tests" {
		t.Fatalf("Tabs = %+v, want global tabs", got.Tabs)
	}
	if got.UI == nil || got.UI.GetTheme() != "global-theme" {
		t.Fatalf("UI.GetTheme() = %q, want %q", got.UI.GetTheme(), "global-theme")
	}
	if got.Keybinds == nil || got.Keybinds.Prefix != "ctrl+b" {
		t.Fatalf("Keybinds.Prefix = %q, want %q", got.Keybinds.Prefix, "ctrl+b")
	}
}

func TestLoadRepoTabsReplaceBaseAndAppendedTabsAppend(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	repo := filepath.Join(root, "repo")
	t.Setenv("HOME", home)

	writeJSONFile(t, filepath.Join(home, ".config", "falcode", "config.json"), map[string]any{
		"tabs": []map[string]any{{"name": "Agent", "command": "opencode"}, {"name": "Git", "command": "lazygit"}},
	})
	writeJSONFile(t, filepath.Join(repo, "falcode.json"), map[string]any{
		"tabs":          []map[string]any{{"name": "Console"}},
		"appended_tabs": []map[string]any{{"name": "Tests", "command": "go test ./..."}},
	})

	got, err := Load(repo)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(got.Tabs) != 2 {
		t.Fatalf("len(Tabs) = %d, want 2", len(got.Tabs))
	}
	if got.Tabs[0].Name != "Console" || got.Tabs[1].Name != "Tests" {
		t.Fatalf("Tabs = %+v, want repo tab then appended tab", got.Tabs)
	}
}

func TestLoadRepoConfigMergesUIAndNotifications(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	repo := filepath.Join(root, "repo")
	t.Setenv("HOME", home)

	writeJSONFile(t, filepath.Join(home, ".config", "falcode", "config.json"), map[string]any{
		"ui": map[string]any{
			"theme":                  "base-theme",
			"theme_scheme":           "dark",
			"new_workspace_button":   true,
			"show_workspace_numbers": true,
		},
		"notifications": map[string]any{
			"sound_on_idle":      true,
			"notify_on_question": true,
			"provider":           "osascript",
		},
	})
	writeJSONFile(t, filepath.Join(repo, "falcode.json"), map[string]any{
		"ui": map[string]any{
			"new_workspace_button":   false,
			"show_workspace_numbers": false,
		},
		"notifications": map[string]any{
			"notify_on_question": false,
			"provider":           "terminal-notifier",
		},
	})

	got, err := Load(repo)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got.UI == nil || got.UI.GetTheme() != "base-theme" {
		t.Fatalf("UI.GetTheme() = %q, want %q", got.UI.GetTheme(), "base-theme")
	}
	if got.UI.GetNewWorkspaceButton() {
		t.Fatalf("GetNewWorkspaceButton() = true, want false")
	}
	if got.UI.GetShowWorkspaceNumbers() {
		t.Fatalf("GetShowWorkspaceNumbers() = true, want false")
	}
	notif := got.GetNotifications()
	if notif.GetSoundOnIdle() != true {
		t.Fatalf("GetSoundOnIdle() = false, want true")
	}
	if notif.GetNotifyOnQuestion() != false {
		t.Fatalf("GetNotifyOnQuestion() = true, want false")
	}
	if notif.GetProvider() != "terminal-notifier" {
		t.Fatalf("GetProvider() = %q, want %q", notif.GetProvider(), "terminal-notifier")
	}
}
