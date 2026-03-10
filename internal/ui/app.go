package ui

import (
	"fmt"
	"strings"
	"time"

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
	cfgTabs   []*config.Tab
	extraTabs [][]string

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

	return &Model{
		cfg:          cfg,
		keybinds:     keybinds,
		styles:       newStyles(theme),
		zm:           zm,
		worktrees:    worktrees,
		repoRoot:     worktrees[0].Path,
		cfgTabs:      cfg.Tabs,
		extraTabs:    extraTabs,
		panes:        make(map[PaneKey]*Pane),
		width:        cols,
		height:       rows,
		sheet:        NewSheet(),
		currentLayer: keybinds.Bindings,
		layerTitle:   "falcode",
		version:      version,
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
		m.activeWS,
		m.activeInner,
		m.width,
		m.prefixMode,
		m.currentStatus(),
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

	view := tabBar + "\n" + paneContent

	// Footer: context hint (left) and build version (right).
	footer := RenderFooter(m.keybinds.Prefix, m.version, m.prefixMode, m.width, m.styles)
	view = view + "\n" + footer

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
	if msg.Type == tea.KeyEsc {
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
		if zi := m.zm.Get(InnerTabZoneID(i)); zi != nil && zi.InBounds(msg) {
			m.switchInnerTab(i)
			return m, m.ensurePaneCmd(PaneKey{Workspace: m.activeWS, Tab: i})
		}
	}

	return m, nil
}

// ============================================================
// Actions
// ============================================================

func (m *Model) executeAction(b *config.Keybind) tea.Cmd {
	m.exitPrefixMode()
	switch b.Action {
	case config.ActionQuit:
		return tea.Quit
	case config.ActionNextTab:
		m.switchInnerTab(m.wrapInner(m.activeInner + 1))
		return m.ensurePaneCmd(PaneKey{Workspace: m.activeWS, Tab: m.activeInner})
	case config.ActionPrevTab:
		m.switchInnerTab(m.wrapInner(m.activeInner - 1))
		return m.ensurePaneCmd(PaneKey{Workspace: m.activeWS, Tab: m.activeInner})
	case config.ActionNewTab:
		m.addExtraTab()
		return m.ensurePaneCmd(PaneKey{Workspace: m.activeWS, Tab: m.activeInner})
	case config.ActionCloseTab:
		m.closeCurrentTab()
	case config.ActionNextWorkspace:
		return m.switchWorkspaceCmd((m.activeWS + 1) % len(m.worktrees))
	case config.ActionPrevWorkspace:
		return m.switchWorkspaceCmd(m.wrapWS(m.activeWS - 1))
	case config.ActionDeleteWorkspace:
		return m.deleteWorkspaceCmd()
	case config.ActionPassthrough:
		m.passthroughPrefix()
	}
	return nil
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

func (m *Model) addExtraTab() {
	n := len(m.extraTabs[m.activeWS]) + 1
	label := fmt.Sprintf("console-%d", n)
	m.extraTabs[m.activeWS] = append(m.extraTabs[m.activeWS], label)
	m.activeInner = len(m.cfgTabs) + len(m.extraTabs[m.activeWS]) - 1
}

func (m *Model) closeCurrentTab() {
	if m.activeInner < len(m.cfgTabs) {
		m.setStatus("cannot close a built-in tab")
		return
	}
	extraIdx := m.activeInner - len(m.cfgTabs)
	key := PaneKey{Workspace: m.activeWS, Tab: m.activeInner}
	if p, ok := m.panes[key]; ok {
		p.Stop()
		delete(m.panes, key)
	}
	extra := m.extraTabs[m.activeWS]
	m.extraTabs[m.activeWS] = append(extra[:extraIdx], extra[extraIdx+1:]...)
	if m.activeInner > 0 {
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
	h := m.height - TabBarHeight() - FooterHeight()
	if h < 1 {
		h = 1
	}
	return h
}

// ============================================================
// Status helpers
// ============================================================

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
