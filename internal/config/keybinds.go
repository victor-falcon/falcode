package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Action names understood by the UI layer.
const (
	ActionQuit            = "quit"
	ActionNextTab         = "next_tab"
	ActionPrevTab         = "prev_tab"
	ActionNewTab          = "new_tab"
	ActionCloseTab        = "close_tab"
	ActionNextWorkspace   = "next_workspace"
	ActionPrevWorkspace   = "prev_workspace"
	ActionDeleteWorkspace = "delete_workspace"
	ActionPassthrough     = "passthrough"
	ActionGoToTab         = "go_to_tab"
	ActionGoToWorkspace   = "go_to_workspace"
)

// Keybind represents a single binding entry. It is either:
//   - A leaf action (Action != "") with an optional Params map.
//   - A group/layer (Bindings != nil) that opens a sub-menu.
type Keybind struct {
	Key         string         `json:"key"`
	Label       string         `json:"label,omitempty"`
	Description string         `json:"description"`
	Action      string         `json:"action,omitempty"`
	Params      map[string]any `json:"params,omitempty"`
	Bindings    []*Keybind     `json:"bindings,omitempty"`
}

// IsGroup returns true when this keybind opens a sub-layer.
func (k *Keybind) IsGroup() bool { return len(k.Bindings) > 0 }

// DisplayLabel returns Label if set, otherwise Key.
func (k *Keybind) DisplayLabel() string {
	if k.Label != "" {
		return k.Label
	}
	return k.Key
}

// KeybindsConfig is the top-level keybind configuration.
type KeybindsConfig struct {
	Prefix   string     `json:"prefix"`
	Bindings []*Keybind `json:"bindings"`
}

// LoadKeybinds loads from ~/.config/falcode/keybinds.json or returns the defaults.
func LoadKeybinds() (*KeybindsConfig, error) {
	p := filepath.Join(os.Getenv("HOME"), ".config", "falcode", "keybinds.json")
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultKeybinds(), nil
		}
		return nil, fmt.Errorf("reading keybinds %s: %w", p, err)
	}
	var kb KeybindsConfig
	if err := json.Unmarshal(data, &kb); err != nil {
		return nil, fmt.Errorf("parsing keybinds %s: %w", p, err)
	}
	if kb.Prefix == "" {
		kb.Prefix = "ctrl+b"
	}
	return &kb, nil
}

// DefaultKeybinds returns the built-in keybind configuration.
func DefaultKeybinds() *KeybindsConfig {
	return &KeybindsConfig{
		Prefix: "ctrl+b",
		Bindings: []*Keybind{
			{
				Key:         "q",
				Description: "Quit",
				Action:      ActionQuit,
			},
			{
				Key:         "ctrl+b",
				Description: "Send prefix to terminal",
				Action:      ActionPassthrough,
			},
			{
				Key:         "t",
				Description: "Tab",
				Bindings: []*Keybind{
					{Key: "n", Description: "Next tab", Action: ActionNextTab},
					{Key: "p", Description: "Previous tab", Action: ActionPrevTab},
					{Key: "c", Description: "New console tab", Action: ActionNewTab},
					{Key: "x", Description: "Close tab", Action: ActionCloseTab},
				},
			},
			{
				Key:         "w",
				Description: "Workspace",
				Bindings: []*Keybind{
					{Key: "n", Description: "Next workspace", Action: ActionNextWorkspace},
					{Key: "p", Description: "Previous workspace", Action: ActionPrevWorkspace},
					{Key: "d", Description: "Delete workspace", Action: ActionDeleteWorkspace},
				},
			},
		},
	}
}
