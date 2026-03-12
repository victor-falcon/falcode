package ui

import (
	_ "embed"
	"os"
	"path/filepath"
)

//go:embed plugin/falcode.js
var falcodePluginJS []byte

// InstallOpenCodePlugin writes the embedded falcode.js plugin to
// ~/.config/opencode/plugins/falcode.js, but only when the
// ~/.config/opencode/ directory already exists (i.e. the user has OpenCode
// installed). This is a no-op otherwise, so it is safe to call unconditionally.
//
// The plugin is always overwritten so that falcode updates are automatically
// picked up on the next launch.
//
// Errors are silently ignored — a missing plugin degrades gracefully to no
// agent status indicators without affecting any other functionality.
func InstallOpenCodePlugin() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	ocConfigDir := filepath.Join(home, ".config", "opencode")
	if _, err := os.Stat(ocConfigDir); os.IsNotExist(err) {
		// OpenCode not installed (or config dir not yet created) — skip.
		return
	}

	pluginsDir := filepath.Join(ocConfigDir, "plugins")
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		return
	}

	dest := filepath.Join(pluginsDir, "falcode.js")
	//nolint:errcheck
	os.WriteFile(dest, falcodePluginJS, 0o644)
}
