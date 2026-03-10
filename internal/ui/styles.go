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

	// Footer bar
	FooterBg   lipgloss.Style
	FooterText lipgloss.Style
	FooterKey  lipgloss.Style

	// Raw colours for compositing
	SheetBgColor lipgloss.Color
}

func newStyles(t *config.ThemeColors) uiStyles {
	sheetBg := lipgloss.Color(t.SheetBg)

	return uiStyles{
		WorkspaceActive: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(t.WorkspaceActiveFg)).
			Background(lipgloss.Color(t.WorkspaceActive)).
			Padding(0, 1),

		WorkspaceInactive: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.WorkspaceInactiveFg)).
			Background(lipgloss.Color(t.WorkspaceInactive)).
			Padding(0, 1),

		WorkspaceBarBg: lipgloss.NewStyle().
			Background(lipgloss.Color(t.WorkspaceBarBg)),

		PrefixIndicator: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(t.PrefixIndicatorFg)).
			Background(lipgloss.Color(t.WorkspaceBarBg)),

		StatusMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.StatusFg)).
			Background(lipgloss.Color(t.WorkspaceBarBg)),

		InnerActive: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(t.InnerActiveFg)).
			Background(lipgloss.Color(t.InnerActive)).
			Padding(0, 1),

		InnerInactive: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.InnerInactiveFg)).
			Background(lipgloss.Color(t.InnerInactive)).
			Padding(0, 1),

		InnerBarBg: lipgloss.NewStyle().
			Background(lipgloss.Color(t.InnerBarBg)),

		InnerSeparator: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.InnerSeparator)).
			Background(lipgloss.Color(t.InnerBarBg)),

		SheetBox: lipgloss.NewStyle().
			Background(sheetBg).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(t.SheetBorder)).
			BorderBackground(sheetBg).
			Padding(0, 1),

		SheetTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(t.SheetTitle)).
			Background(sheetBg),

		SheetKey: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(t.SheetKey)).
			Background(sheetBg),

		SheetDesc: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.SheetDescription)).
			Background(sheetBg),

		SheetGroup: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.SheetGroup)).
			Background(sheetBg),

		SheetSep: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.SheetSeparator)).
			Background(sheetBg),

		ExitBanner: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.SheetDescription)).
			Background(sheetBg).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(t.SheetBorder)).
			BorderBackground(sheetBg).
			Padding(0, 1),

		FooterBg: lipgloss.NewStyle().
			Background(lipgloss.Color(t.InnerBarBg)),

		FooterText: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.InnerInactiveFg)).
			Background(lipgloss.Color(t.InnerBarBg)),

		FooterKey: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(t.SheetKey)).
			Background(lipgloss.Color(t.InnerBarBg)),

		SheetBgColor: sheetBg,
	}
}
