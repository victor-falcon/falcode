package ui

import (
	"github.com/charmbracelet/bubbletea"
)

// keyToBytes converts a bubbletea KeyMsg into the raw bytes that should be
// written to the active PTY. Returns nil if the key has no meaningful mapping.
//
// Note: tea.KeyEnter == tea.KeyCtrlM (13) and tea.KeyTab == tea.KeyCtrlI (9),
// so those Ctrl variants are handled by the Enter/Tab cases and omitted below.
func keyToBytes(msg tea.KeyMsg) []byte {
	// Printable characters
	if msg.Type == tea.KeyRunes {
		s := string(msg.Runes)
		if msg.Alt {
			return append([]byte{0x1b}, []byte(s)...)
		}
		return []byte(s)
	}

	// Special keys
	switch msg.Type {
	case tea.KeySpace:
		if msg.Alt {
			return []byte{0x1b, ' '}
		}
		return []byte{' '}
	case tea.KeyEnter: // == tea.KeyCtrlM
		return []byte{'\r'}
	case tea.KeyBackspace:
		if msg.Alt {
			return []byte{0x1b, 127}
		}
		return []byte{127}
	case tea.KeyTab: // == tea.KeyCtrlI
		return []byte{'\t'}
	case tea.KeyShiftTab:
		return []byte{0x1b, '[', 'Z'}
	case tea.KeyEsc:
		return []byte{0x1b}
	case tea.KeyDelete:
		return []byte{0x1b, '[', '3', '~'}
	case tea.KeyInsert:
		return []byte{0x1b, '[', '2', '~'}

	// Arrow keys
	case tea.KeyUp:
		if msg.Alt {
			return []byte{0x1b, 0x1b, '[', 'A'}
		}
		return []byte{0x1b, '[', 'A'}
	case tea.KeyDown:
		if msg.Alt {
			return []byte{0x1b, 0x1b, '[', 'B'}
		}
		return []byte{0x1b, '[', 'B'}
	case tea.KeyRight:
		if msg.Alt {
			return []byte{0x1b, '[', '1', ';', '3', 'C'}
		}
		return []byte{0x1b, '[', 'C'}
	case tea.KeyLeft:
		if msg.Alt {
			return []byte{0x1b, '[', '1', ';', '3', 'D'}
		}
		return []byte{0x1b, '[', 'D'}

	// Home / End / PgUp / PgDn
	case tea.KeyHome:
		return []byte{0x1b, '[', 'H'}
	case tea.KeyEnd:
		return []byte{0x1b, '[', 'F'}
	case tea.KeyPgUp:
		return []byte{0x1b, '[', '5', '~'}
	case tea.KeyPgDown:
		return []byte{0x1b, '[', '6', '~'}

	// F-keys
	case tea.KeyF1:
		return []byte{0x1b, 'O', 'P'}
	case tea.KeyF2:
		return []byte{0x1b, 'O', 'Q'}
	case tea.KeyF3:
		return []byte{0x1b, 'O', 'R'}
	case tea.KeyF4:
		return []byte{0x1b, 'O', 'S'}
	case tea.KeyF5:
		return []byte{0x1b, '[', '1', '5', '~'}
	case tea.KeyF6:
		return []byte{0x1b, '[', '1', '7', '~'}
	case tea.KeyF7:
		return []byte{0x1b, '[', '1', '8', '~'}
	case tea.KeyF8:
		return []byte{0x1b, '[', '1', '9', '~'}
	case tea.KeyF9:
		return []byte{0x1b, '[', '2', '0', '~'}
	case tea.KeyF10:
		return []byte{0x1b, '[', '2', '1', '~'}
	case tea.KeyF11:
		return []byte{0x1b, '[', '2', '3', '~'}
	case tea.KeyF12:
		return []byte{0x1b, '[', '2', '4', '~'}

	// Ctrl+letter — KeyCtrlI (9) and KeyCtrlM (13) are already covered
	// by KeyTab and KeyEnter respectively, so they are skipped here.
	case tea.KeyCtrlA:
		return []byte{1}
	case tea.KeyCtrlB:
		return []byte{2}
	case tea.KeyCtrlC:
		return []byte{3}
	case tea.KeyCtrlD:
		return []byte{4}
	case tea.KeyCtrlE:
		return []byte{5}
	case tea.KeyCtrlF:
		return []byte{6}
	case tea.KeyCtrlG:
		return []byte{7}
	case tea.KeyCtrlH:
		return []byte{8}
	case tea.KeyCtrlJ:
		return []byte{10}
	case tea.KeyCtrlK:
		return []byte{11}
	case tea.KeyCtrlL:
		return []byte{12}
	case tea.KeyCtrlN:
		return []byte{14}
	case tea.KeyCtrlO:
		return []byte{15}
	case tea.KeyCtrlP:
		return []byte{16}
	case tea.KeyCtrlQ:
		return []byte{17}
	case tea.KeyCtrlR:
		return []byte{18}
	case tea.KeyCtrlS:
		return []byte{19}
	case tea.KeyCtrlT:
		return []byte{20}
	case tea.KeyCtrlU:
		return []byte{21}
	case tea.KeyCtrlV:
		return []byte{22}
	case tea.KeyCtrlW:
		return []byte{23}
	case tea.KeyCtrlX:
		return []byte{24}
	case tea.KeyCtrlY:
		return []byte{25}
	case tea.KeyCtrlZ:
		return []byte{26}
	}

	return nil
}
