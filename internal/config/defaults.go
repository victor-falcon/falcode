package config

const (
	// ConfigSchemaURL is the remote JSON Schema for config.json.
	ConfigSchemaURL = "https://raw.githubusercontent.com/victor-falcon/falcode/main/internal/config/schema.json"
	// ThemeSchemaURL is the remote JSON Schema for theme files.
	ThemeSchemaURL = "https://raw.githubusercontent.com/victor-falcon/falcode/main/internal/config/themes/schema.json"
)

// DefaultUIConfig returns the built-in UI configuration defaults.
func DefaultUIConfig() *UIConfig {
	newTabBtn := true
	return &UIConfig{
		Theme:          "default",
		ThemeScheme:    "system",
		NewTabButton:   &newTabBtn,
		CloseTabButton: CloseTabButtonFocus,
	}
}

// DefaultConfig returns the built-in configuration used when no config file is found.
func DefaultConfig() *Config {
	return &Config{
		Tabs: []*Tab{
			{Name: "Agent", Command: "opencode"},
			{Name: "Git", Command: "lazygit"},
			{Name: "Console"},
		},
		UI:       DefaultUIConfig(),
		Keybinds: DefaultKeybinds(),
	}
}
