package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"
	"github.com/victor-falcon/falcode/internal/config"
	"github.com/victor-falcon/falcode/internal/git"
)

// layerState is a pushed which-key layer on the navigation stack.
type layerState struct {
	bindings []*config.Keybind
	title    string
}

// animTickMsg drives the sheet spring animation.
type animTickMsg struct{}

// Model is the root bubbletea model.
type Model struct {
	cfg      *config.Config
	keybinds *config.KeybindsConfig
	styles   uiStyles
	zm       *zone.Manager

	// send is prog.Send, stored so lazy pane launches can dispatch messages.
	send func(tea.Msg)

	worktrees   []*git.Worktree
	repoRoot    string // main worktree absolute path
	activeWS    int    // active workspace (outer tab) index
	activeInner int    // active inner tab index

	// cfgTabs are the fixed tabs from config (shared across workspaces).
	// extraTabs[ws] lists dynamically-added console tab labels per workspace.
	// closedCfgTabs[ws] records which cfgTab indices have been hidden per-workspace;
	// cfgTabs itself is never mutated so extra-tab pane keys remain stable.
	cfgTabs       []*config.Tab
	extraTabs     [][]string
	closedCfgTabs []map[int]bool

	// panes is the PTY pane registry; lazily populated on first tab visit.
	panes map[PaneKey]*Pane

	width  int
	height int
	ready  bool

	// Build version shown in the footer.
	version string

	// Keybind / prefix mode state.
	prefixMode   bool
	layerStack   []layerState
	currentLayer []*config.Keybind
	layerTitle   string

	// Which-key sheet with spring animation.
	sheet *Sheet

	// Tab name prompt state.
	namingTab    bool
	tabNameInput textinput.Model

	// Status message displayed in the tab bar gap.
	statusMsg     string
	statusClearAt time.Time
}

// New creates the initial Model. Call SetSend before running the program.
func New(
	cfg *config.Config,
	keybinds *config.KeybindsConfig,
	theme *config.ThemeColors,
	worktrees []*git.Worktree,
	cols, rows int,
	version string,
) *Model {
	zm := zone.New()
	zm.SetEnabled(true)

	extraTabs := make([][]string, len(worktrees))
	for i := range extraTabs {
		extraTabs[i] = []string{}
	}

	closedCfgTabs := make([]map[int]bool, len(worktrees))
	for i := range closedCfgTabs {
		closedCfgTabs[i] = make(map[int]bool)
	}

	ti := textinput.New()
	ti.Placeholder = "tab name"
	ti.CharLimit = 32

	return &Model{
		cfg:           cfg,
		keybinds:      keybinds,
		styles:        newStyles(theme),
		zm:            zm,
		worktrees:     worktrees,
		repoRoot:      worktrees[0].Path,
		cfgTabs:       cfg.Tabs,
		extraTabs:     extraTabs,
		closedCfgTabs: closedCfgTabs,
		panes:         make(map[PaneKey]*Pane),
		width:         cols,
		height:        rows,
		sheet:         NewSheet(),
		tabNameInput:  ti,
		currentLayer:  keybinds.Bindings,
		layerTitle:    "falcode",
		version:       version,
	}
}

// SetSend stores the bubbletea program's Send function so that background
// goroutines (PTY readers) can dispatch messages into the event loop.
func (m *Model) SetSend(send func(tea.Msg)) {
	m.send = send
}

// StartAll eagerly starts the first visible pane.
func (m *Model) StartAll() {
	m.ensurePaneStarted(PaneKey{Workspace: 0, Tab: 0})
}

// StopAll terminates all running PTY panes.
func (m *Model) StopAll() {
	for _, p := range m.panes {
		p.Stop()
	}
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		for _, p := range m.panes {
			p.Resize(m.width, m.paneHeight())
		}
		return m, nil

	case PaneOutputMsg:
		// Output arrived; bubbletea will call View() to re-render.
		return m, nil

	case PaneExitMsg:
		// Non-interactive (command) panes show an in-pane restart banner, so we
		// only surface a status message when there is an actual error.
		if msg.Err != nil {
			m.setStatus(fmt.Sprintf("process exited: %v", msg.Err))
		}
		return m, nil

	case animTickMsg:
		done := m.sheet.Tick()
		if done {
			return m, nil
		}
		return m, animTick()

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

// View implements tea.Model.
func (m *Model) View() string {
	if !m.ready {
		return "Initializing…\n"
	}

	tabBar := RenderTabBar(
		m.zm,
		m.worktrees,
		m.cfgTabs,
		m.extraTabs[m.activeWS],
		m.closedCfgTabs[m.activeWS],
		m.activeWS,
		m.activeInner,
		m.width,
		m.prefixMode,
		m.currentStatus(),
		m.cfg.UI,
		m.styles,
	)

	paneContent := ""
	if pane := m.activePane(); pane != nil {
		paneContent = pane.View()
	}

	// Ensure pane content fills the full pane height.
	paneLines := strings.Split(paneContent, "\n")
	for len(paneLines) < m.paneHeight() {
		paneLines = append(paneLines, strings.Repeat(" ", m.width))
	}
	paneContent = strings.Join(paneLines, "\n")

	// Overlay a restart banner centered over the pane area when a
	// non-interactive (command) pane has stopped. This is done before joining
	// with the tab bar so the tab bar is never affected.
	if pane := m.activePane(); pane != nil && pane.Exited() && !pane.IsInteractive() {
		banner := m.styles.ExitBanner.Render("process stopped  ·  press Enter to restart")
		paneContent = overlayCentered(paneContent, banner, m.width, m.paneHeight())
	}

	// Overlay the tab name prompt when the user is creating a new tab.
	if m.namingTab {
		prompt := m.renderTabNamePrompt()
		paneContent = overlayCentered(paneContent, prompt, m.width, m.paneHeight())
	}

	view := tabBar + "\n" + paneContent

	// Footer: context hint (left) and build version (right).
	if !m.cfg.UI.GetHideFooter() {
		footer := RenderFooter(m.keybinds.Prefix, m.version, m.prefixMode, m.width, m.styles)
		view = view + "\n" + footer
	}

	// Overlay the which-key sheet if visible.
	if m.sheet.Visible() {
		sheetStr := RenderSheet(m.currentLayer, m.layerTitle, m.styles)
		sheetLines := strings.Split(sheetStr, "\n")
		offset := m.sheet.AnimOffset(len(sheetLines))
		view = overlayBottomRight(view, sheetStr, m.width, offset)
	}

	// Let bubblezone scan the output to record zone positions for mouse hits.
	return m.zm.Scan(view)
}

// ============================================================
// Key handling
// ============================================================

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Tab naming prompt intercepts all keys.
	if m.namingTab {
		switch msg.Type {
		case tea.KeyEnter:
			name := strings.TrimSpace(m.tabNameInput.Value())
			if name == "" {
				name = fmt.Sprintf("console-%d", len(m.extraTabs[m.activeWS])+1)
			}
			m.namingTab = false
			m.tabNameInput.Blur()
			m.addExtraTab(name)
			return m, m.ensurePaneCmd(PaneKey{Workspace: m.activeWS, Tab: m.activeInner})
		case tea.KeyEsc:
			m.namingTab = false
			m.tabNameInput.Blur()
			return m, nil
		}
		var cmd tea.Cmd
		m.tabNameInput, cmd = m.tabNameInput.Update(msg)
		return m, cmd
	}

	if m.prefixMode {
		return m.handleLayerKey(msg)
	}

	if m.matchesPrefixKey(msg) {
		m.enterPrefixMode()
		return m, animTick()
	}

	// Forward to active PTY.
	if pane := m.activePane(); pane != nil {
		// If a non-interactive command pane has stopped, Enter restarts it;
		// all other keys are swallowed until the process is running again.
		if pane.Exited() && !pane.IsInteractive() {
			if msg.Type == tea.KeyEnter {
				return m.restartPane(PaneKey{Workspace: m.activeWS, Tab: m.activeInner})
			}
			return m, nil
		}
		if b := keyToBytes(msg); b != nil {
			pane.Write(b)
		}
	}
	return m, nil
}

func (m *Model) handleLayerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// ESC always exits to lock mode immediately, regardless of layer depth.
	if msg.Type == tea.KeyEsc {
		m.exitPrefixMode()
		return m, nil
	}

	// Backspace navigates up one level, or exits to lock if already at root.
	if msg.Type == tea.KeyBackspace {
		if len(m.layerStack) > 0 {
			m.popLayer()
			return m, nil
		}
		m.exitPrefixMode()
		return m, nil
	}

	keyStr := keyMsgString(msg)
	for _, b := range m.currentLayer {
		if b.Key == keyStr || b.DisplayLabel() == keyStr {
			if b.IsGroup() {
				m.pushLayer(b.Bindings, b.Description)
				return m, nil
			}
			return m, m.executeAction(b)
		}
	}

	m.setStatus(fmt.Sprintf("unknown binding: %s", keyStr))
	m.exitPrefixMode()
	return m, nil
}

// ============================================================
// Mouse handling
// ============================================================

func (m *Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}

	// Workspace (outer) tabs.
	for i := range m.worktrees {
		if zi := m.zm.Get(WorkspaceTabZoneID(i)); zi != nil && zi.InBounds(msg) {
			return m, m.switchWorkspaceCmd(i)
		}
	}

	// Inner tabs.
	totalInner := len(m.cfgTabs) + len(m.extraTabs[m.activeWS])
	for i := 0; i < totalInner; i++ {
		if zi := m.zm.Get(InnerTabCloseZoneID(i)); zi != nil && zi.InBounds(msg) {
			m.closeTab(i)
			return m, nil
		}
		if zi := m.zm.Get(InnerTabZoneID(i)); zi != nil && zi.InBounds(msg) {
			m.switchInnerTab(i)
			return m, m.ensurePaneCmd(PaneKey{Workspace: m.activeWS, Tab: i})
		}
	}

	// + new-tab button.
	if zi := m.zm.Get(NewTabBtnZoneID()); zi != nil && zi.InBounds(msg) {
		m.tabNameInput.SetValue("")
		m.tabNameInput.Focus()
		m.namingTab = true
		return m, nil
	}

	return m, nil
}

// ============================================================
// Actions
// ============================================================

func (m *Model) executeAction(b *config.Keybind) tea.Cmd {
	var cmds []tea.Cmd

	for _, action := range b.ActionList() {
		switch action {
		case config.ActionLock:
			m.exitPrefixMode()
		case config.ActionQuit:
			// Quit terminates the program immediately; no further actions run.
			return tea.Quit
		case config.ActionNextTab:
			m.switchInnerTab(m.wrapInner(m.activeInner + 1))
			cmds = append(cmds, m.ensurePaneCmd(PaneKey{Workspace: m.activeWS, Tab: m.activeInner}))
		case config.ActionPrevTab:
			m.switchInnerTab(m.wrapInner(m.activeInner - 1))
			cmds = append(cmds, m.ensurePaneCmd(PaneKey{Workspace: m.activeWS, Tab: m.activeInner}))
		case config.ActionNewTab:
			m.tabNameInput.SetValue("")
			m.tabNameInput.Focus()
			m.namingTab = true
			m.exitPrefixMode()
		case config.ActionCloseTab:
			m.closeCurrentTab()
		case config.ActionNextWorkspace:
			cmds = append(cmds, m.switchWorkspaceCmd((m.activeWS+1)%len(m.worktrees)))
		case config.ActionPrevWorkspace:
			cmds = append(cmds, m.switchWorkspaceCmd(m.wrapWS(m.activeWS-1)))
		case config.ActionDeleteWorkspace:
			cmds = append(cmds, m.deleteWorkspaceCmd())
		case config.ActionPassthrough:
			m.passthroughPrefix()
		}
	}

	return tea.Batch(cmds...)
}

func (m *Model) switchWorkspaceCmd(idx int) tea.Cmd {
	if idx < 0 || idx >= len(m.worktrees) {
		return nil
	}
	m.activeWS = idx
	m.activeInner = 0
	return m.ensurePaneCmd(PaneKey{Workspace: m.activeWS, Tab: 0})
}

func (m *Model) switchInnerTab(idx int) {
	m.activeInner = idx
}

func (m *Model) addExtraTab(name string) {
	m.extraTabs[m.activeWS] = append(m.extraTabs[m.activeWS], name)
	m.activeInner = len(m.cfgTabs) + len(m.extraTabs[m.activeWS]) - 1
}

func (m *Model) closeCurrentTab() {
	m.closeTab(m.activeInner)
}

func (m *Model) closeTab(idx int) {
	if idx < len(m.cfgTabs) {
		// Built-in tab: only closeable when interactive (no command).
		tab := m.cfgTabs[idx]
		if !tab.IsInteractive() {
			m.setStatus("cannot close a tab with a command")
			return
		}
		// Stop the pane if running, then hide it per-workspace.
		key := PaneKey{Workspace: m.activeWS, Tab: idx}
		if p, ok := m.panes[key]; ok {
			p.Stop()
			delete(m.panes, key)
		}
		m.closedCfgTabs[m.activeWS][idx] = true
		// Move active tab to the nearest still-visible tab.
		if m.activeInner == idx {
			m.activeInner = m.prevVisibleTab(m.activeWS, idx)
		}
		return
	}

	// Extra tab.
	extraIdx := idx - len(m.cfgTabs)
	key := PaneKey{Workspace: m.activeWS, Tab: idx}
	if p, ok := m.panes[key]; ok {
		p.Stop()
		delete(m.panes, key)
	}
	extra := m.extraTabs[m.activeWS]
	m.extraTabs[m.activeWS] = append(extra[:extraIdx], extra[extraIdx+1:]...)

	// Re-index pane keys for tabs that shifted down in this workspace.
	newPanes := make(map[PaneKey]*Pane, len(m.panes))
	for k, p := range m.panes {
		if k.Workspace == m.activeWS && k.Tab > idx {
			k.Tab--
		}
		newPanes[k] = p
	}
	m.panes = newPanes

	if m.activeInner >= idx && m.activeInner > 0 {
		m.activeInner--
	}
}

func (m *Model) deleteWorkspaceCmd() tea.Cmd {
	if len(m.worktrees) <= 1 {
		m.setStatus("cannot delete the only workspace")
		return nil
	}
	if m.worktrees[m.activeWS].IsMain {
		m.setStatus("cannot delete the main worktree")
		return nil
	}

	wt := m.worktrees[m.activeWS]

	// Stop and remove all panes for this workspace.
	for key, p := range m.panes {
		if key.Workspace == m.activeWS {
			p.Stop()
			delete(m.panes, key)
		}
	}

	// Remove the worktree from our lists.
	deleted := m.activeWS
	m.worktrees = append(m.worktrees[:deleted], m.worktrees[deleted+1:]...)
	m.extraTabs = append(m.extraTabs[:deleted], m.extraTabs[deleted+1:]...)
	m.closedCfgTabs = append(m.closedCfgTabs[:deleted], m.closedCfgTabs[deleted+1:]...)
	if m.activeWS >= len(m.worktrees) {
		m.activeWS = len(m.worktrees) - 1
	}
	m.activeInner = 0

	// Re-index pane keys for workspaces that shifted down.
	newPanes := make(map[PaneKey]*Pane, len(m.panes))
	for key, p := range m.panes {
		if key.Workspace > deleted {
			key.Workspace--
		}
		newPanes[key] = p
	}
	m.panes = newPanes

	repoRoot := m.repoRoot
	wtPath := wt.Path
	return func() tea.Msg {
		if err := git.Remove(repoRoot, wtPath); err != nil {
			return PaneExitMsg{Err: fmt.Errorf("git worktree remove: %w", err)}
		}
		return nil
	}
}

// restartPane stops the exited pane, removes it from the registry, and
// launches a fresh instance of the same command.
func (m *Model) restartPane(key PaneKey) (tea.Model, tea.Cmd) {
	if p, ok := m.panes[key]; ok {
		p.Stop()
		delete(m.panes, key)
	}
	return m, m.ensurePaneCmd(key)
}

func (m *Model) passthroughPrefix() {
	if b := prefixKeyBytes(m.keybinds.Prefix); b != nil {
		if pane := m.activePane(); pane != nil {
			pane.Write(b)
		}
	}
}

// ============================================================
// Prefix / layer helpers
// ============================================================

func (m *Model) enterPrefixMode() {
	m.prefixMode = true
	m.layerStack = nil
	m.currentLayer = m.keybinds.Bindings
	m.layerTitle = "falcode"
	m.sheet.Open()
}

func (m *Model) exitPrefixMode() {
	m.prefixMode = false
	m.layerStack = nil
	m.currentLayer = m.keybinds.Bindings
	m.layerTitle = "falcode"
	m.sheet.Close()
}

func (m *Model) pushLayer(bindings []*config.Keybind, title string) {
	m.layerStack = append(m.layerStack, layerState{
		bindings: m.currentLayer,
		title:    m.layerTitle,
	})
	m.currentLayer = bindings
	m.layerTitle = title
}

func (m *Model) popLayer() {
	if len(m.layerStack) == 0 {
		return
	}
	top := m.layerStack[len(m.layerStack)-1]
	m.layerStack = m.layerStack[:len(m.layerStack)-1]
	m.currentLayer = top.bindings
	m.layerTitle = top.title
}

// ============================================================
// Pane helpers
// ============================================================

func (m *Model) activePane() *Pane {
	return m.panes[PaneKey{Workspace: m.activeWS, Tab: m.activeInner}]
}

// ensurePaneCmd returns a command that lazily starts a pane if not yet started.
func (m *Model) ensurePaneCmd(key PaneKey) tea.Cmd {
	if _, ok := m.panes[key]; ok {
		return nil // already started
	}
	tab := m.tabForKey(key)
	if tab == nil {
		return nil
	}
	wt := m.worktrees[key.Workspace]
	p := NewPane(key, tab, wt.Path, m.width, m.paneHeight())
	m.panes[key] = p
	send := m.send
	return func() tea.Msg {
		if err := p.Start(send); err != nil {
			return PaneExitMsg{Key: key, Err: err}
		}
		return nil
	}
}

// ensurePaneStarted is a synchronous version used before the event loop starts.
func (m *Model) ensurePaneStarted(key PaneKey) {
	if m.send == nil {
		return
	}
	if _, ok := m.panes[key]; ok {
		return
	}
	tab := m.tabForKey(key)
	if tab == nil {
		return
	}
	wt := m.worktrees[key.Workspace]
	p := NewPane(key, tab, wt.Path, m.width, m.paneHeight())
	m.panes[key] = p
	p.Start(m.send) //nolint:errcheck
}

func (m *Model) tabForKey(key PaneKey) *config.Tab {
	if key.Tab < len(m.cfgTabs) {
		return m.cfgTabs[key.Tab]
	}
	extraIdx := key.Tab - len(m.cfgTabs)
	if key.Workspace >= len(m.extraTabs) || extraIdx >= len(m.extraTabs[key.Workspace]) {
		return nil
	}
	// Dynamic console tab — no Command means interactive shell.
	return &config.Tab{Name: m.extraTabs[key.Workspace][extraIdx]}
}

func (m *Model) paneHeight() int {
	h := m.height - TabBarHeight()
	if !m.cfg.UI.GetHideFooter() {
		h -= FooterHeight()
	}
	if h < 1 {
		h = 1
	}
	return h
}

// ============================================================
// Status helpers
// ============================================================

func (m *Model) renderTabNamePrompt() string {
	st := m.styles
	title := st.SheetTitle.Render("New Tab Name")
	sep := st.SheetSep.Render(strings.Repeat("─", 24))
	input := m.tabNameInput.View()
	content := strings.Join([]string{title, sep, input}, "\n")
	return st.SheetBox.Render(content)
}

func (m *Model) setStatus(msg string) {
	m.statusMsg = msg
	m.statusClearAt = time.Now().Add(3 * time.Second)
}

func (m *Model) currentStatus() string {
	if m.statusMsg != "" && time.Now().After(m.statusClearAt) {
		m.statusMsg = ""
	}
	return m.statusMsg
}

// ============================================================
// Index helpers
// ============================================================

// prevVisibleTab returns the nearest visible tab index strictly before closedIdx
// for the given workspace. It walks backwards through cfgTabs (skipping closed
// ones), then falls back to the last extraTab, and finally returns 0.
func (m *Model) prevVisibleTab(ws, closedIdx int) int {
	closed := m.closedCfgTabs[ws]
	// Search backwards through cfgTabs (indices 0 … len(cfgTabs)-1).
	for i := closedIdx - 1; i >= 0; i-- {
		if i < len(m.cfgTabs) {
			if !closed[i] {
				return i
			}
		} else {
			// It's an extra tab — always visible.
			return i
		}
	}
	// Nothing before closedIdx; fall back to the first visible tab from the front.
	for i := 0; i < len(m.cfgTabs); i++ {
		if !closed[i] {
			return i
		}
	}
	// If somehow all cfgTabs are closed, land on the first extra tab.
	if len(m.extraTabs[ws]) > 0 {
		return len(m.cfgTabs)
	}
	return 0
}

func (m *Model) wrapInner(idx int) int {
	total := len(m.cfgTabs) + len(m.extraTabs[m.activeWS])
	if total == 0 {
		return 0
	}
	return ((idx % total) + total) % total
}

func (m *Model) wrapWS(idx int) int {
	total := len(m.worktrees)
	return ((idx % total) + total) % total
}

// ============================================================
// Misc helpers
// ============================================================

func (m *Model) matchesPrefixKey(msg tea.KeyMsg) bool {
	return keyMsgString(msg) == m.keybinds.Prefix
}

// keyMsgString returns a normalised string representation of a key.
func keyMsgString(msg tea.KeyMsg) string {
	if msg.Type == tea.KeyRunes {
		return string(msg.Runes)
	}
	return msg.String()
}

// prefixKeyBytes encodes a prefix string like "ctrl+b" as raw bytes.
func prefixKeyBytes(prefix string) []byte {
	lower := strings.ToLower(prefix)
	if strings.HasPrefix(lower, "ctrl+") {
		letter := strings.TrimPrefix(lower, "ctrl+")
		if len(letter) == 1 && letter[0] >= 'a' && letter[0] <= 'z' {
			return []byte{letter[0] - 'a' + 1}
		}
	}
	return []byte(prefix)
}

// animTick returns a command that sends an animTickMsg at the animation framerate.
func animTick() tea.Cmd {
	return tea.Tick(time.Second/sheetFPS, func(_ time.Time) tea.Msg {
		return animTickMsg{}
	})
}
