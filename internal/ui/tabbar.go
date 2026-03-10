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
	zoneNewWorkspaceBtn = "new-ws-btn"
)

// WorkspaceTabZoneID returns the bubblezone ID for a workspace tab.
func WorkspaceTabZoneID(idx int) string { return fmt.Sprintf("%s%d", zoneWorkspacePrefix, idx) }

// InnerTabZoneID returns the bubblezone ID for an inner tab.
func InnerTabZoneID(idx int) string { return fmt.Sprintf("%s%d", zoneInnerPrefix, idx) }

// InnerTabCloseZoneID returns the bubblezone ID for an inner tab's × button.
func InnerTabCloseZoneID(idx int) string { return fmt.Sprintf("%s%d", zoneInnerClose, idx) }

// NewTabBtnZoneID returns the bubblezone ID for the + new-tab button.
func NewTabBtnZoneID() string { return zoneNewTabBtn }

// NewWorkspaceBtnZoneID returns the bubblezone ID for the + new-workspace button.
func NewWorkspaceBtnZoneID() string { return zoneNewWorkspaceBtn }

// TabBarHeight returns the number of rows the tab bar occupies (2: workspace + inner).
func TabBarHeight() int { return 2 }

// RenderTabBar renders the full two-row tab bar.
func RenderTabBar(
	zm *zone.Manager,
	worktrees []*git.Worktree,
	innerTabs []*config.Tab,
	extraTabs []string, // dynamically added console tab labels
	closedCfgTabs map[int]bool, // per-workspace hidden built-in tab indices
	activeWS, activeInner int,
	totalWidth int,
	prefixMode bool,
	statusMsg string,
	ui *config.UIConfig,
	st uiStyles,
) string {
	wsRow := renderWorkspaceRow(zm, worktrees, activeWS, totalWidth, prefixMode, statusMsg, ui, st)
	innerRow := renderInnerRow(zm, innerTabs, extraTabs, closedCfgTabs, activeInner, totalWidth, ui, st)
	return lipgloss.JoinVertical(lipgloss.Left, wsRow, innerRow)
}

// renderWorkspaceRow renders the top row of workspace (outer) tabs.
func renderWorkspaceRow(
	zm *zone.Manager,
	worktrees []*git.Worktree,
	activeWS, totalWidth int,
	prefixMode bool,
	statusMsg string,
	ui *config.UIConfig,
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

	// + new-workspace button immediately after the workspace tabs.
	if ui.GetNewWorkspaceButton() {
		newWSPart := zm.Mark(NewWorkspaceBtnZoneID(), st.WorkspaceInactive.Render("+"))
		tabsStr += newWSPart
	}

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
	closedCfgTabs map[int]bool, // which built-in tab indices are hidden this workspace
	activeInner, totalWidth int,
	ui *config.UIConfig,
	st uiStyles,
) string {
	closeMode := ui.GetCloseTabButton()
	showNewTab := ui.GetNewTabButton()

	sep := st.InnerSeparator.Render("│")

	// logicalIdx tracks the absolute tab index (used for zone IDs and activeInner).
	logicalIdx := 0
	first := true

	var parts []string

	renderTab := func(label string, showClose bool) {
		isActive := logicalIdx == activeInner
		i := logicalIdx

		var tabPart string
		if showClose {
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

		if !first {
			parts = append(parts, sep)
		}
		parts = append(parts, tabPart)
		first = false
	}

	// Built-in (config) tabs.
	for i, t := range cfgTabs {
		logicalIdx = i
		if closedCfgTabs[i] {
			continue // hidden for this workspace — skip rendering, keep logical index
		}
		isActive := logicalIdx == activeInner
		canClose := t.IsInteractive() && (closeMode == config.CloseTabButtonAll ||
			(closeMode == config.CloseTabButtonFocus && isActive))
		renderTab(t.Name, canClose)
	}

	// Extra (dynamically created) tabs — always closeable when closeMode allows.
	cfgCount := len(cfgTabs)
	for j, label := range extraTabs {
		logicalIdx = cfgCount + j
		isActive := logicalIdx == activeInner
		showClose := closeMode == config.CloseTabButtonAll ||
			(closeMode == config.CloseTabButtonFocus && isActive)
		renderTab(label, showClose)
	}

	// + new-tab button at the end.
	if showNewTab {
		newTabPart := zm.Mark(NewTabBtnZoneID(), st.InnerInactive.Render("+"))
		if !first {
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
