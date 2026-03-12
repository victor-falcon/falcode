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

// workspaceLabel returns the key string to display next to workspace tab i
// when show_workspace_numbers is enabled. It looks up the keybind that fires
// go_to_workspace for that index; if none is found it falls back to the
// 1-based position number (e.g. "10").
func workspaceLabel(keybinds *config.KeybindsConfig, i int) string {
	if keybinds != nil {
		if key := config.FindDirectKey(keybinds.Bindings, config.ActionGoToWorkspace, i); key != "" {
			return key
		}
	}
	return fmt.Sprintf("%d", i+1)
}

// tabLabel returns the key string to display next to inner tab i when
// show_tab_numbers is enabled. It looks up the keybind that fires go_to_tab
// for that index; if none is found it falls back to the 1-based position
// number (e.g. "10").
func tabLabel(keybinds *config.KeybindsConfig, i int) string {
	if keybinds != nil {
		if key := config.FindDirectKey(keybinds.Bindings, config.ActionGoToTab, i); key != "" {
			return key
		}
	}
	return fmt.Sprintf("%d", i+1)
}

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
// tabSpinners maps workspace index to a spinner character for any tab that
// should show an animated spinner in place of its × close button.
// wsAgentStatuses maps workspace index to the most urgent agent status for that
// workspace. agentSpinnerView is the current frame of the agent spinner.
func RenderTabBar(
	zm *zone.Manager,
	worktrees []*git.Worktree,
	innerTabs []*config.Tab,
	extraTabs []string, // dynamically added console tab labels
	closedCfgTabs map[int]bool, // per-workspace hidden built-in tab indices
	renamedCfgTabs map[int]string, // per-workspace name overrides for built-in tabs
	activeWS, activeInner int,
	totalWidth int,
	prefixMode bool,
	statusMsg string,
	ui *config.UIConfig,
	keybinds *config.KeybindsConfig,
	st uiStyles,
	tabSpinners map[int]string,
	wsAgentStatuses map[int]AgentStatus,
	agentSpinnerView string,
) string {
	if ui.GetCompactTabs() {
		return renderCompactRow(zm, worktrees, innerTabs, extraTabs, closedCfgTabs, renamedCfgTabs,
			activeWS, activeInner, totalWidth, prefixMode, statusMsg, ui, keybinds, st, tabSpinners,
			wsAgentStatuses, agentSpinnerView)
	}
	wsRow := renderWorkspaceRow(zm, worktrees, activeWS, totalWidth, prefixMode, statusMsg, ui, keybinds, st, tabSpinners, wsAgentStatuses, agentSpinnerView)
	innerRow := renderInnerRow(zm, innerTabs, extraTabs, closedCfgTabs, renamedCfgTabs, activeInner, totalWidth, ui, keybinds, st)
	return lipgloss.JoinVertical(lipgloss.Left, wsRow, innerRow)
}

// renderWorkspaceRow renders the top row of workspace (outer) tabs.
// tabSpinners maps workspace index to a spinner character; matched tabs show
// the spinner in place of their × close button.
// wsAgentStatuses maps workspace index to the most urgent agent status for
// that workspace. agentSpinnerView is the current spinner frame.
func renderWorkspaceRow(
	zm *zone.Manager,
	worktrees []*git.Worktree,
	activeWS, totalWidth int,
	prefixMode bool,
	statusMsg string,
	ui *config.UIConfig,
	keybinds *config.KeybindsConfig,
	st uiStyles,
	tabSpinners map[int]string,
	wsAgentStatuses map[int]AgentStatus,
	agentSpinnerView string,
) string {
	closeMode := ui.GetCloseWorkspaceButton()

	var tabs []string
	for i, wt := range worktrees {
		wsPrefix := ""
		if ui.GetShowWorkspaceNumbers() {
			wsPrefix = workspaceLabel(keybinds, i)
		}
		wsName := wt.Name()
		isActive := i == activeWS

		// buildWSContent assembles the visible label. When wsPrefix is non-empty
		// the number is rendered with the dim num-style; the name uses the base tab
		// style. padRight controls whether right padding is added (omitted when a
		// close button follows immediately after).
		buildWSContent := func(tabStyle, numStyle lipgloss.Style, padRight bool) string {
			if wsPrefix == "" {
				if padRight {
					return tabStyle.Render(wsName)
				}
				return tabStyle.PaddingRight(0).Render(wsName)
			}
			numS := numStyle.PaddingLeft(1).PaddingRight(0)
			nameS := tabStyle.PaddingLeft(0).PaddingRight(0)
			if padRight {
				nameS = nameS.PaddingRight(1)
			}
			return numS.Render(wsPrefix) + nameS.Render(" "+wsName)
		}

		// agentIconStr returns the styled agent status icon for this workspace,
		// or "" when the status is Idle (no icon shown).
		agentIconStr := func(tabBg lipgloss.Style) string {
			status := wsAgentStatuses[i]
			bg := tabBg.GetBackground()
			pad := tabBg.PaddingLeft(0).PaddingRight(0)
			switch status {
			case AgentStatusWorking:
				return st.AgentWorking.Background(bg).PaddingLeft(1).PaddingRight(0).Render(agentSpinnerView)
			case AgentStatusPermission:
				return st.AgentPermission.Background(bg).PaddingLeft(1).PaddingRight(0).Render("!") + pad.Render(" ")
			case AgentStatusQuestion:
				return st.AgentQuestion.Background(bg).PaddingLeft(1).PaddingRight(0).Render("?") + pad.Render(" ")
			case AgentStatusDone:
				return st.AgentDone.Background(bg).PaddingLeft(1).PaddingRight(0).Render("✓") + pad.Render(" ")
			}
			return ""
		}

		// The × is only meaningful on deletable workspaces (non-main, not the last one).
		canClose := !wt.IsMain && len(worktrees) > 1 &&
			(closeMode == config.CloseWorkspaceButtonAll ||
				(closeMode == config.CloseWorkspaceButtonFocus && isActive))

		var tabStr string
		if spinnerChar, ok := tabSpinners[i]; ok {
			// Tab has an active spinner: show it in place of ×.
			// The name part is still clickable (for workspace switching).
			if isActive {
				namePart := zm.Mark(WorkspaceTabZoneID(i), buildWSContent(st.WorkspaceActive, st.WorkspaceTabNumActive, false))
				agentIcon := agentIconStr(st.WorkspaceActive)
				spinnerPart := st.WorkspaceActive.Bold(false).PaddingLeft(0).PaddingRight(0).Render(" " + spinnerChar)
				tabStr = namePart + agentIcon + spinnerPart
			} else {
				namePart := zm.Mark(WorkspaceTabZoneID(i), buildWSContent(st.WorkspaceInactive, st.WorkspaceTabNumInactive, false))
				agentIcon := agentIconStr(st.WorkspaceInactive)
				spinnerPart := st.WorkspaceInactive.PaddingLeft(0).PaddingRight(0).Render(" " + spinnerChar)
				tabStr = namePart + agentIcon + spinnerPart
			}
		} else if canClose {
			if isActive {
				namePart := zm.Mark(WorkspaceTabZoneID(i), buildWSContent(st.WorkspaceActive, st.WorkspaceTabNumActive, false))
				agentIcon := agentIconStr(st.WorkspaceActive)
				closePart := zm.Mark(WorkspaceCloseZoneID(i), st.WorkspaceActive.Bold(false).PaddingLeft(0).Render(" ×"))
				tabStr = namePart + agentIcon + closePart
			} else {
				namePart := zm.Mark(WorkspaceTabZoneID(i), buildWSContent(st.WorkspaceInactive, st.WorkspaceTabNumInactive, false))
				agentIcon := agentIconStr(st.WorkspaceInactive)
				closePart := zm.Mark(WorkspaceCloseZoneID(i), st.WorkspaceInactive.PaddingLeft(0).Render(" ×"))
				tabStr = namePart + agentIcon + closePart
			}
		} else {
			if isActive {
				icon := agentIconStr(st.WorkspaceActive)
				if icon != "" {
					namePart := zm.Mark(WorkspaceTabZoneID(i), buildWSContent(st.WorkspaceActive, st.WorkspaceTabNumActive, false))
					tabStr = namePart + icon
				} else {
					tabStr = zm.Mark(WorkspaceTabZoneID(i), buildWSContent(st.WorkspaceActive, st.WorkspaceTabNumActive, true))
				}
			} else {
				icon := agentIconStr(st.WorkspaceInactive)
				if icon != "" {
					namePart := zm.Mark(WorkspaceTabZoneID(i), buildWSContent(st.WorkspaceInactive, st.WorkspaceTabNumInactive, false))
					tabStr = namePart + icon
				} else {
					tabStr = zm.Mark(WorkspaceTabZoneID(i), buildWSContent(st.WorkspaceInactive, st.WorkspaceTabNumInactive, true))
				}
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
	renamedCfgTabs map[int]string, // per-workspace name overrides for built-in tabs
	activeInner, totalWidth int,
	ui *config.UIConfig,
	keybinds *config.KeybindsConfig,
	st uiStyles,
) string {
	closeMode := ui.GetCloseTabButton()
	showNewTab := ui.GetNewTabButton()

	sep := st.InnerSeparator.Render("│")

	// logicalIdx tracks the absolute tab index (used for zone IDs and activeInner).
	logicalIdx := 0
	first := true

	var parts []string

	renderTab := func(prefix, name string, showClose bool) {
		isActive := logicalIdx == activeInner
		i := logicalIdx

		// buildContent assembles the visible label. When prefix is non-empty the
		// letter is rendered with the dim num-style; the name uses the base tab
		// style. padRight controls whether right padding is added (omitted when a
		// close button follows immediately after).
		buildContent := func(tabStyle, numStyle lipgloss.Style, padRight bool) string {
			if prefix == "" {
				if padRight {
					return tabStyle.Render(name)
				}
				return tabStyle.PaddingRight(0).Render(name)
			}
			numS := numStyle.PaddingLeft(1).PaddingRight(0)
			nameS := tabStyle.PaddingLeft(0).PaddingRight(0)
			if padRight {
				nameS = nameS.PaddingRight(1)
			}
			return numS.Render(prefix) + nameS.Render(" "+name)
		}

		var tabPart string
		if showClose {
			if isActive {
				content := buildContent(st.InnerActive, st.InnerTabNumActive, false)
				namePart := zm.Mark(InnerTabZoneID(i), content)
				closePart := zm.Mark(InnerTabCloseZoneID(i), st.InnerActive.Bold(false).PaddingLeft(0).Render(" ×"))
				tabPart = namePart + closePart
			} else {
				content := buildContent(st.InnerInactive, st.InnerTabNumInactive, false)
				namePart := zm.Mark(InnerTabZoneID(i), content)
				closePart := zm.Mark(InnerTabCloseZoneID(i), st.InnerInactive.PaddingLeft(0).Render(" ×"))
				tabPart = namePart + closePart
			}
		} else {
			var content string
			if isActive {
				content = buildContent(st.InnerActive, st.InnerTabNumActive, true)
			} else {
				content = buildContent(st.InnerInactive, st.InnerTabNumInactive, true)
			}
			tabPart = zm.Mark(InnerTabZoneID(i), content)
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
		prefix := ""
		if ui.GetShowTabNumbers() {
			prefix = tabLabel(keybinds, logicalIdx)
		}
		name := t.Name
		if renamedCfgTabs != nil {
			if rn, ok := renamedCfgTabs[i]; ok {
				name = rn
			}
		}
		renderTab(prefix, name, canClose)
	}

	// Extra (dynamically created) tabs — always closeable when closeMode allows.
	cfgCount := len(cfgTabs)
	for j, name := range extraTabs {
		logicalIdx = cfgCount + j
		isActive := logicalIdx == activeInner
		showClose := closeMode == config.CloseTabButtonAll ||
			(closeMode == config.CloseTabButtonFocus && isActive)
		prefix := ""
		if ui.GetShowTabNumbers() {
			prefix = tabLabel(keybinds, logicalIdx)
		}
		renderTab(prefix, name, showClose)
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
// deletingWSIdx is the index of the workspace being deleted (-1 if none);
// spinnerChar replaces the × close button on that tab.
func renderCompactRow(
	zm *zone.Manager,
	worktrees []*git.Worktree,
	cfgTabs []*config.Tab,
	extraTabs []string,
	closedCfgTabs map[int]bool,
	renamedCfgTabs map[int]string, // per-workspace name overrides for built-in tabs
	activeWS, activeInner, totalWidth int,
	prefixMode bool,
	statusMsg string,
	ui *config.UIConfig,
	keybinds *config.KeybindsConfig,
	st uiStyles,
	tabSpinners map[int]string,
	wsAgentStatuses map[int]AgentStatus,
	agentSpinnerView string,
) string {
	closeWSMode := ui.GetCloseWorkspaceButton()
	closeTabMode := ui.GetCloseTabButton()
	showNewTab := ui.GetNewTabButton()
	showNewWS := ui.GetNewWorkspaceButton()

	// agentIconStr returns the styled agent status icon for workspace i,
	// rendered against the given tab background style.
	agentIconStr := func(i int, tabBg lipgloss.Style) string {
		status := wsAgentStatuses[i]
		bg := tabBg.GetBackground()
		pad := tabBg.PaddingLeft(0).PaddingRight(0)
		switch status {
		case AgentStatusWorking:
			return pad.Render(" ") +
				st.AgentWorking.Background(bg).Render(agentSpinnerView)
		case AgentStatusPermission:
			return pad.Render(" ") +
				st.AgentPermission.Background(bg).Render("!") + pad.Render(" ")
		case AgentStatusQuestion:
			return pad.Render(" ") +
				st.AgentQuestion.Background(bg).Render("?") + pad.Render(" ")
		case AgentStatusDone:
			return pad.Render(" ") +
				st.AgentDone.Background(bg).Render("✓") + pad.Render(" ")
		}
		return ""
	}

	// renderWSTab builds a single workspace tab string (active or inactive).
	renderWSTab := func(i int, wt *git.Worktree, isActive bool) string {
		wsPrefix := ""
		if ui.GetShowWorkspaceNumbers() {
			wsPrefix = workspaceLabel(keybinds, i)
		}
		wsName := wt.Name()
		buildWSContent := func(tabStyle, numStyle lipgloss.Style, padRight bool) string {
			if wsPrefix == "" {
				if padRight {
					return tabStyle.Render(wsName)
				}
				return tabStyle.PaddingRight(0).Render(wsName)
			}
			numS := numStyle.PaddingLeft(1).PaddingRight(0)
			nameS := tabStyle.PaddingLeft(0).PaddingRight(0)
			if padRight {
				nameS = nameS.PaddingRight(1)
			}
			return numS.Render(wsPrefix) + nameS.Render(" "+wsName)
		}
		canClose := !wt.IsMain && len(worktrees) > 1 &&
			(closeWSMode == config.CloseWorkspaceButtonAll ||
				(closeWSMode == config.CloseWorkspaceButtonFocus && isActive))

		if spinnerChar, ok := tabSpinners[i]; ok {
			// Tab has an active spinner: show it in place of ×. Name still clickable.
			if isActive {
				name := zm.Mark(WorkspaceTabZoneID(i), buildWSContent(st.WorkspaceActive, st.WorkspaceTabNumActive, false))
				icon := agentIconStr(i, st.WorkspaceActive)
				sp := st.WorkspaceActive.Bold(false).PaddingLeft(0).PaddingRight(0).Render(" " + spinnerChar)
				return name + icon + sp
			}
			name := zm.Mark(WorkspaceTabZoneID(i), buildWSContent(st.WorkspaceInactive, st.WorkspaceTabNumInactive, false))
			icon := agentIconStr(i, st.WorkspaceInactive)
			sp := st.WorkspaceInactive.PaddingLeft(0).PaddingRight(0).Render(" " + spinnerChar)
			return name + icon + sp
		}
		if canClose {
			if isActive {
				name := zm.Mark(WorkspaceTabZoneID(i), buildWSContent(st.WorkspaceActive, st.WorkspaceTabNumActive, false))
				icon := agentIconStr(i, st.WorkspaceActive)
				close := zm.Mark(WorkspaceCloseZoneID(i), st.WorkspaceActive.Bold(false).PaddingLeft(0).Render(" ×"))
				return name + icon + close
			}
			name := zm.Mark(WorkspaceTabZoneID(i), buildWSContent(st.WorkspaceInactive, st.WorkspaceTabNumInactive, false))
			icon := agentIconStr(i, st.WorkspaceInactive)
			close := zm.Mark(WorkspaceCloseZoneID(i), st.WorkspaceInactive.PaddingLeft(0).Render(" ×"))
			return name + icon + close
		}
		if isActive {
			icon := agentIconStr(i, st.WorkspaceActive)
			if icon != "" {
				name := zm.Mark(WorkspaceTabZoneID(i), buildWSContent(st.WorkspaceActive, st.WorkspaceTabNumActive, false))
				return name + icon
			}
			return zm.Mark(WorkspaceTabZoneID(i), buildWSContent(st.WorkspaceActive, st.WorkspaceTabNumActive, true))
		}
		icon := agentIconStr(i, st.WorkspaceInactive)
		if icon != "" {
			name := zm.Mark(WorkspaceTabZoneID(i), buildWSContent(st.WorkspaceInactive, st.WorkspaceTabNumInactive, false))
			return name + icon
		}
		return zm.Mark(WorkspaceTabZoneID(i), buildWSContent(st.WorkspaceInactive, st.WorkspaceTabNumInactive, true))
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

	appendInnerTab := func(prefix, name string, showClose bool) {
		isActive := logicalIdx == activeInner
		i := logicalIdx

		buildContent := func(tabStyle, numStyle lipgloss.Style, padRight bool) string {
			if prefix == "" {
				if padRight {
					return tabStyle.Render(name)
				}
				return tabStyle.PaddingRight(0).Render(name)
			}
			numS := numStyle.PaddingLeft(1).PaddingRight(0)
			nameS := tabStyle.PaddingLeft(0).PaddingRight(0)
			if padRight {
				nameS = nameS.PaddingRight(1)
			}
			return numS.Render(prefix) + nameS.Render(" "+name)
		}

		var tabPart string
		if showClose {
			if isActive {
				content := buildContent(st.InnerActive, st.InnerTabNumActive, false)
				tabName := zm.Mark(InnerTabZoneID(i), content)
				cl := zm.Mark(InnerTabCloseZoneID(i), st.InnerActive.Bold(false).PaddingLeft(0).Render(" ×"))
				tabPart = tabName + cl
			} else {
				content := buildContent(st.InnerInactive, st.InnerTabNumInactive, false)
				tabName := zm.Mark(InnerTabZoneID(i), content)
				cl := zm.Mark(InnerTabCloseZoneID(i), st.InnerInactive.PaddingLeft(0).Render(" ×"))
				tabPart = tabName + cl
			}
		} else {
			if isActive {
				tabPart = zm.Mark(InnerTabZoneID(i), buildContent(st.InnerActive, st.InnerTabNumActive, true))
			} else {
				tabPart = zm.Mark(InnerTabZoneID(i), buildContent(st.InnerInactive, st.InnerTabNumInactive, true))
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
		prefix := ""
		if ui.GetShowTabNumbers() {
			prefix = tabLabel(keybinds, logicalIdx)
		}
		name := t.Name
		if renamedCfgTabs != nil {
			if rn, ok := renamedCfgTabs[i]; ok {
				name = rn
			}
		}
		appendInnerTab(prefix, name, canClose)
	}

	cfgCount := len(cfgTabs)
	for j, name := range extraTabs {
		logicalIdx = cfgCount + j
		isActive := logicalIdx == activeInner
		showClose := closeTabMode == config.CloseTabButtonAll ||
			(closeTabMode == config.CloseTabButtonFocus && isActive)
		prefix := ""
		if ui.GetShowTabNumbers() {
			prefix = tabLabel(keybinds, logicalIdx)
		}
		appendInnerTab(prefix, name, showClose)
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
