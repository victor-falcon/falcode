package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/victor-falcon/falcode/internal/config"
)

// uiStyles holds all pre-built lipgloss styles derived from the active theme.
type uiStyles struct {
	// Workspace tab bar (outer)
	WorkspaceActive   lipgloss.Style
	WorkspaceInactive lipgloss.Style
	WorkspaceBarBg    lipgloss.Style
	PrefixIndicator   lipgloss.Style
	StatusMsg         lipgloss.Style

	// Inner tab bar
	InnerActive    lipgloss.Style
	InnerInactive  lipgloss.Style
	InnerBarBg     lipgloss.Style
	InnerSeparator lipgloss.Style

	// Which-key sheet
	SheetBox   lipgloss.Style
	SheetTitle lipgloss.Style
	SheetKey   lipgloss.Style
	SheetDesc  lipgloss.Style
	SheetGroup lipgloss.Style
	SheetSep   lipgloss.Style

	// Exit / restart banner
	ExitBanner lipgloss.Style

	// Warning / danger text (used in delete-confirmation dialog)
	WarningMsg lipgloss.Style

	// Footer bar
	FooterBg   lipgloss.Style
	FooterText lipgloss.Style
	FooterKey  lipgloss.Style

	// Raw colour for compositing — may be lipgloss.NoColor{} when transparent.
	SheetBgColor lipgloss.TerminalColor
}

// toColor converts a theme color string to a lipgloss.TerminalColor.
// The special value "transparent" maps to lipgloss.NoColor{}, which tells
// lipgloss not to set any background/foreground, effectively inheriting the
// terminal's default (transparent). Any other value is treated as a hex color.
func toColor(s string) lipgloss.TerminalColor {
	if s == "transparent" {
		return lipgloss.NoColor{}
	}
	return lipgloss.Color(s)
}

func newStyles(t *config.ThemeColors) uiStyles {
	sheetBg := toColor(t.SheetBg)

	return uiStyles{
		WorkspaceActive: lipgloss.NewStyle().
			Bold(true).
			Foreground(toColor(t.WorkspaceActiveFg)).
			Background(toColor(t.WorkspaceActive)).
			Padding(0, 1),

		WorkspaceInactive: lipgloss.NewStyle().
			Foreground(toColor(t.WorkspaceInactiveFg)).
			Background(toColor(t.WorkspaceInactive)).
			Padding(0, 1),

		WorkspaceBarBg: lipgloss.NewStyle().
			Background(toColor(t.WorkspaceBarBg)),

		PrefixIndicator: lipgloss.NewStyle().
			Bold(true).
			Foreground(toColor(t.PrefixIndicatorFg)).
			Background(toColor(t.WorkspaceBarBg)),

		StatusMsg: lipgloss.NewStyle().
			Foreground(toColor(t.StatusFg)).
			Background(toColor(t.WorkspaceBarBg)),

		InnerActive: lipgloss.NewStyle().
			Bold(true).
			Foreground(toColor(t.InnerActiveFg)).
			Background(toColor(t.InnerActive)).
			Padding(0, 1),

		InnerInactive: lipgloss.NewStyle().
			Foreground(toColor(t.InnerInactiveFg)).
			Background(toColor(t.InnerInactive)).
			Padding(0, 1),

		InnerBarBg: lipgloss.NewStyle().
			Background(toColor(t.InnerBarBg)),

		InnerSeparator: lipgloss.NewStyle().
			Foreground(toColor(t.InnerSeparator)).
			Background(toColor(t.InnerBarBg)),

		SheetBox: lipgloss.NewStyle().
			Background(sheetBg).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(toColor(t.SheetBorder)).
			BorderBackground(sheetBg).
			Padding(0, 1),

		SheetTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(toColor(t.SheetTitle)).
			Background(sheetBg),

		SheetKey: lipgloss.NewStyle().
			Bold(true).
			Foreground(toColor(t.SheetKey)).
			Background(sheetBg),

		SheetDesc: lipgloss.NewStyle().
			Foreground(toColor(t.SheetDescription)).
			Background(sheetBg),

		SheetGroup: lipgloss.NewStyle().
			Foreground(toColor(t.SheetGroup)).
			Background(sheetBg),

		SheetSep: lipgloss.NewStyle().
			Foreground(toColor(t.SheetSeparator)).
			Background(sheetBg),

		ExitBanner: lipgloss.NewStyle().
			Foreground(toColor(t.SheetDescription)).
			Background(sheetBg).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(toColor(t.SheetBorder)).
			BorderBackground(sheetBg).
			Padding(0, 1),

		WarningMsg: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF5F5F")).
			Background(sheetBg),

		FooterBg: lipgloss.NewStyle().
			Background(toColor(t.InnerBarBg)),

		FooterText: lipgloss.NewStyle().
			Foreground(toColor(t.InnerInactiveFg)).
			Background(toColor(t.InnerBarBg)),

		FooterKey: lipgloss.NewStyle().
			Bold(true).
			Foreground(toColor(t.SheetKey)).
			Background(toColor(t.InnerBarBg)),

		SheetBgColor: sheetBg,
	}
}
