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
// The cursor is rendered as a soft cursor: the reverse-video attribute is
// XOR-toggled on the cursor cell, making it always visually distinct.
// This is necessary because bubbletea repositions the terminal hardware cursor
// to the bottom of the screen after every frame, so we cannot rely on it.
func renderVT(vt vt10x.Terminal, cols, rows int) string {
	cur := vt.Cursor()
	curVisible := vt.CursorVisible()

	var sb strings.Builder
	var prevFg, prevBg vt10x.Color
	var prevMode int16
	resetNeeded := true

	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			cell := vt.Cell(col, row)

			// XOR the reverse attribute on the cursor cell to draw a soft
			// block cursor that is always visible regardless of cell content.
			mode := cell.Mode
			if curVisible && col == cur.X && row == cur.Y {
				mode ^= vtAttrReverse
			}

			if resetNeeded || cell.FG != prevFg || cell.BG != prevBg || mode != prevMode {
				sb.WriteString("\x1b[0m") // reset all
				if cell.BG != vt10x.DefaultBG {
					sb.WriteString(bgEscape(cell.BG))
				}
				fg := cell.FG
				// Bold + low-colour → bright variant (matches typical terminal behaviour)
				if mode&vtAttrBold != 0 && fg < 8 {
					fg += 8
				}
				if fg != vt10x.DefaultFG {
					sb.WriteString(fgEscape(fg))
				}
				if mode&vtAttrBold != 0 {
					sb.WriteString("\x1b[1m")
				}
				if mode&vtAttrItalic != 0 {
					sb.WriteString("\x1b[3m")
				}
				if mode&vtAttrUnderline != 0 {
					sb.WriteString("\x1b[4m")
				}
				if mode&vtAttrBlink != 0 {
					sb.WriteString("\x1b[5m")
				}
				if mode&vtAttrReverse != 0 {
					sb.WriteString("\x1b[7m")
				}
				prevFg = cell.FG
				prevBg = cell.BG
				prevMode = mode
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

// renderVTWithScrollback renders a view that combines scrollback history with
// the live VT screen. scrollOffset rows of scrollback are shown at the top,
// followed by live VT rows; the last row is replaced with a scroll indicator.
//
// Layout (scrollOffset = S, total rows = R, scrollback len = SB):
//
//	Display row 0       → scrollback[SB - S]         (oldest visible scrollback)
//	Display row S-1     → scrollback[SB - 1]          (newest scrollback row)
//	Display row S       → live VT row 0
//	Display row R-2     → live VT row R-2-S
//	Display row R-1     → scroll indicator bar
func renderVTWithScrollback(scrollback [][]vt10x.Glyph, vt vt10x.Terminal, scrollOffset, cols, rows int) string {
	sbLen := len(scrollback)
	// Clamp scrollOffset so we never go past the beginning of the buffer.
	if scrollOffset > sbLen {
		scrollOffset = sbLen
	}

	// The last row is reserved for the scroll indicator, so content rows = rows-1.
	contentRows := rows - 1

	var sb strings.Builder
	var prevFg, prevBg vt10x.Color
	var prevMode int16
	resetNeeded := true

	for displayRow := 0; displayRow < contentRows; displayRow++ {
		// Which "virtual" row are we showing?
		// virtual 0 → scrollback[sbLen - scrollOffset]
		// virtual scrollOffset → live VT row 0
		sbIdx := sbLen - scrollOffset + displayRow

		for col := 0; col < cols; col++ {
			var cell vt10x.Glyph

			if sbIdx < 0 {
				// Before available scrollback — blank cell.
				cell = vt10x.Glyph{}
			} else if sbIdx < sbLen {
				// Scrollback row.
				row := scrollback[sbIdx]
				if col < len(row) {
					cell = row[col]
				}
			} else {
				// Live VT row.
				vtRow := sbIdx - sbLen // 0-indexed VT row
				if vtRow < rows {
					cell = vt.Cell(col, vtRow)
				}
			}

			if resetNeeded || cell.FG != prevFg || cell.BG != prevBg || cell.Mode != prevMode {
				sb.WriteString("\x1b[0m")
				if cell.BG != vt10x.DefaultBG {
					sb.WriteString(bgEscape(cell.BG))
				}
				fg := cell.FG
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
		sb.WriteByte('\n')
	}

	// Last row: scroll indicator bar (reversed video).
	label := fmt.Sprintf(" \u2191 SCROLL  \u00b7  %d rows above live  \u00b7  scroll or any key to return ", scrollOffset)
	if len(label) > cols {
		label = label[:cols]
	}
	// Pad to full width.
	for len([]rune(label)) < cols {
		label += " "
	}
	sb.WriteString("\x1b[7m") // reverse video
	sb.WriteString(label)
	sb.WriteString("\x1b[0m")

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
	case c < 256:
		return fmt.Sprintf("\x1b[38;5;%dm", int(c))
	default:
		// Truecolor: vt10x stores RGB as r<<16 | g<<8 | b
		r := (c >> 16) & 0xFF
		g := (c >> 8) & 0xFF
		b := c & 0xFF
		return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", r, g, b)
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
	case c < 256:
		return fmt.Sprintf("\x1b[48;5;%dm", int(c))
	default:
		// Truecolor: vt10x stores RGB as r<<16 | g<<8 | b
		r := (c >> 16) & 0xFF
		g := (c >> 8) & 0xFF
		b := c & 0xFF
		return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r, g, b)
	}
}
