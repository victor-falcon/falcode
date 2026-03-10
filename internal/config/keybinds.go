package config

// Action names understood by the UI layer.
const (
	ActionQuit            = "quit"
	ActionNextTab         = "next_tab"
	ActionPrevTab         = "prev_tab"
	ActionNewTab          = "new_tab"
	ActionCloseTab        = "close_tab"
	ActionNextWorkspace   = "next_workspace"
	ActionPrevWorkspace   = "prev_workspace"
	ActionNewWorkspace    = "new_workspace"
	ActionDeleteWorkspace = "delete_workspace"
	ActionPassthrough     = "passthrough"
	ActionGoToTab         = "go_to_tab"
	ActionGoToWorkspace   = "go_to_workspace"
	// ActionLock exits prefix mode after the current action chain completes.
	// Without it in the chain, the user stays in the current layer.
	ActionLock = "lock"
)

// Keybind represents a single binding entry. It is either:
//   - A leaf action (Action or Actions set) with an optional Params map.
//   - A group/layer (Bindings != nil) that opens a sub-menu.
//
// Use Action for convenience when only one action is needed, or Actions to
// chain multiple actions (e.g. ["close_tab", "lock"]).
type Keybind struct {
	Key         string         `json:"key"`
	Label       string         `json:"label,omitempty"`
	Description string         `json:"description"`
	Action      string         `json:"action,omitempty"`  // single action shorthand
	Actions     []string       `json:"actions,omitempty"` // multi-action chain
	Params      map[string]any `json:"params,omitempty"`
	Bindings    []*Keybind     `json:"bindings,omitempty"`
}

// IsGroup returns true when this keybind opens a sub-layer.
func (k *Keybind) IsGroup() bool { return len(k.Bindings) > 0 }

// ActionList returns the effective list of actions to execute for this
// keybind. If Actions is set it takes precedence; otherwise the singular
// Action field is wrapped in a slice.
func (k *Keybind) ActionList() []string {
	if len(k.Actions) > 0 {
		return k.Actions
	}
	if k.Action != "" {
		return []string{k.Action}
	}
	return nil
}

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

// DefaultKeybinds returns the built-in keybind configuration.
//
// Navigation actions (next/prev tab, next/prev workspace) intentionally omit
// the "lock" action so the user stays in the current layer and can press the
// key repeatedly. Structural/one-off actions include "lock" to return to the
// locked (normal) state after execution.
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
				Key:         "t",
				Description: "Tab",
				Bindings: []*Keybind{
					{Key: "l", Description: "Next tab", Action: ActionNextTab},
					{Key: "h", Description: "Previous tab", Action: ActionPrevTab},
					{Key: "n", Description: "New console tab", Actions: []string{ActionNewTab, ActionLock}},
					{Key: "x", Description: "Close tab", Actions: []string{ActionCloseTab, ActionLock}},
				},
			},
			{
				Key:         "w",
				Description: "Workspace",
				Bindings: []*Keybind{
					{Key: "l", Description: "Next workspace", Action: ActionNextWorkspace},
					{Key: "h", Description: "Previous workspace", Action: ActionPrevWorkspace},
					{Key: "n", Description: "Create workspace", Actions: []string{ActionNewWorkspace, ActionLock}},
					{Key: "x", Description: "Delete workspace", Actions: []string{ActionDeleteWorkspace, ActionLock}},
				},
			},
		},
	}
}
