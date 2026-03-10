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
)

// WorkspaceTabZoneID returns the bubblezone ID for a workspace tab.
func WorkspaceTabZoneID(idx int) string { return fmt.Sprintf("%s%d", zoneWorkspacePrefix, idx) }

// InnerTabZoneID returns the bubblezone ID for an inner tab.
func InnerTabZoneID(idx int) string { return fmt.Sprintf("%s%d", zoneInnerPrefix, idx) }

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
	st uiStyles,
) string {
	wsRow := renderWorkspaceRow(zm, worktrees, activeWS, totalWidth, prefixMode, statusMsg, st)
	innerRow := renderInnerRow(zm, innerTabs, extraTabs, activeInner, totalWidth, st)
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

// renderInnerRow renders the second row of inner (per-workspace) tabs.
func renderInnerRow(
	zm *zone.Manager,
	cfgTabs []*config.Tab,
	extraTabs []string,
	activeInner, totalWidth int,
	st uiStyles,
) string {
	// Combine configured tabs and dynamically-added ones.
	allLabels := make([]string, 0, len(cfgTabs)+len(extraTabs))
	for _, t := range cfgTabs {
		allLabels = append(allLabels, t.Name)
	}
	allLabels = append(allLabels, extraTabs...)

	sep := st.InnerSeparator.Render("│")

	var parts []string
	for i, label := range allLabels {
		var styled string
		if i == activeInner {
			styled = st.InnerActive.Render(label)
		} else {
			styled = st.InnerInactive.Render(label)
		}
		if i > 0 {
			parts = append(parts, sep)
		}
		parts = append(parts, zm.Mark(InnerTabZoneID(i), styled))
	}

	row := strings.Join(parts, "")
	rowWidth := lipgloss.Width(row)
	if rowWidth < totalWidth {
		row += st.InnerBarBg.Render(strings.Repeat(" ", totalWidth-rowWidth))
	}
	return row
}
