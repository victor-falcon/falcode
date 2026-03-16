package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestKeyToBytes(t *testing.T) {
	tests := []struct {
		name string
		msg  tea.KeyMsg
		want []byte
	}{
		{name: "printable rune", msg: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}, want: []byte("a")},
		{name: "alt rune", msg: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}, Alt: true}, want: []byte{0x1b, 'a'}},
		{name: "multibyte rune", msg: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x', 'e', 'a', 'm', 'p', 'l', 'e', ' ', '世'}}, want: []byte("xeample 世")},
		{name: "space", msg: tea.KeyMsg{Type: tea.KeySpace}, want: []byte{' '}},
		{name: "alt enter", msg: tea.KeyMsg{Type: tea.KeyEnter, Alt: true}, want: []byte{0x1b, '\r'}},
		{name: "shift tab", msg: tea.KeyMsg{Type: tea.KeyShiftTab}, want: []byte{0x1b, '[', 'Z'}},
		{name: "up arrow", msg: tea.KeyMsg{Type: tea.KeyUp}, want: []byte{0x1b, '[', 'A'}},
		{name: "alt right arrow", msg: tea.KeyMsg{Type: tea.KeyRight, Alt: true}, want: []byte{0x1b, '[', '1', ';', '3', 'C'}},
		{name: "f5", msg: tea.KeyMsg{Type: tea.KeyF5}, want: []byte{0x1b, '[', '1', '5', '~'}},
		{name: "ctrl c", msg: tea.KeyMsg{Type: tea.KeyCtrlC}, want: []byte{3}},
		{name: "unknown", msg: tea.KeyMsg{Type: tea.KeyCtrlAt}, want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := keyToBytes(tt.msg)
			if len(got) != len(tt.want) {
				t.Fatalf("len(keyToBytes()) = %d, want %d; got=%v want=%v", len(got), len(tt.want), got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("keyToBytes()[%d] = %v, want %v; full got=%v want=%v", i, got[i], tt.want[i], got, tt.want)
				}
			}
		})
	}
}
