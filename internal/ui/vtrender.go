package ui

import (
	"fmt"
	"strings"

	"github.com/hinshun/vt10x"
)

// vt10x attribute bit masks (unexported in vt10x, so we replicate them).
const (
	vtAttrReverse   int16 = 1 << 0
	vtAttrUnderline int16 = 1 << 1
	vtAttrBold      int16 = 1 << 2
	vtAttrItalic    int16 = 1 << 4
	vtAttrBlink     int16 = 1 << 5
)

// renderVT walks the vt10x terminal grid and produces a full ANSI string.
func renderVT(vt vt10x.Terminal, cols, rows int) string {
	var sb strings.Builder
	var prevFg, prevBg vt10x.Color
	var prevMode int16
	resetNeeded := true

	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			cell := vt.Cell(col, row)

			if resetNeeded || cell.FG != prevFg || cell.BG != prevBg || cell.Mode != prevMode {
				sb.WriteString("\x1b[0m") // reset all
				if cell.BG != vt10x.DefaultBG {
					sb.WriteString(bgEscape(cell.BG))
				}
				fg := cell.FG
				// Bold + low-colour → bright variant (matches typical terminal behaviour)
				if cell.Mode&vtAttrBold != 0 && fg < 8 {
					fg += 8
				}
				if fg != vt10x.DefaultFG {
					sb.WriteString(fgEscape(fg))
				}
				if cell.Mode&vtAttrBold != 0 {
					sb.WriteString("\x1b[1m")
				}
				if cell.Mode&vtAttrItalic != 0 {
					sb.WriteString("\x1b[3m")
				}
				if cell.Mode&vtAttrUnderline != 0 {
					sb.WriteString("\x1b[4m")
				}
				if cell.Mode&vtAttrBlink != 0 {
					sb.WriteString("\x1b[5m")
				}
				if cell.Mode&vtAttrReverse != 0 {
					sb.WriteString("\x1b[7m")
				}
				prevFg = cell.FG
				prevBg = cell.BG
				prevMode = cell.Mode
				resetNeeded = false
			}

			ch := cell.Char
			if ch == 0 {
				ch = ' '
			}
			sb.WriteRune(ch)
		}
		sb.WriteString("\x1b[0m")
		resetNeeded = true
		if row < rows-1 {
			sb.WriteByte('\n')
		}
	}

	return sb.String()
}

func fgEscape(c vt10x.Color) string {
	switch {
	case c == vt10x.DefaultFG:
		return ""
	case c < 8:
		return fmt.Sprintf("\x1b[%dm", 30+int(c))
	case c < 16:
		return fmt.Sprintf("\x1b[%dm", 90+int(c)-8)
	default:
		return fmt.Sprintf("\x1b[38;5;%dm", int(c)-17)
	}
}

func bgEscape(c vt10x.Color) string {
	switch {
	case c == vt10x.DefaultBG:
		return ""
	case c < 8:
		return fmt.Sprintf("\x1b[%dm", 40+int(c))
	case c < 16:
		return fmt.Sprintf("\x1b[%dm", 100+int(c)-8)
	default:
		return fmt.Sprintf("\x1b[48;5;%dm", int(c)-17)
	}
}
