package config

import "testing"

func TestKeybindActionList(t *testing.T) {
	tests := []struct {
		name string
		bind *Keybind
		want []string
	}{
		{
			name: "actions take precedence",
			bind: &Keybind{Action: ActionQuit, Actions: []string{ActionNewTab, ActionLock}},
			want: []string{ActionNewTab, ActionLock},
		},
		{
			name: "single action becomes slice",
			bind: &Keybind{Action: ActionQuit},
			want: []string{ActionQuit},
		},
		{
			name: "empty returns nil",
			bind: &Keybind{},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.bind.ActionList()
			if len(got) != len(tt.want) {
				t.Fatalf("len(ActionList()) = %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("ActionList()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestKeybindDisplayLabel(t *testing.T) {
	if got := (&Keybind{Key: "a", Label: "Agent"}).DisplayLabel(); got != "Agent" {
		t.Fatalf("DisplayLabel() = %q, want %q", got, "Agent")
	}
	if got := (&Keybind{Key: "a"}).DisplayLabel(); got != "a" {
		t.Fatalf("DisplayLabel() = %q, want %q", got, "a")
	}
}

func TestFindDirectKey(t *testing.T) {
	binds := []*Keybind{
		{Key: "1", Action: ActionGoToWorkspace, Params: map[string]any{"index": 0}},
		{
			Key: "t",
			Bindings: []*Keybind{
				{Key: "a", Action: ActionGoToTab, Params: map[string]any{"index": float64(0)}},
			},
		},
		{Key: "x", Action: ActionGoToWorkspace, Params: map[string]any{"index": "0"}},
	}

	if got := FindDirectKey(binds, ActionGoToWorkspace, 0); got != "1" {
		t.Fatalf("FindDirectKey(workspace, 0) = %q, want %q", got, "1")
	}
	if got := FindDirectKey(binds, ActionGoToTab, 0); got != "a" {
		t.Fatalf("FindDirectKey(tab, 0) = %q, want %q", got, "a")
	}
	if got := FindDirectKey(binds, ActionGoToWorkspace, 1); got != "" {
		t.Fatalf("FindDirectKey(workspace, 1) = %q, want empty string", got)
	}
}

func TestDefaultKeybinds(t *testing.T) {
	got := DefaultKeybinds()

	if got.Prefix != "ctrl+b" {
		t.Fatalf("Prefix = %q, want %q", got.Prefix, "ctrl+b")
	}
	if key := FindDirectKey(got.Bindings, ActionGoToWorkspace, 0); key != "1" {
		t.Fatalf("workspace key 0 = %q, want %q", key, "1")
	}
	if key := FindDirectKey(got.Bindings, ActionGoToWorkspace, 8); key != "9" {
		t.Fatalf("workspace key 8 = %q, want %q", key, "9")
	}
	if key := FindDirectKey(got.Bindings, ActionGoToTab, 0); key != "a" {
		t.Fatalf("tab key 0 = %q, want %q", key, "a")
	}
	if key := FindDirectKey(got.Bindings, ActionGoToTab, 25); key != "z" {
		t.Fatalf("tab key 25 = %q, want %q", key, "z")
	}

	workspaceBindings := 0
	tabBindings := 0
	for _, b := range got.Bindings {
		if b.Action == ActionGoToWorkspace {
			workspaceBindings++
		}
		if b.Action == ActionGoToTab {
			tabBindings++
		}
	}
	if workspaceBindings != 9 {
		t.Fatalf("workspace direct bindings = %d, want 9", workspaceBindings)
	}
	if tabBindings != 26 {
		t.Fatalf("tab direct bindings = %d, want 26", tabBindings)
	}

	if got.Bindings[4].Key != "u" {
		t.Fatalf("unexpected root binding order around direct keys")
	}
	if got.Bindings[5].SheetKey != "1-9" {
		t.Fatalf("first workspace binding SheetKey = %q, want %q", got.Bindings[5].SheetKey, "1-9")
	}
	if !got.Bindings[6].SheetHide {
		t.Fatalf("second workspace binding SheetHide = false, want true")
	}
	if got.Bindings[14].SheetKey != "a-z" {
		t.Fatalf("first tab binding SheetKey = %q, want %q", got.Bindings[14].SheetKey, "a-z")
	}
	if !got.Bindings[15].SheetHide {
		t.Fatalf("second tab binding SheetHide = false, want true")
	}
}
