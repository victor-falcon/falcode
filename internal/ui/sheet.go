package ui

import (
	"strings"

	"github.com/charmbracelet/harmonica"
	"github.com/charmbracelet/lipgloss"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/victor-falcon/falcode/internal/config"
)

// sheetFPS is how often the animation ticks (times per second).
const sheetFPS = 60

// SheetAnimMsg is sent on each animation frame.
type SheetAnimMsg struct{}

// Sheet manages the which-key overlay and its Harmonica spring animation.
type Sheet struct {
	spring   harmonica.Spring
	y        float64 // current animated Y offset (0 = fully visible, positive = hidden below)
	vy       float64 // spring velocity
	targetY  float64 // 0 = open, 1 = closed
	visible  bool
	rendered string // last rendered output, used by View
}

// NewSheet creates a Sheet with a spring tuned for a snappy slide.
func NewSheet() *Sheet {
	return &Sheet{
		spring:  harmonica.NewSpring(harmonica.FPS(sheetFPS), 300, 1.0),
		y:       1.0,
		targetY: 1.0,
	}
}

// Open starts animating the sheet into view.
func (s *Sheet) Open() {
	s.visible = true
	s.targetY = 0
}

// Close starts animating the sheet out of view.
func (s *Sheet) Close() {
	s.targetY = 1
}

// Tick advances the spring by one frame. Returns true if the sheet is now
// fully closed and can be hidden.
func (s *Sheet) Tick() bool {
	s.y, s.vy = s.spring.Update(s.y, s.vy, s.targetY)
	if s.targetY == 1 && s.y > 0.98 {
		s.y = 1
		s.vy = 0
		s.visible = false
		return true // fully closed
	}
	return false
}

// Visible returns true when the sheet should be composited into the view.
func (s *Sheet) Visible() bool { return s.visible }

// AnimOffset returns the current Y offset as a row count offset (0..maxRows).
func (s *Sheet) AnimOffset(maxRows int) int {
	return int(s.y * float64(maxRows))
}

// RenderSheet builds the styled which-key box for the given bindings.
func RenderSheet(bindings []*config.Keybind, title string, st uiStyles) string {
	if len(bindings) == 0 {
		return ""
	}

	// Determine column widths.
	maxKeyW := 0
	maxDescW := 0
	for _, b := range bindings {
		if w := len(b.DisplayLabel()); w > maxKeyW {
			maxKeyW = w
		}
		desc := b.Description
		if b.IsGroup() {
			desc += "  +"
		}
		if w := len(desc); w > maxDescW {
			maxDescW = w
		}
	}
	sepW := maxKeyW + 2 + maxDescW
	if sepW < len(title) {
		sepW = len(title)
	}

	var rows []string

	// Title + separator.
	rows = append(rows, st.SheetTitle.Render(title))
	rows = append(rows, st.SheetSep.Render(strings.Repeat("─", sepW)))

	for _, b := range bindings {
		key := st.SheetKey.Render(padRight(b.DisplayLabel(), maxKeyW))
		var desc string
		if b.IsGroup() {
			desc = st.SheetGroup.Render(b.Description + "  +")
		} else {
			desc = st.SheetDesc.Render(b.Description)
		}
		rows = append(rows, key+"  "+desc)
	}

	content := strings.Join(rows, "\n")
	return st.SheetBox.Render(content)
}

// overlayBottomRight composites sheetStr over baseStr at the bottom-right corner.
// totalWidth is the full terminal width.
func overlayBottomRight(baseStr, sheetStr string, totalWidth, sheetRowOffset int) string {
	if sheetStr == "" {
		return baseStr
	}

	baseLines := strings.Split(baseStr, "\n")
	sheetLines := strings.Split(sheetStr, "\n")
	sheetH := len(sheetLines)
	sheetW := lipgloss.Width(sheetStr) / sheetH // approximate — use first line
	for _, l := range sheetLines {
		if w := lipgloss.Width(l); w > sheetW {
			sheetW = w
		}
	}

	totalLines := len(baseLines)

	// The sheet is anchored at the bottom; sheetRowOffset shifts it down
	// (positive offset = partially hidden below the fold).
	sheetStartRow := totalLines - sheetH + sheetRowOffset

	for i, sheetLine := range sheetLines {
		baseRow := sheetStartRow + i
		if baseRow < 0 || baseRow >= totalLines {
			continue
		}

		base := baseLines[baseRow]
		// Truncate base line to leave room for the sheet.
		truncated := xansi.Truncate(base, totalWidth-sheetW, "")
		// Pad truncated to (totalWidth - sheetW) characters.
		truncW := lipgloss.Width(truncated)
		pad := totalWidth - sheetW - truncW
		if pad < 0 {
			pad = 0
		}
		baseLines[baseRow] = truncated + strings.Repeat(" ", pad) + sheetLine
	}

	return strings.Join(baseLines, "\n")
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
