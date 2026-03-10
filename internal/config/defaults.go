package config

// DefaultConfig returns the built-in configuration used when no config file is found.
func DefaultConfig() *Config {
	return &Config{
		Tabs: []*Tab{
			{Name: "Agent", Command: "opencode"},
			{Name: "Git", Command: "lazygit"},
			{Name: "Console"},
		},
		Theme:       "default",
		ThemeScheme: "system",
		Keybinds:    DefaultKeybinds(),
	}
}
