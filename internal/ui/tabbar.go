package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
	"github.com/victor-falcon/falcode/internal/config"
	"github.com/victor-falcon/falcode/internal/git"
)

const (
	zoneWorkspacePrefix = "ws-"
	zoneInnerPrefix     = "it-"
	zoneInnerClose      = "it-close-"
	zoneNewTabBtn       = "new-tab-btn"
)

// WorkspaceTabZoneID returns the bubblezone ID for a workspace tab.
func WorkspaceTabZoneID(idx int) string { return fmt.Sprintf("%s%d", zoneWorkspacePrefix, idx) }

// InnerTabZoneID returns the bubblezone ID for an inner tab.
func InnerTabZoneID(idx int) string { return fmt.Sprintf("%s%d", zoneInnerPrefix, idx) }

// InnerTabCloseZoneID returns the bubblezone ID for an inner tab's × button.
func InnerTabCloseZoneID(idx int) string { return fmt.Sprintf("%s%d", zoneInnerClose, idx) }

// NewTabBtnZoneID returns the bubblezone ID for the + new-tab button.
func NewTabBtnZoneID() string { return zoneNewTabBtn }

// TabBarHeight returns the number of rows the tab bar occupies (2: workspace + inner).
func TabBarHeight() int { return 2 }

// RenderTabBar renders the full two-row tab bar.
func RenderTabBar(
	zm *zone.Manager,
	worktrees []*git.Worktree,
	innerTabs []*config.Tab,
	extraTabs []string, // dynamically added console tab labels
	activeWS, activeInner int,
	totalWidth int,
	prefixMode bool,
	statusMsg string,
	ui *config.UIConfig,
	st uiStyles,
) string {
	wsRow := renderWorkspaceRow(zm, worktrees, activeWS, totalWidth, prefixMode, statusMsg, st)
	innerRow := renderInnerRow(zm, innerTabs, extraTabs, activeInner, totalWidth, ui, st)
	return lipgloss.JoinVertical(lipgloss.Left, wsRow, innerRow)
}

// renderWorkspaceRow renders the top row of workspace (outer) tabs.
func renderWorkspaceRow(
	zm *zone.Manager,
	worktrees []*git.Worktree,
	activeWS, totalWidth int,
	prefixMode bool,
	statusMsg string,
	st uiStyles,
) string {
	var tabs []string
	for i, wt := range worktrees {
		label := wt.Name()
		var styled string
		if i == activeWS {
			styled = st.WorkspaceActive.Render(label)
		} else {
			styled = st.WorkspaceInactive.Render(label)
		}
		tabs = append(tabs, zm.Mark(WorkspaceTabZoneID(i), styled))
	}

	tabsStr := strings.Join(tabs, "")
	tabsWidth := lipgloss.Width(tabsStr)

	// Indicator on the right: PREFIX mode or status message.
	indicator := ""
	if prefixMode {
		indicator = st.PrefixIndicator.Render(" [PREFIX] ")
	} else if statusMsg != "" {
		indicator = st.StatusMsg.Render(" " + statusMsg + " ")
	}

	remainingWidth := totalWidth - tabsWidth - lipgloss.Width(indicator)
	if remainingWidth < 0 {
		remainingWidth = 0
	}
	gap := st.WorkspaceBarBg.Render(strings.Repeat(" ", remainingWidth))

	return tabsStr + gap + indicator
}

// renderInnerRow renders the second row of inner (per-workspace) tabs,
// including optional × close buttons and + new-tab button.
func renderInnerRow(
	zm *zone.Manager,
	cfgTabs []*config.Tab,
	extraTabs []string,
	activeInner, totalWidth int,
	ui *config.UIConfig,
	st uiStyles,
) string {
	closeMode := ui.GetCloseTabButton()
	showNewTab := ui.GetNewTabButton()

	// Combine configured tabs and dynamically-added ones.
	allLabels := make([]string, 0, len(cfgTabs)+len(extraTabs))
	for _, t := range cfgTabs {
		allLabels = append(allLabels, t.Name)
	}
	allLabels = append(allLabels, extraTabs...)

	cfgCount := len(cfgTabs) // built-in tabs cannot be closed
	sep := st.InnerSeparator.Render("│")

	var parts []string
	for i, label := range allLabels {
		isActive := i == activeInner
		isExtra := i >= cfgCount
		showClose := isExtra && (closeMode == config.CloseTabButtonAll ||
			(closeMode == config.CloseTabButtonFocus && isActive))

		var tabPart string
		if showClose {
			// Split the tab into two zones: label (switches tab) and × (closes tab).
			// The label loses its right padding; the × carries left spacing + right padding.
			if isActive {
				namePart := zm.Mark(InnerTabZoneID(i), st.InnerActive.PaddingRight(0).Render(label))
				closePart := zm.Mark(InnerTabCloseZoneID(i), st.InnerActive.Bold(false).PaddingLeft(0).Render(" ×"))
				tabPart = namePart + closePart
			} else {
				namePart := zm.Mark(InnerTabZoneID(i), st.InnerInactive.PaddingRight(0).Render(label))
				closePart := zm.Mark(InnerTabCloseZoneID(i), st.InnerInactive.PaddingLeft(0).Render(" ×"))
				tabPart = namePart + closePart
			}
		} else {
			var styled string
			if isActive {
				styled = st.InnerActive.Render(label)
			} else {
				styled = st.InnerInactive.Render(label)
			}
			tabPart = zm.Mark(InnerTabZoneID(i), styled)
		}

		if i > 0 {
			parts = append(parts, sep)
		}
		parts = append(parts, tabPart)
	}

	// + new-tab button at the end.
	if showNewTab {
		newTabPart := zm.Mark(NewTabBtnZoneID(), st.InnerInactive.Render("+"))
		if len(parts) > 0 {
			parts = append(parts, sep)
		}
		parts = append(parts, newTabPart)
	}

	row := strings.Join(parts, "")
	rowWidth := lipgloss.Width(row)
	if rowWidth < totalWidth {
		row += st.InnerBarBg.Render(strings.Repeat(" ", totalWidth-rowWidth))
	}
	return row
}
