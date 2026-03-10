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
	zoneWorkspaceClose  = "ws-close-"
	zoneInnerPrefix     = "it-"
	zoneInnerClose      = "it-close-"
	zoneNewTabBtn       = "new-tab-btn"
	zoneNewWorkspaceBtn = "new-ws-btn"
)

// WorkspaceTabZoneID returns the bubblezone ID for a workspace tab.
func WorkspaceTabZoneID(idx int) string { return fmt.Sprintf("%s%d", zoneWorkspacePrefix, idx) }

// WorkspaceCloseZoneID returns the bubblezone ID for a workspace tab's × button.
func WorkspaceCloseZoneID(idx int) string { return fmt.Sprintf("%s%d", zoneWorkspaceClose, idx) }

// InnerTabZoneID returns the bubblezone ID for an inner tab.
func InnerTabZoneID(idx int) string { return fmt.Sprintf("%s%d", zoneInnerPrefix, idx) }

// InnerTabCloseZoneID returns the bubblezone ID for an inner tab's × button.
func InnerTabCloseZoneID(idx int) string { return fmt.Sprintf("%s%d", zoneInnerClose, idx) }

// NewTabBtnZoneID returns the bubblezone ID for the + new-tab button.
func NewTabBtnZoneID() string { return zoneNewTabBtn }

// NewWorkspaceBtnZoneID returns the bubblezone ID for the + new-workspace button.
func NewWorkspaceBtnZoneID() string { return zoneNewWorkspaceBtn }

// TabBarHeight returns the number of rows the tab bar occupies.
// In compact mode it is 1 (unified row); otherwise it is 2 (workspace + inner).
func TabBarHeight(ui *config.UIConfig) int {
	if ui.GetCompactTabs() {
		return 1
	}
	return 2
}

// RenderTabBar renders the tab bar. In compact mode a single unified row is
// produced; otherwise the classic two-row layout (workspace + inner) is used.
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
	if ui.GetCompactTabs() {
		return renderCompactRow(zm, worktrees, innerTabs, extraTabs, closedCfgTabs,
			activeWS, activeInner, totalWidth, prefixMode, statusMsg, ui, st)
	}
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
	closeMode := ui.GetCloseWorkspaceButton()

	var tabs []string
	for i, wt := range worktrees {
		label := wt.Name()
		isActive := i == activeWS
		// The × is only meaningful on deletable workspaces (non-main, not the last one).
		canClose := !wt.IsMain && len(worktrees) > 1 &&
			(closeMode == config.CloseWorkspaceButtonAll ||
				(closeMode == config.CloseWorkspaceButtonFocus && isActive))

		var tabStr string
		if canClose {
			if isActive {
				namePart := zm.Mark(WorkspaceTabZoneID(i), st.WorkspaceActive.PaddingRight(0).Render(label))
				closePart := zm.Mark(WorkspaceCloseZoneID(i), st.WorkspaceActive.Bold(false).PaddingLeft(0).Render(" ×"))
				tabStr = namePart + closePart
			} else {
				namePart := zm.Mark(WorkspaceTabZoneID(i), st.WorkspaceInactive.PaddingRight(0).Render(label))
				closePart := zm.Mark(WorkspaceCloseZoneID(i), st.WorkspaceInactive.PaddingLeft(0).Render(" ×"))
				tabStr = namePart + closePart
			}
		} else {
			if isActive {
				tabStr = zm.Mark(WorkspaceTabZoneID(i), st.WorkspaceActive.Render(label))
			} else {
				tabStr = zm.Mark(WorkspaceTabZoneID(i), st.WorkspaceInactive.Render(label))
			}
		}
		tabs = append(tabs, tabStr)
	}

	tabsStr := strings.Join(tabs, "")

	// + new-workspace button immediately after the workspace tabs.
	if ui.GetNewWorkspaceButton() {
		newWSPart := zm.Mark(NewWorkspaceBtnZoneID(), st.NewWorkspaceBtn.Render("+"))
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
		newTabPart := zm.Mark(NewTabBtnZoneID(), st.NewTabBtn.Render("+"))
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

// renderCompactRow renders a single unified tab row combining workspace tabs
// and inner tabs. The layout is:
//
//	[ws-before...] [active-ws] inner-tab │ inner-tab │ + [ws-after...]  gap  +
//
// Inactive workspace tabs that precede the active workspace are shown on the
// left; those that follow are shown on the right, before the trailing gap.
// The new-workspace + button is anchored to the far right.
func renderCompactRow(
	zm *zone.Manager,
	worktrees []*git.Worktree,
	cfgTabs []*config.Tab,
	extraTabs []string,
	closedCfgTabs map[int]bool,
	activeWS, activeInner, totalWidth int,
	prefixMode bool,
	statusMsg string,
	ui *config.UIConfig,
	st uiStyles,
) string {
	closeWSMode := ui.GetCloseWorkspaceButton()
	closeTabMode := ui.GetCloseTabButton()
	showNewTab := ui.GetNewTabButton()
	showNewWS := ui.GetNewWorkspaceButton()

	// renderWSTab builds a single workspace tab string (active or inactive).
	renderWSTab := func(i int, wt *git.Worktree, isActive bool) string {
		label := wt.Name()
		canClose := !wt.IsMain && len(worktrees) > 1 &&
			(closeWSMode == config.CloseWorkspaceButtonAll ||
				(closeWSMode == config.CloseWorkspaceButtonFocus && isActive))
		if canClose {
			if isActive {
				name := zm.Mark(WorkspaceTabZoneID(i), st.WorkspaceActive.PaddingRight(0).Render(label))
				close := zm.Mark(WorkspaceCloseZoneID(i), st.WorkspaceActive.Bold(false).PaddingLeft(0).Render(" ×"))
				return name + close
			}
			name := zm.Mark(WorkspaceTabZoneID(i), st.WorkspaceInactive.PaddingRight(0).Render(label))
			close := zm.Mark(WorkspaceCloseZoneID(i), st.WorkspaceInactive.PaddingLeft(0).Render(" ×"))
			return name + close
		}
		if isActive {
			return zm.Mark(WorkspaceTabZoneID(i), st.WorkspaceActive.Render(label))
		}
		return zm.Mark(WorkspaceTabZoneID(i), st.WorkspaceInactive.Render(label))
	}

	var parts []string
	sep := st.InnerSeparator.Render("│")

	// wsFirst tracks whether we've emitted any element yet (for leading separator logic).
	wsFirst := true

	appendWS := func(tab string) {
		if !wsFirst {
			parts = append(parts, sep)
		}
		parts = append(parts, tab)
		wsFirst = false
	}

	// 1. Workspace tabs that precede the active workspace.
	for i, wt := range worktrees {
		if i < activeWS {
			appendWS(renderWSTab(i, wt, false))
		}
	}

	// 2. Active workspace tab.
	if activeWS < len(worktrees) {
		appendWS(renderWSTab(activeWS, worktrees[activeWS], true))
	}

	// 3. Inner (per-workspace) tabs with │ separators between them.
	// innerFirst is separate: we always emit a separator before the first inner
	// tab to visually separate the workspace section from the inner tab section,
	// but only when there is at least one workspace tab already rendered.
	innerFirst := true
	logicalIdx := 0

	appendInnerTab := func(label string, showClose bool) {
		isActive := logicalIdx == activeInner
		i := logicalIdx

		var tabPart string
		if showClose {
			if isActive {
				name := zm.Mark(InnerTabZoneID(i), st.InnerActive.PaddingRight(0).Render(label))
				cl := zm.Mark(InnerTabCloseZoneID(i), st.InnerActive.Bold(false).PaddingLeft(0).Render(" ×"))
				tabPart = name + cl
			} else {
				name := zm.Mark(InnerTabZoneID(i), st.InnerInactive.PaddingRight(0).Render(label))
				cl := zm.Mark(InnerTabCloseZoneID(i), st.InnerInactive.PaddingLeft(0).Render(" ×"))
				tabPart = name + cl
			}
		} else {
			if isActive {
				tabPart = zm.Mark(InnerTabZoneID(i), st.InnerActive.Render(label))
			} else {
				tabPart = zm.Mark(InnerTabZoneID(i), st.InnerInactive.Render(label))
			}
		}
		// Separator before this inner tab: between inner tabs OR between the
		// last workspace tab and the first inner tab.
		if !innerFirst || !wsFirst {
			parts = append(parts, sep)
		}
		parts = append(parts, tabPart)
		innerFirst = false
		wsFirst = false
	}

	for i, t := range cfgTabs {
		logicalIdx = i
		if closedCfgTabs[i] {
			continue
		}
		isActive := logicalIdx == activeInner
		canClose := t.IsInteractive() && (closeTabMode == config.CloseTabButtonAll ||
			(closeTabMode == config.CloseTabButtonFocus && isActive))
		appendInnerTab(t.Name, canClose)
	}

	cfgCount := len(cfgTabs)
	for j, label := range extraTabs {
		logicalIdx = cfgCount + j
		isActive := logicalIdx == activeInner
		showClose := closeTabMode == config.CloseTabButtonAll ||
			(closeTabMode == config.CloseTabButtonFocus && isActive)
		appendInnerTab(label, showClose)
	}

	// 4. + new-tab button (with a │ separator before it).
	if showNewTab {
		newTabPart := zm.Mark(NewTabBtnZoneID(), st.NewTabBtn.Render("+"))
		if !innerFirst || !wsFirst {
			parts = append(parts, sep)
		}
		parts = append(parts, newTabPart)
	}

	// 5. Workspace tabs that follow the active workspace (with separators).
	for i, wt := range worktrees {
		if i > activeWS {
			parts = append(parts, sep)
			parts = append(parts, renderWSTab(i, wt, false))
		}
	}

	row := strings.Join(parts, "")
	rowWidth := lipgloss.Width(row)

	// Right-hand elements: optional indicator and new-workspace button.
	indicator := ""
	if prefixMode {
		indicator = st.PrefixIndicator.Render(" [PREFIX] ")
	} else if statusMsg != "" {
		indicator = st.StatusMsg.Render(" " + statusMsg + " ")
	}

	newWSPart := ""
	if showNewWS {
		newWSPart = zm.Mark(NewWorkspaceBtnZoneID(), st.NewWorkspaceBtn.Render("+"))
	}

	// 6. Gap fills the space between the tab content and the right-side elements.
	remainingWidth := totalWidth - rowWidth - lipgloss.Width(indicator) - lipgloss.Width(newWSPart)
	if remainingWidth < 0 {
		remainingWidth = 0
	}
	gap := st.WorkspaceBarBg.Render(strings.Repeat(" ", remainingWidth))

	return row + gap + indicator + newWSPart
}
