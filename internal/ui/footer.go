package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// FooterHeight is the number of rows the footer occupies.
func FooterHeight() int { return 1 }

// RenderFooter renders a single-row footer with a context hint on the left
// and the build version on the right.
// When prefixMode is true (prefix key pressed, waiting for a binding) the left
// side shows "ESC to cancel"; otherwise it shows "Press <Key> to unlock".
func RenderFooter(prefix, version string, prefixMode bool, totalWidth int, st uiStyles) string {
	// Left side depends on whether the prefix is currently active.
	var left string
	if prefixMode {
		left = st.FooterKey.Render(" ESC ") +
			st.FooterText.Render("to cancel ")
	} else {
		left = st.FooterText.Render(" Press ") +
			st.FooterKey.Render(formatPrefix(prefix)) +
			st.FooterText.Render(" to unlock ")
	}

	// Right: version string
	right := st.FooterText.Render(" " + version + " ")

	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	gapW := totalWidth - leftW - rightW
	if gapW < 0 {
		gapW = 0
	}

	return left + st.FooterBg.Render(strings.Repeat(" ", gapW)) + right
}

// formatPrefix turns a raw prefix string like "ctrl+b" into a display form
// like "Ctrl+B", capitalising known modifier names and uppercasing the key.
func formatPrefix(prefix string) string {
	parts := strings.Split(prefix, "+")
	for i, p := range parts {
		switch strings.ToLower(p) {
		case "ctrl":
			parts[i] = "Ctrl"
		case "alt":
			parts[i] = "Alt"
		case "shift":
			parts[i] = "Shift"
		case "super", "cmd":
			parts[i] = "Cmd"
		default:
			parts[i] = strings.ToUpper(p)
		}
	}
	return strings.Join(parts, "+")
}
