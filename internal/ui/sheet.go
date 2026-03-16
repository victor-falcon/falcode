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

	// sheetLabel returns the key label to display in the sheet for b.
	sheetLabel := func(b *config.Keybind) string {
		if b.SheetKey != "" {
			return b.SheetKey
		}
		return b.DisplayLabel()
	}

	// Determine column widths (skip hidden entries).
	maxKeyW := 0
	maxDescW := 0
	for _, b := range bindings {
		if b.SheetHide {
			continue
		}
		if w := len(sheetLabel(b)); w > maxKeyW {
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
		if b.SheetHide {
			continue
		}
		key := st.SheetKey.Render(padRight(sheetLabel(b), maxKeyW))
		var desc string
		if b.IsGroup() {
			desc = st.SheetGroup.Render(b.Description + "  +")
		} else {
			desc = st.SheetDesc.Render(b.Description)
		}
		rows = append(rows, key+st.SheetDesc.Render("  ")+desc)
	}

	content := strings.Join(rows, "\n")
	return st.SheetBox.Render(content)
}

// overlayCentered composites overlayStr horizontally and vertically centered
// over baseStr. baseStr is the pane content exactly totalWidth×totalHeight
// characters. Each line in vt10x output ends with \x1b[0m so ANSI state does
// not bleed across lines, making per-line truncation safe.
func overlayCentered(baseStr, overlayStr string, totalWidth, totalHeight int) string {
	if overlayStr == "" {
		return baseStr
	}

	baseLines := strings.Split(baseStr, "\n")
	overlayLines := strings.Split(overlayStr, "\n")
	overlayH := len(overlayLines)

	// Find the widest overlay line.
	overlayW := 0
	for _, l := range overlayLines {
		if w := lipgloss.Width(l); w > overlayW {
			overlayW = w
		}
	}

	// Center position within the pane area.
	startRow := (totalHeight - overlayH) / 2
	startCol := (totalWidth - overlayW) / 2
	if startCol < 0 {
		startCol = 0
	}

	for i, overlayLine := range overlayLines {
		row := startRow + i
		if row < 0 || row >= len(baseLines) {
			continue
		}

		base := baseLines[row]

		// Left: keep base content up to startCol (safe with ANSI codes).
		left := xansi.Truncate(base, startCol, "")
		if leftW := lipgloss.Width(left); leftW < startCol {
			left += strings.Repeat(" ", startCol-leftW)
		}

		// Right: fill remaining width after the overlay with spaces.
		rightW := totalWidth - startCol - lipgloss.Width(overlayLine)
		right := ""
		if rightW > 0 {
			right = strings.Repeat(" ", rightW)
		}

		baseLines[row] = left + overlayLine + right
	}

	return strings.Join(baseLines, "\n")
}

// overlayBottomRight composites sheetStr over baseStr near the bottom-right
// corner with a 1-cell right margin. totalWidth is the full terminal width.
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
		// Truncate base line to leave room for the sheet and its right margin.
		truncated := xansi.Truncate(base, totalWidth-sheetW-1, "")
		// Pad truncated to (totalWidth - sheetW - 1) characters.
		truncW := lipgloss.Width(truncated)
		pad := totalWidth - sheetW - 1 - truncW
		if pad < 0 {
			pad = 0
		}
		baseLines[baseRow] = truncated + strings.Repeat(" ", pad) + sheetLine
	}

	return strings.Join(baseLines, "\n")
}

// overlayTopRight composites overlayStr over baseStr anchored near the
// top-right corner with a 1-cell right margin, starting at startRow
// (0 = very first line).
func overlayTopRight(baseStr, overlayStr string, totalWidth, startRow int) string {
	if overlayStr == "" {
		return baseStr
	}

	baseLines := strings.Split(baseStr, "\n")
	overlayLines := strings.Split(overlayStr, "\n")

	// Find the widest overlay line.
	overlayW := 0
	for _, l := range overlayLines {
		if w := lipgloss.Width(l); w > overlayW {
			overlayW = w
		}
	}

	startCol := totalWidth - overlayW - 1
	if startCol < 0 {
		startCol = 0
	}

	for i, overlayLine := range overlayLines {
		baseRow := startRow + i
		if baseRow >= len(baseLines) {
			break
		}
		base := baseLines[baseRow]

		// Keep base content up to startCol.
		left := xansi.Truncate(base, startCol, "")
		if leftW := lipgloss.Width(left); leftW < startCol {
			left += strings.Repeat(" ", startCol-leftW)
		}

		baseLines[baseRow] = left + overlayLine
	}

	return strings.Join(baseLines, "\n")
}

func overlayHeight(overlayStr string) int {
	if overlayStr == "" {
		return 0
	}
	return len(strings.Split(overlayStr, "\n"))
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
