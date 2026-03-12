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
	ActionRenameTab       = "rename_tab"
	// ActionLock exits prefix mode after the current action chain completes.
	// Without it in the chain, the user stays in the current layer.
	ActionLock = "lock"
	// ActionToggleScheme switches the UI color scheme between dark and light.
	ActionToggleScheme = "toggle_scheme"
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
	// SheetKey overrides the key label shown in the which-key sheet.
	// Useful for collapsing a range of bindings into one row (e.g. "1-9").
	SheetKey string `json:"sheet_key,omitempty"`
	// SheetHide removes this binding from the which-key sheet display.
	// Use it for the non-representative entries in a range already summarised
	// by another binding's SheetKey.
	SheetHide bool `json:"sheet_hide,omitempty"`
}

// IsGroup returns true when this keybind opens a sub-layer.
func (k *Keybind) IsGroup() bool { return len(k.Bindings) > 0 }

// FindDirectKey searches binds recursively for a binding whose action list
// contains action and whose Params["index"] equals index. It returns the Key
// of the first matching binding, or "" when none is found.
//
// This is used by the tab-bar renderer to display the exact key the user
// should press to jump to a given workspace or inner tab — so the label
// automatically reflects whatever keybind configuration is active.
func FindDirectKey(binds []*Keybind, action string, index int) string {
	for _, b := range binds {
		// Recurse into sub-layers first so nested bindings are reachable.
		if b.IsGroup() {
			if key := FindDirectKey(b.Bindings, action, index); key != "" {
				return key
			}
			continue
		}
		// Check whether this leaf binding targets the requested action+index.
		for _, a := range b.ActionList() {
			if a != action {
				continue
			}
			if idx, ok := b.Params["index"]; ok {
				// Params values may be float64 (JSON-decoded) or int.
				var iv int
				switch v := idx.(type) {
				case int:
					iv = v
				case float64:
					iv = int(v)
				default:
					continue
				}
				if iv == index {
					return b.Key
				}
			}
		}
	}
	return ""
}

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
//
// Direct go-to actions (go_to_workspace, go_to_tab) also omit "lock" by
// default — add it explicitly in your config if you prefer to exit prefix
// mode after jumping to a workspace or tab.
func DefaultKeybinds() *KeybindsConfig {
	// Build workspace bindings: keys 1-9 → go_to_workspace index 0-8.
	wsBindings := make([]*Keybind, 9)
	for i := 0; i < 9; i++ {
		b := &Keybind{
			Key:         string(rune('1' + i)),
			Description: "Go to workspace",
			Action:      ActionGoToWorkspace,
			Params:      map[string]any{"index": i},
		}
		if i == 0 {
			b.SheetKey = "1-9"
		} else {
			b.SheetHide = true
		}
		wsBindings[i] = b
	}

	// Build tab bindings: keys a-z → go_to_tab index 0-25.
	tabBindings := make([]*Keybind, 26)
	for i := 0; i < 26; i++ {
		b := &Keybind{
			Key:         string(rune('a' + i)),
			Description: "Go to tab",
			Action:      ActionGoToTab,
			Params:      map[string]any{"index": i},
		}
		if i == 0 {
			b.SheetKey = "a-z"
		} else {
			b.SheetHide = true
		}
		tabBindings[i] = b
	}

	root := []*Keybind{
		{
			Key:         "ctrl+b",
			Description: "Send ctrl+b to pane",
			Actions:     []string{ActionPassthrough, ActionLock},
		},
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
				{Key: "r", Description: "Rename tab", Actions: []string{ActionRenameTab, ActionLock}},
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
		{
			Key:         "u",
			Description: "UI",
			Bindings: []*Keybind{
				{Key: "a", Description: "Toggle appearance", Actions: []string{ActionToggleScheme, ActionLock}},
			},
		},
	}

	root = append(root, wsBindings...)
	root = append(root, tabBindings...)

	return &KeybindsConfig{
		Prefix:   "ctrl+b",
		Bindings: root,
	}
}
