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

// Config is the top-level application configuration.
type Config struct {
	Tabs  []*Tab `json:"tabs"`
	Theme string `json:"theme,omitempty"`
}

// Load returns the Config using a 3-priority search:
//  1. <cwd>/falcode.json
//  2. ~/.config/falcode/config.json
//  3. Built-in defaults
func Load(cwd string) (*Config, error) {
	paths := []string{
		filepath.Join(cwd, "falcode.json"),
		filepath.Join(os.Getenv("HOME"), ".config", "falcode", "config.json"),
	}

	for _, p := range paths {
		cfg, err := loadFile(p)
		if err == nil {
			return cfg, nil
		}
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("reading config %s: %w", p, err)
		}
	}

	return DefaultConfig(), nil
}

func loadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if len(cfg.Tabs) == 0 {
		return nil, os.ErrNotExist
	}
	return &cfg, nil
}
