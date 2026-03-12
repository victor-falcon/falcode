package ui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
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

// WorkspaceCreatedMsg is dispatched by the background goroutine when a new
// git worktree has been successfully created.
type WorkspaceCreatedMsg struct{ Worktree *git.Worktree }

// WorkspaceCreateErrMsg is dispatched when worktree creation fails.
type WorkspaceCreateErrMsg struct{ Err error }

// WorkspaceDirtyCheckMsg carries the result of the dirty-state check that
// runs before showing the delete-confirmation dialog.
type WorkspaceDirtyCheckMsg struct {
	WS    int
	Dirty bool
}

// WorkspaceDeleteProgressMsg is dispatched as each deletion step completes.
// Step 1 = git worktree ref removed, Step 2 = folder deleted, Step 3 = tab removed.
type WorkspaceDeleteProgressMsg struct {
	Step int
	Err  error
}

// WorkspaceScriptOutputMsg carries a single line of stdout/stderr from the
// worktree setup script while it is running.
type WorkspaceScriptOutputMsg struct{ Line string }

// WorkspaceScriptDoneMsg is dispatched when the worktree setup script exits.
type WorkspaceScriptDoneMsg struct{ Err error }

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
	renamingTab  bool
	tabNameInput textinput.Model

	// renamedCfgTabs[ws][cfgTabIdx] stores per-workspace name overrides for
	// built-in (config) tabs that have been renamed by the user.
	renamedCfgTabs []map[int]string

	// Workspace creation prompt state.
	// Step 0: ask for workspace name; step 1: ask for branch name.
	namingWS      bool
	wsNamingStep  int
	wsNameInput   textinput.Model
	wsBranchInput textinput.Model
	wsPendingName string // workspace name captured at step 0

	// Workspace deletion confirmation state.
	confirmDeleteWS bool
	wsDeleteDirty   bool
	wsDeleteTarget  int // workspace index captured when delete was initiated

	// Workspace deletion progress state (shown after confirmation).
	deletingWS      bool          // deletion toast is visible
	deleteWSStep    int           // number of completed steps: 0=none, 1=ref removed, 2=folder deleted, 3=tab removed
	deleteWSErr     error         // error from a deletion step, if any
	deleteWSSpinner spinner.Model // animated spinner shown in the toast and tab during deletion

	// Workspace creation loading state: true while git worktree add is running.
	creatingWS      bool
	createWSSpinner spinner.Model // animated spinner shown in the creating-workspace modal and script output modal

	// Worktree setup script execution state.
	runningScript bool
	scriptWSIdx   int // workspace index where the setup script is running
	scriptDone    bool
	scriptOutput  []string // ring-buffer of the last scriptOutputMax lines
	scriptTitle   string   // base name of the script being run
	scriptErr     error

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

	renamedCfgTabs := make([]map[int]string, len(worktrees))
	for i := range renamedCfgTabs {
		renamedCfgTabs[i] = make(map[int]string)
	}

	ti := textinput.New()
	ti.Placeholder = "tab name"
	ti.CharLimit = 32

	wsName := textinput.New()
	wsName.Placeholder = "workspace name"
	wsName.CharLimit = 64

	wsBranch := textinput.New()
	wsBranch.CharLimit = 128

	return &Model{
		cfg:             cfg,
		keybinds:        keybinds,
		styles:          newStyles(theme),
		zm:              zm,
		worktrees:       worktrees,
		repoRoot:        worktrees[0].Path,
		cfgTabs:         cfg.Tabs,
		extraTabs:       extraTabs,
		closedCfgTabs:   closedCfgTabs,
		renamedCfgTabs:  renamedCfgTabs,
		panes:           make(map[PaneKey]*Pane),
		width:           cols,
		height:          rows,
		sheet:           NewSheet(),
		tabNameInput:    ti,
		wsNameInput:     wsName,
		wsBranchInput:   wsBranch,
		currentLayer:    keybinds.Bindings,
		layerTitle:      "falcode",
		version:         version,
		deleteWSSpinner: spinner.New(spinner.WithSpinner(spinner.Hamburger)),
		createWSSpinner: spinner.New(spinner.WithSpinner(spinner.Hamburger)),
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
func (m *Model) Init() tea.Cmd {
	// Enable the Kitty keyboard protocol (level 1 – disambiguate escape codes)
	// after bubbletea has finished its own terminal setup (alt screen, mouse,
	// etc.). Sending it before prog.Run() risks being overwritten by bubbletea's
	// initialisation sequences.
	return func() tea.Msg {
		os.Stdout.WriteString("\x1b[>1u") //nolint:errcheck
		return nil
	}
}

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		for key, p := range m.panes {
			tab := m.tabForKey(key)
			p.Resize(m.paneColsForTab(tab), m.paneHeight())
		}
		return m, nil

	case tea.FocusMsg:
		// The terminal regained focus (e.g. user switched back to this tab).
		// Force a full repaint to flush bubbletea's stale line-diff cache,
		// which otherwise causes duplicated or mispositioned content.
		return m, tea.ClearScreen

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

	case WorkspaceCreatedMsg:
		// Insert the new worktree at the end and switch to it.
		m.creatingWS = false
		m.worktrees = append(m.worktrees, msg.Worktree)
		m.extraTabs = append(m.extraTabs, []string{})
		m.closedCfgTabs = append(m.closedCfgTabs, make(map[int]bool))
		m.renamedCfgTabs = append(m.renamedCfgTabs, make(map[int]string))
		switchCmd := m.switchWorkspaceCmd(len(m.worktrees) - 1)

		// Look for a setup script inside the newly created worktree.
		scriptPath := git.FindWorktreeScript(msg.Worktree.Path, m.cfg.GetWorktreeScripts())
		if scriptPath == "" {
			return m, switchCmd
		}
		m.runningScript = true
		m.scriptWSIdx = len(m.worktrees) - 1
		m.scriptDone = false
		m.scriptOutput = nil
		m.scriptErr = nil
		m.scriptTitle = filepath.Base(scriptPath)
		return m, tea.Batch(switchCmd, m.runWorktreeScriptCmd(msg.Worktree.Path, scriptPath, m.repoRoot))

	case WorkspaceCreateErrMsg:
		m.creatingWS = false
		m.setStatus(fmt.Sprintf("create workspace: %v", msg.Err))
		return m, nil

	case WorkspaceScriptOutputMsg:
		const scriptOutputMax = 20
		m.scriptOutput = append(m.scriptOutput, msg.Line)
		if len(m.scriptOutput) > scriptOutputMax {
			m.scriptOutput = m.scriptOutput[len(m.scriptOutput)-scriptOutputMax:]
		}
		return m, nil

	case WorkspaceScriptDoneMsg:
		m.scriptDone = true
		m.scriptErr = msg.Err
		return m, nil

	case WorkspaceDirtyCheckMsg:
		m.confirmDeleteWS = true
		m.wsDeleteDirty = msg.Dirty
		m.wsDeleteTarget = msg.WS
		return m, nil

	case WorkspaceDeleteProgressMsg:
		if msg.Err != nil {
			m.deleteWSErr = msg.Err
			return m, nil
		}
		m.deleteWSStep = msg.Step
		switch msg.Step {
		case 1:
			return m, m.deleteWSFolderCmd()
		case 2:
			// Brief pause so the toast can show "Removing tab..." before it
			// disappears along with the workspace.
			return m, tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
				return WorkspaceDeleteProgressMsg{Step: 3}
			})
		case 3:
			m.finalizeDeleteWorkspace(m.wsDeleteTarget)
			m.deletingWS = false
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		if m.deletingWS {
			m.deleteWSSpinner, cmd = m.deleteWSSpinner.Update(msg)
			if cmd != nil {
				return m, cmd
			}
		}
		if m.creatingWS || m.runningScript {
			m.createWSSpinner, cmd = m.createWSSpinner.Update(msg)
			if cmd != nil {
				return m, cmd
			}
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

	default:
		// bubbletea wraps unrecognized escape sequences (e.g. Shift+Enter via
		// the Kitty keyboard protocol: ESC [ 13 ; 2 u) as the unexported
		// unknownCSISequenceMsg type, which is a named []byte slice. Decode
		// and route them so that falcode's own keys (Enter, Esc, Tab,
		// Backspace) keep working while modifier variants reach the active PTY.
		return m.handleUnknownMsg(msg)
	}
}

// View implements tea.Model.
func (m *Model) View() string {
	if !m.ready {
		return "Initializing…\n"
	}

	// tabSpinners maps workspace index → spinner char for any tab that should
	// show an animated spinner in place of its × close button.
	tabSpinners := make(map[int]string)
	if m.deletingWS {
		tabSpinners[m.wsDeleteTarget] = m.deleteWSSpinner.View()
	}
	if m.runningScript && !m.scriptDone {
		tabSpinners[m.scriptWSIdx] = m.createWSSpinner.View()
	}

	tabBar := RenderTabBar(
		m.zm,
		m.worktrees,
		m.cfgTabs,
		m.extraTabs[m.activeWS],
		m.closedCfgTabs[m.activeWS],
		m.renamedCfgTabs[m.activeWS],
		m.activeWS,
		m.activeInner,
		m.width,
		m.prefixMode,
		m.currentStatus(),
		m.cfg.UI,
		m.keybinds,
		m.styles,
		tabSpinners,
	)

	paneContent := ""
	pane := m.activePane()
	isConsolePane := pane != nil && pane.IsInteractive()
	if pane != nil {
		paneContent = pane.View()
	}

	// Ensure pane content fills the full pane height.
	paneLines := strings.Split(paneContent, "\n")
	for len(paneLines) < m.paneHeight() {
		paneLines = append(paneLines, strings.Repeat(" ", m.width))
	}

	// Console (interactive) panes get 1-cell horizontal padding on each side.
	if isConsolePane {
		for i, line := range paneLines {
			paneLines[i] = " " + line + " "
		}
	}

	paneContent = strings.Join(paneLines, "\n")

	// Overlay a restart banner centered over the pane area when a
	// non-interactive (command) pane has stopped. This is done before joining
	// with the tab bar so the tab bar is never affected.
	if pane != nil && pane.Exited() && !pane.IsInteractive() {
		banner := m.styles.ExitBanner.Render("process stopped  ·  press Enter to restart")
		paneContent = overlayCentered(paneContent, banner, m.width, m.paneHeight())
	}

	// Overlay the tab name prompt when the user is creating or renaming a tab.
	if m.namingTab {
		prompt := m.renderTabNamePrompt("New Tab Name")
		paneContent = overlayCentered(paneContent, prompt, m.width, m.paneHeight())
	}
	if m.renamingTab {
		prompt := m.renderTabNamePrompt("Rename Tab")
		paneContent = overlayCentered(paneContent, prompt, m.width, m.paneHeight())
	}

	// Overlay the workspace name prompt when the user is creating a new workspace.
	if m.namingWS {
		prompt := m.renderWSNamePrompt()
		paneContent = overlayCentered(paneContent, prompt, m.width, m.paneHeight())
	}

	// Overlay the workspace delete confirmation dialog.
	if m.confirmDeleteWS {
		dialog := m.renderDeleteWSConfirm()
		paneContent = overlayCentered(paneContent, dialog, m.width, m.paneHeight())
	}

	// Overlay a loading indicator while the git worktree is being created.
	if m.creatingWS {
		loading := m.renderWSCreatingModal()
		paneContent = overlayCentered(paneContent, loading, m.width, m.paneHeight())
	}

	// Overlay the script output modal only on the workspace where the script
	// is running; other workspaces remain fully usable in the meantime.
	if m.runningScript && m.activeWS == m.scriptWSIdx {
		scriptModal := m.renderScriptOutputModal()
		paneContent = overlayCentered(paneContent, scriptModal, m.width, m.paneHeight())
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

	// Overlay the deletion toast just below the tab bar at the top-right,
	// applied last so it stays visible regardless of which workspace is active.
	if m.deletingWS {
		toast := m.renderDeleteWSToast()
		view = overlayTopRight(view, toast, m.width, TabBarHeight(m.cfg.UI)+1)
	}

	// Let bubblezone scan the output to record zone positions for mouse hits.
	return m.zm.Scan(view)
}

// ============================================================
// Key handling
// ============================================================

// handleUnknownMsg decodes unrecognized input byte sequences and routes them
// appropriately. bubbletea reports sequences it cannot parse (e.g. keys sent
// via the Kitty keyboard protocol) as the unexported unknownCSISequenceMsg
// type, which is a named []byte slice.
//
// When Kitty keyboard protocol is active the terminal re-encodes several
// formerly ambiguous keys (Enter → ESC[13u, Esc → ESC[27u, Tab → ESC[9u,
// Backspace → ESC[127u). We translate those back into the corresponding
// tea.KeyMsg values so that falcode's own UI (modals, prefix mode) continues
// to work correctly, while modifier variants (e.g. Shift+Enter → ESC[13;2u)
// are forwarded verbatim to the active PTY.
func (m *Model) handleUnknownMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	v := reflect.ValueOf(msg)
	if v.Kind() != reflect.Slice || v.Type().Elem().Kind() != reflect.Uint8 {
		return m, nil
	}
	raw := v.Bytes()
	if len(raw) == 0 {
		return m, nil
	}

	// Try to decode as a Kitty keyboard protocol CSI sequence first.
	if kc, mod, ok := parseKittySeq(raw); ok {
		return m.handleKittyKey(kc, mod, raw)
	}

	// Not a Kitty sequence — forward raw bytes to the active PTY, but never
	// while a modal or prefix mode is consuming input.
	if m.namingWS || m.namingTab || m.renamingTab || m.confirmDeleteWS || m.prefixMode || m.creatingWS || (m.runningScript && m.activeWS == m.scriptWSIdx) {
		return m, nil
	}
	if pane := m.activePane(); pane != nil && !pane.Exited() {
		pane.Write(raw)
	}
	return m, nil
}

// parseKittySeq parses a Kitty keyboard protocol CSI sequence of the form
//
//	ESC [ <keycode> u  or  ESC [ <keycode> ; <modifier> u
//
// and returns the key code (Unicode codepoint), the modifier value (1 = none,
// 2 = shift, 3 = alt, 5 = ctrl, …), and ok=true on success.
func parseKittySeq(raw []byte) (keycode, modifier int, ok bool) {
	if len(raw) < 4 || raw[0] != 0x1b || raw[1] != '[' || raw[len(raw)-1] != 'u' {
		return 0, 0, false
	}
	inner := string(raw[2 : len(raw)-1])
	parts := strings.SplitN(inner, ";", 2)
	kc, err := strconv.Atoi(parts[0])
	if err != nil || kc <= 0 {
		return 0, 0, false
	}
	mod := 1
	if len(parts) == 2 {
		m, err := strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, false
		}
		mod = m
	}
	return kc, mod, true
}

// handleKittyKey routes a decoded Kitty keyboard protocol key event.
//
// The modifier encoding is: modifier = 1 + bitmask where bit0=shift,
// bit1=alt, bit2=ctrl. So modifier==1 means no modifiers.
//
// Keys that falcode itself cares about (Enter, Esc, Tab, Backspace) are
// translated into the equivalent tea.KeyMsg and forwarded to handleKey so
// that modals and prefix mode keep working. Modifier variants of those keys
// (e.g. Shift+Enter) and all other Kitty sequences are written verbatim to
// the active PTY.
func (m *Model) handleKittyKey(keycode, modifier int, raw []byte) (tea.Model, tea.Cmd) {
	modBits := modifier - 1 // 0 = no modifier, bit0=shift, bit1=alt, bit2=ctrl
	shift := modBits&1 != 0

	switch keycode {
	case 13: // Enter
		if modifier <= 1 {
			// Plain Enter — let handleKey deal with modals/panes as normal.
			return m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
		}
		// Shift+Enter — forward as \x1b[13;2u (Kitty CSI u format) to PTY.
		// opencode's key parser handles this format: charCode=13, modifier=2
		// (modifier_bits=1, bit0=shift) → {name:"return", shift:true}.
		// Block while a modal is open.
		if m.namingWS || m.namingTab || m.renamingTab || m.confirmDeleteWS || m.prefixMode || m.creatingWS || (m.runningScript && m.activeWS == m.scriptWSIdx) {
			return m, nil
		}
		if pane := m.activePane(); pane != nil && !pane.Exited() {
			pane.Write([]byte("\x1b[13;2u"))
		}
		return m, nil

	case 27: // Escape
		return m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})

	case 9: // Tab
		if shift {
			return m.handleKey(tea.KeyMsg{Type: tea.KeyShiftTab})
		}
		return m.handleKey(tea.KeyMsg{Type: tea.KeyTab})

	case 127: // Backspace
		if modifier <= 1 {
			// Plain backspace — let handleKey deal with modals/panes as normal.
			return m.handleKey(tea.KeyMsg{Type: tea.KeyBackspace})
		}
		// Modified backspace (e.g. Alt+Backspace) — forward to PTY with the
		// correct byte sequence. Block while a modal is consuming input.
		if m.namingWS || m.namingTab || m.renamingTab || m.confirmDeleteWS || m.prefixMode || m.creatingWS || (m.runningScript && m.activeWS == m.scriptWSIdx) {
			return m, nil
		}
		if pane := m.activePane(); pane != nil && !pane.Exited() {
			// Alt+Backspace → \x1b\x7f (word-delete backward, understood by
			// readline, zsh, bash, and most shells).
			alt := (modifier-1)&2 != 0
			if alt {
				pane.Write([]byte{0x1b, 127})
			} else {
				// Other modifier combos — forward the raw Kitty sequence.
				pane.Write(raw)
			}
		}
		return m, nil

	default:
		// Check if this Kitty-encoded key matches the configured prefix key.
		// When Kitty protocol is active, ctrl+b arrives as ESC[98;5u instead of
		// a tea.KeyMsg, so matchesPrefixKey() never sees it. We decode it here.
		if m.kittyKeycodeMatchesPrefix(keycode, modifier) {
			if !m.prefixMode {
				m.enterPrefixMode()
				return m, animTick()
			}
			return m, nil
		}

		// Unknown key — forward raw to PTY when not in a modal.
		if m.namingWS || m.namingTab || m.renamingTab || m.confirmDeleteWS || m.prefixMode || m.creatingWS || (m.runningScript && m.activeWS == m.scriptWSIdx) {
			return m, nil
		}
		if pane := m.activePane(); pane != nil && !pane.Exited() {
			// Convert Ctrl+letter Kitty sequences to traditional control bytes
			// so the child process receives the correct signal byte (e.g.
			// Ctrl+C → 0x03 / SIGINT) instead of the raw Kitty CSI sequence
			// which most programs do not understand.
			ctrl := (modifier-1)&4 != 0
			if ctrl && ((keycode >= 'a' && keycode <= 'z') || (keycode >= 'A' && keycode <= 'Z')) {
				pane.Write([]byte{byte(keycode & 0x1f)})
			} else {
				pane.Write(raw)
			}
		}
		return m, nil
	}
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If a deletion step failed, Esc dismisses the error toast.
	if m.deletingWS && m.deleteWSErr != nil && msg.Type == tea.KeyEsc {
		m.deletingWS = false
		m.deleteWSErr = nil
		return m, nil
	}

	// While the worktree is being created, swallow all keys.
	if m.creatingWS {
		return m, nil
	}

	// While the setup script is running, block input on the script's workspace.
	// When the script finishes, any key on that workspace dismisses the modal.
	// On other workspaces, let keys flow through normally.
	if m.runningScript {
		if m.scriptDone {
			m.runningScript = false
			if m.activeWS == m.scriptWSIdx {
				// Swallow the key that dismissed the "Done" modal.
				return m, nil
			}
			// On a different workspace: clean up silently, fall through to
			// normal key handling below.
		} else if m.activeWS == m.scriptWSIdx {
			// Script still running on this workspace — block all keys.
			return m, nil
		}
		// Script still running but user is on a different workspace — let keys
		// flow through normally.
	}

	// Workspace naming prompt intercepts all keys (highest priority).
	if m.namingWS {
		return m.handleWSNamingKey(msg)
	}

	// Workspace delete confirmation intercepts all keys.
	if m.confirmDeleteWS {
		return m.handleWSDeleteConfirmKey(msg)
	}

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

	// Tab rename prompt intercepts all keys.
	if m.renamingTab {
		switch msg.Type {
		case tea.KeyEnter:
			name := strings.TrimSpace(m.tabNameInput.Value())
			m.renamingTab = false
			m.tabNameInput.Blur()
			if name != "" {
				m.renameTab(m.activeWS, m.activeInner, name)
			}
			return m, nil
		case tea.KeyEsc:
			m.renamingTab = false
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
		// Non-left-press events: forward to PTY if they land in the pane area,
		// then return — they are never used for falcode's own UI elements.
		m.forwardMouseToPTY(msg)
		return m, nil
	}

	// Workspace (outer) tabs — close button takes priority over tab switch.
	for i := range m.worktrees {
		if zi := m.zm.Get(WorkspaceCloseZoneID(i)); zi != nil && zi.InBounds(msg) {
			// Suppress the close button on tabs that currently show a spinner.
			if m.deletingWS && i == m.wsDeleteTarget {
				return m, nil
			}
			if m.runningScript && !m.scriptDone && i == m.scriptWSIdx {
				return m, nil
			}
			return m, m.deleteWorkspaceCmd(i)
		}
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

	// + new-workspace button.
	if zi := m.zm.Get(NewWorkspaceBtnZoneID()); zi != nil && zi.InBounds(msg) {
		m.startWSNamePrompt()
		return m, nil
	}

	// Left-click landed in the pane area — forward to the active PTY.
	m.forwardMouseToPTY(msg)

	return m, nil
}

// forwardMouseToPTY converts a tea.MouseMsg into an SGR mouse escape sequence
// and writes it to the active PTY. Coordinates are translated from the outer
// terminal's absolute (X, Y) to pane-relative coordinates by subtracting the
// tab bar height from Y. Events that fall within the tab bar rows are ignored.
// SGR format: ESC [ < Cb ; Cx ; Cy M  (press/motion)
//
//	ESC [ < Cb ; Cx ; Cy m  (release)
func (m *Model) forwardMouseToPTY(msg tea.MouseMsg) {
	pane := m.activePane()
	if pane == nil || pane.Exited() {
		return
	}

	// Translate to pane-relative coordinates (1-indexed for SGR).
	tabH := TabBarHeight(m.cfg.UI)
	paneRow := msg.Y - tabH // 0-indexed pane row
	if paneRow < 0 {
		// Click is in the tab bar — do not forward.
		return
	}
	cx := msg.X + 1   // SGR is 1-indexed
	cy := paneRow + 1 // SGR is 1-indexed

	// Encode Cb: button bits + modifier bits.
	// Button base values (SGR): left=0, middle=1, right=2, release=3,
	// wheel-up=64, wheel-down=65, wheel-left=66, wheel-right=67.
	// Motion adds 32. Shift adds 4, Alt adds 8, Ctrl adds 16.
	var cb int
	switch msg.Button {
	case tea.MouseButtonLeft:
		cb = 0
	case tea.MouseButtonMiddle:
		cb = 1
	case tea.MouseButtonRight:
		cb = 2
	case tea.MouseButtonNone:
		cb = 3 // release
	case tea.MouseButtonWheelUp:
		cb = 64
	case tea.MouseButtonWheelDown:
		cb = 65
	case tea.MouseButtonWheelLeft:
		cb = 66
	case tea.MouseButtonWheelRight:
		cb = 67
	default:
		return // unsupported button
	}

	if msg.Action == tea.MouseActionMotion {
		cb |= 32
	}
	if msg.Shift {
		cb |= 4
	}
	if msg.Alt {
		cb |= 8
	}
	if msg.Ctrl {
		cb |= 16
	}

	// Final byte: M = press or motion, m = release.
	final := 'M'
	isWheel := msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown ||
		msg.Button == tea.MouseButtonWheelLeft || msg.Button == tea.MouseButtonWheelRight
	if msg.Action == tea.MouseActionRelease && !isWheel {
		final = 'm'
	}

	seq := fmt.Sprintf("\x1b[<%d;%d;%d%c", cb, cx, cy, final)
	pane.Write([]byte(seq))
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
		case config.ActionNewWorkspace:
			m.startWSNamePrompt()
		case config.ActionDeleteWorkspace:
			cmds = append(cmds, m.deleteWorkspaceCmd(m.activeWS))
		case config.ActionPassthrough:
			m.passthroughPrefix()
		case config.ActionGoToWorkspace:
			if idx, ok := intParam(b.Params, "index"); ok {
				cmds = append(cmds, m.switchWorkspaceCmd(idx))
			}
		case config.ActionGoToTab:
			if idx, ok := intParam(b.Params, "index"); ok && m.isVisibleTab(idx) {
				m.switchInnerTab(idx)
				cmds = append(cmds, m.ensurePaneCmd(PaneKey{Workspace: m.activeWS, Tab: m.activeInner}))
			}
		case config.ActionRenameTab:
			if !m.isCurrentTabRenameable() {
				m.setStatus("cannot rename a tab with a command")
			} else {
				m.tabNameInput.SetValue(m.currentTabName(m.activeWS, m.activeInner))
				m.tabNameInput.Focus()
				m.renamingTab = true
				m.exitPrefixMode()
			}
		}
	}

	return tea.Batch(cmds...)
}

// intParam extracts an integer from a Params map. JSON numbers unmarshal as
// float64, so both int and float64 are handled.
func intParam(params map[string]any, key string) (int, bool) {
	if params == nil {
		return 0, false
	}
	v, ok := params[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case int:
		return n, true
	case float64:
		return int(n), true
	}
	return 0, false
}

func (m *Model) switchWorkspaceCmd(idx int) tea.Cmd {
	if idx < 0 || idx >= len(m.worktrees) {
		return nil
	}
	m.activeWS = idx
	m.activeInner = 0
	return m.ensurePaneCmd(PaneKey{Workspace: m.activeWS, Tab: 0})
}

// deleteWorkspaceCmd runs the same guard checks as ActionDeleteWorkspace and, when
// the workspace is deletable, triggers the dirty-check + confirmation flow.
func (m *Model) deleteWorkspaceCmd(wsIdx int) tea.Cmd {
	if m.deletingWS {
		m.setStatus("already deleting a workspace")
		return nil
	}
	if len(m.worktrees) <= 1 {
		m.setStatus("cannot delete the only workspace")
		return nil
	}
	if m.worktrees[wsIdx].IsMain {
		m.setStatus("cannot delete the main worktree")
		return nil
	}
	return m.checkDirtyAndConfirmDeleteCmd(wsIdx)
}

func (m *Model) switchInnerTab(idx int) {
	m.activeInner = idx
}

// isCurrentTabRenameable reports whether the active tab can be renamed.
// Config tabs with a command (non-interactive) cannot be renamed.
func (m *Model) isCurrentTabRenameable() bool {
	idx := m.activeInner
	if idx < len(m.cfgTabs) {
		return m.cfgTabs[idx].IsInteractive()
	}
	// Extra (dynamically added) tabs are always renameable.
	return true
}

// currentTabName returns the effective display name for the tab at (ws, idx),
// taking per-workspace rename overrides into account.
func (m *Model) currentTabName(ws, idx int) string {
	if idx < len(m.cfgTabs) {
		if ws < len(m.renamedCfgTabs) {
			if name, ok := m.renamedCfgTabs[ws][idx]; ok {
				return name
			}
		}
		return m.cfgTabs[idx].Name
	}
	extraIdx := idx - len(m.cfgTabs)
	if ws < len(m.extraTabs) && extraIdx < len(m.extraTabs[ws]) {
		return m.extraTabs[ws][extraIdx]
	}
	return ""
}

// renameTab updates the display name for the tab at (ws, idx).
func (m *Model) renameTab(ws, idx int, name string) {
	if idx < len(m.cfgTabs) {
		if ws < len(m.renamedCfgTabs) {
			m.renamedCfgTabs[ws][idx] = name
		}
		return
	}
	extraIdx := idx - len(m.cfgTabs)
	if ws < len(m.extraTabs) && extraIdx < len(m.extraTabs[ws]) {
		m.extraTabs[ws][extraIdx] = name
	}
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

// beginDeleteWorkspaceCmd stops all panes for the given workspace and fires
// the first async deletion step (removing the git worktree ref). The tab
// remains visible and the progress modal is shown while work proceeds.
func (m *Model) beginDeleteWorkspaceCmd(wsIdx int) tea.Cmd {
	wt := m.worktrees[wsIdx]

	// Signal all panes for this workspace to stop. We do NOT wait for the
	// child processes to fully exit — git worktree remove --force and
	// os.RemoveAll are safe to run while processes still hold open file handles
	// on macOS/Linux, and waiting could block indefinitely if a process ignores
	// the PTY close signal.
	for key, p := range m.panes {
		if key.Workspace == wsIdx {
			p.Stop()
		}
	}

	repoRoot := m.repoRoot
	wtPath := wt.Path
	branch := wt.Branch
	return func() tea.Msg {
		err := git.RemoveRef(repoRoot, wtPath, branch)
		return WorkspaceDeleteProgressMsg{Step: 1, Err: err}
	}
}

// deleteWSFolderCmd fires the second async deletion step: wiping the worktree
// directory (and its now-empty parent) from disk.
func (m *Model) deleteWSFolderCmd() tea.Cmd {
	wtPath := m.worktrees[m.wsDeleteTarget].Path
	return func() tea.Msg {
		git.RemoveFolder(wtPath)
		return WorkspaceDeleteProgressMsg{Step: 2}
	}
}

// finalizeDeleteWorkspace removes the workspace at wsIdx from all model slices,
// re-indexes pane keys for workspaces that shifted down, and adjusts the active
// indices. Must be called from the Update loop (i.e. synchronously).
func (m *Model) finalizeDeleteWorkspace(wsIdx int) {
	// Clean up any remaining stopped pane entries for the deleted workspace.
	for key := range m.panes {
		if key.Workspace == wsIdx {
			delete(m.panes, key)
		}
	}

	// Remove the workspace from all parallel slices.
	m.worktrees = append(m.worktrees[:wsIdx], m.worktrees[wsIdx+1:]...)
	m.extraTabs = append(m.extraTabs[:wsIdx], m.extraTabs[wsIdx+1:]...)
	m.closedCfgTabs = append(m.closedCfgTabs[:wsIdx], m.closedCfgTabs[wsIdx+1:]...)
	m.renamedCfgTabs = append(m.renamedCfgTabs[:wsIdx], m.renamedCfgTabs[wsIdx+1:]...)
	if m.activeWS >= len(m.worktrees) {
		m.activeWS = len(m.worktrees) - 1
	}
	m.activeInner = 0

	// Re-index pane keys for workspaces that shifted down.
	newPanes := make(map[PaneKey]*Pane, len(m.panes))
	for key, p := range m.panes {
		if key.Workspace > wsIdx {
			key.Workspace--
		}
		newPanes[key] = p
	}
	m.panes = newPanes
}

// restartPane stops the exited pane, removes it from the registry, and
// launches a fresh instance of the same command.
// auto_run is intentionally ignored here — restarting is always an explicit
// user action (pressing Enter), so the command must always start.
func (m *Model) restartPane(key PaneKey) (tea.Model, tea.Cmd) {
	if p, ok := m.panes[key]; ok {
		p.Stop()
		delete(m.panes, key)
	}
	tab := m.tabForKey(key)
	if tab == nil {
		return m, nil
	}
	wt := m.worktrees[key.Workspace]
	p := NewPane(key, tab, wt.Path, m.paneColsForTab(tab), m.paneHeight())
	m.panes[key] = p
	send := m.send
	return m, func() tea.Msg {
		if err := p.Start(send); err != nil {
			return PaneExitMsg{Key: key, Err: err}
		}
		return nil
	}
}

func (m *Model) passthroughPrefix() {
	if b := prefixKeyBytes(m.keybinds.Prefix); b != nil {
		if pane := m.activePane(); pane != nil {
			pane.Write(b)
		}
	}
}

// ============================================================
// Workspace creation
// ============================================================

// startWSNamePrompt begins the two-step workspace naming flow.
func (m *Model) startWSNamePrompt() {
	m.namingWS = true
	m.wsNamingStep = 0
	m.wsPendingName = ""
	m.wsBranchInput.SetValue("")
	m.wsBranchInput.Focus()
	m.wsNameInput.SetValue("")
	m.wsNameInput.Blur()
}

// handleWSNamingKey processes keys while the workspace naming prompt is active.
func (m *Model) handleWSNamingKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.namingWS = false
		m.wsNameInput.Blur()
		m.wsBranchInput.Blur()
		return m, nil

	case tea.KeyEnter:
		if m.wsNamingStep == 0 {
			// Step 0 → capture branch name (mandatory), advance to worktree name step.
			branch := strings.TrimSpace(m.wsBranchInput.Value())
			if branch == "" {
				// No branch entered; stay on step 0.
				return m, nil
			}
			m.wsPendingName = branch
			m.wsNamingStep = 1
			m.wsBranchInput.Blur()
			m.wsNameInput.SetValue("")
			m.wsNameInput.Placeholder = branch // hint: press Enter to reuse branch name
			m.wsNameInput.Focus()
			return m, nil
		}

		// Step 1 → worktree name (empty = reuse branch name).
		wsName := strings.TrimSpace(m.wsNameInput.Value())
		if wsName == "" {
			wsName = m.wsPendingName
		}
		branchName := m.wsPendingName
		m.namingWS = false
		m.creatingWS = true
		m.wsNameInput.Blur()
		m.wsBranchInput.Blur()
		m.createWSSpinner = spinner.New(spinner.WithSpinner(spinner.Hamburger))
		return m, tea.Batch(m.createWorkspaceCmd(wsName, branchName), m.createWSSpinner.Tick)
	}

	// Forward keystrokes to the active input.
	var cmd tea.Cmd
	if m.wsNamingStep == 0 {
		m.wsBranchInput, cmd = m.wsBranchInput.Update(msg)
	} else {
		m.wsNameInput, cmd = m.wsNameInput.Update(msg)
	}
	return m, cmd
}

// createWorkspaceCmd runs git worktree creation in the background and
// dispatches WorkspaceCreatedMsg or WorkspaceCreateErrMsg when done.
func (m *Model) createWorkspaceCmd(worktreeName, branchName string) tea.Cmd {
	repoRoot := m.repoRoot
	return func() tea.Msg {
		wt, err := git.Create(repoRoot, worktreeName, branchName)
		if err != nil {
			return WorkspaceCreateErrMsg{Err: err}
		}
		return WorkspaceCreatedMsg{Worktree: wt}
	}
}

// runWorktreeScriptCmd executes scriptPath (found inside worktreePath) as a
// shell script with repoRoot passed as $1. stdout and stderr are merged and
// streamed line-by-line via WorkspaceScriptOutputMsg. When the process exits,
// WorkspaceScriptDoneMsg is dispatched.
func (m *Model) runWorktreeScriptCmd(worktreePath, scriptPath, repoRoot string) tea.Cmd {
	send := m.send
	return func() tea.Msg {
		sh := os.Getenv("SHELL")
		if sh == "" {
			sh = "/bin/sh"
		}
		cmd := exec.Command(sh, scriptPath, repoRoot)
		cmd.Dir = worktreePath

		// Use an io.Pipe to merge stdout and stderr into a single stream.
		pr, pw := io.Pipe()
		cmd.Stdout = pw
		cmd.Stderr = pw

		if err := cmd.Start(); err != nil {
			pw.Close()
			pr.Close()
			return WorkspaceScriptDoneMsg{Err: fmt.Errorf("start script: %w", err)}
		}

		// Wait for the process in a goroutine so we can stream output
		// concurrently. Close the pipe writer when done so the scanner sees EOF.
		var waitErr error
		waitDone := make(chan struct{})
		go func() {
			waitErr = cmd.Wait()
			pw.Close()
			close(waitDone)
		}()

		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			send(WorkspaceScriptOutputMsg{Line: scanner.Text()})
		}
		pr.Close()
		<-waitDone

		return WorkspaceScriptDoneMsg{Err: waitErr}
	}
}

// ============================================================
// Workspace deletion confirmation
// ============================================================

// checkDirtyAndConfirmDeleteCmd runs git status for the target workspace in
// the background, then dispatches WorkspaceDirtyCheckMsg so the UI can show
// the confirmation dialog.
func (m *Model) checkDirtyAndConfirmDeleteCmd(wsIdx int) tea.Cmd {
	wtPath := m.worktrees[wsIdx].Path
	return func() tea.Msg {
		dirty := git.HasUncommittedChanges(wtPath)
		return WorkspaceDirtyCheckMsg{WS: wsIdx, Dirty: dirty}
	}
}

// handleWSDeleteConfirmKey processes y/n while the delete confirmation dialog
// is shown.
func (m *Model) handleWSDeleteConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.confirmDeleteWS = false
		return m, nil
	}

	switch strings.ToLower(string(msg.Runes)) {
	case "y":
		wsIdx := m.wsDeleteTarget
		m.confirmDeleteWS = false
		m.deletingWS = true
		m.deleteWSStep = 0
		m.deleteWSErr = nil
		m.deleteWSSpinner = spinner.New(spinner.WithSpinner(spinner.Hamburger))
		return m, tea.Batch(m.beginDeleteWorkspaceCmd(wsIdx), m.deleteWSSpinner.Tick)
	case "n":
		m.confirmDeleteWS = false
		return m, nil
	}

	return m, nil
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
	p := NewPane(key, tab, wt.Path, m.paneColsForTab(tab), m.paneHeight())
	m.panes[key] = p
	// If auto_run is disabled, mark the pane as stopped immediately so the
	// "press Enter to restart" banner is shown without running the command.
	if !tab.IsInteractive() && !tab.ShouldAutoRun() {
		p.MarkStopped()
		return nil
	}
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
	p := NewPane(key, tab, wt.Path, m.paneColsForTab(tab), m.paneHeight())
	m.panes[key] = p
	// If auto_run is disabled, mark the pane as stopped so the banner is shown.
	if !tab.IsInteractive() && !tab.ShouldAutoRun() {
		p.MarkStopped()
		return
	}
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

// isVisibleTab reports whether logical tab index idx exists and is currently
// visible for the active workspace. It returns false for out-of-range indices
// and for built-in tabs that have been closed in this workspace.
func (m *Model) isVisibleTab(idx int) bool {
	if idx < 0 {
		return false
	}
	if idx < len(m.cfgTabs) {
		return !m.closedCfgTabs[m.activeWS][idx]
	}
	extraIdx := idx - len(m.cfgTabs)
	if m.activeWS >= len(m.extraTabs) {
		return false
	}
	return extraIdx < len(m.extraTabs[m.activeWS])
}

func (m *Model) paneHeight() int {
	h := m.height - TabBarHeight(m.cfg.UI)
	if !m.cfg.UI.GetHideFooter() {
		h -= FooterHeight()
	}
	if h < 1 {
		h = 1
	}
	return h
}

// paneColsForTab returns the PTY column count for a given tab.
// Console (interactive) tabs lose 2 columns for the 1-cell paddingX on each side.
func (m *Model) paneColsForTab(tab *config.Tab) int {
	if tab != nil && tab.IsInteractive() {
		if m.width > 2 {
			return m.width - 2
		}
		return 1
	}
	return m.width
}

// ============================================================
// Status helpers
// ============================================================

func (m *Model) renderTabNamePrompt(title string) string {
	st := m.styles
	titleStr := st.SheetTitle.Render(title)
	sep := st.SheetSep.Render(strings.Repeat("─", 24))
	input := m.tabNameInput.View()
	content := strings.Join([]string{titleStr, sep, input}, "\n")
	return st.SheetBox.Render(content)
}

func (m *Model) renderWSNamePrompt() string {
	st := m.styles
	var titleStr string
	var inputView string
	if m.wsNamingStep == 0 {
		titleStr = "New Workspace — Branch"
		inputView = m.wsBranchInput.View()
	} else {
		titleStr = "New Workspace — Name"
		inputView = m.wsNameInput.View()
	}
	title := st.SheetTitle.Render(titleStr)
	sep := st.SheetSep.Render(strings.Repeat("─", 28))
	content := strings.Join([]string{title, sep, inputView}, "\n")
	return st.SheetBox.Render(content)
}

func (m *Model) renderDeleteWSConfirm() string {
	st := m.styles
	wsIdx := m.wsDeleteTarget
	var wsName string
	if wsIdx >= 0 && wsIdx < len(m.worktrees) {
		wsName = m.worktrees[wsIdx].Name()
	}

	title := st.SheetTitle.Render(fmt.Sprintf("Delete '%s'?", wsName))
	sep := st.SheetSep.Render(strings.Repeat("─", 28))

	var lines []string
	lines = append(lines, title, sep)

	if m.wsDeleteDirty {
		lines = append(lines,
			st.WarningMsg.Render("Uncommitted changes will be lost!"),
			st.SheetDesc.Render(""),
		)
	}

	lines = append(lines, st.SheetDesc.Render("[y] confirm   [n] / Esc cancel"))

	content := strings.Join(lines, "\n")
	return st.SheetBox.Render(content)
}

// renderWSCreatingModal renders the loading overlay shown while the git
// worktree is being created in the background.
func (m *Model) renderWSCreatingModal() string {
	st := m.styles
	title := st.SheetTitle.Render("Creating Workspace")
	sep := st.SheetSep.Render(strings.Repeat("─", 28))
	body := st.SheetDesc.Render(m.createWSSpinner.View() + "Setting up git worktree…")
	content := strings.Join([]string{title, sep, body}, "\n")
	return st.SheetBox.Render(content)
}

// renderDeleteWSToast renders the compact top-right toast shown while a
// workspace is being deleted. It displays the workspace name and the current
// in-progress step with an animated braille spinner. On error it shows the
// failure and prompts the user to press Esc to dismiss.
func (m *Model) renderDeleteWSToast() string {
	st := m.styles

	wsName := ""
	if wsIdx := m.wsDeleteTarget; wsIdx >= 0 && wsIdx < len(m.worktrees) {
		wsName = m.worktrees[wsIdx].Name()
	}

	// Step labels shown as the current in-progress action.
	stepLabels := []string{
		"Removing git worktree...",
		"Deleting folder...",
		"Removing tab...",
	}

	title := st.SheetTitle.Render(fmt.Sprintf("Deleting '%s' workspace.", wsName))

	var stepLine string
	if m.deleteWSErr != nil {
		// Error state: show which step failed and the error.
		failedLabel := ""
		if m.deleteWSStep >= 0 && m.deleteWSStep < len(stepLabels) {
			failedLabel = stepLabels[m.deleteWSStep]
		}
		stepLine = st.WarningMsg.Render(fmt.Sprintf("✗ %s", failedLabel)) + "\n" +
			st.WarningMsg.Render(fmt.Sprintf("  %v", m.deleteWSErr)) + "\n" +
			st.SheetDesc.Render("Esc to dismiss")
	} else {
		spinner := m.deleteWSSpinner.View()
		label := ""
		if m.deleteWSStep < len(stepLabels) {
			label = stepLabels[m.deleteWSStep]
		}
		stepLine = st.SheetDesc.Render(fmt.Sprintf("%s %s", spinner, label))
	}

	content := strings.Join([]string{title, stepLine}, "\n")
	return st.SheetBox.Render(content)
}

// renderScriptOutputModal renders the setup script output overlay. It shows
// the last scriptOutputMax lines streamed from the script's stdout/stderr,
// plus a status line indicating whether the script is still running or has
// finished. When done the user dismisses it with any key.
func (m *Model) renderScriptOutputModal() string {
	// Fill most of the terminal width; leave a small margin for the box border.
	innerWidth := m.width - 10
	if innerWidth < 60 {
		innerWidth = 60
	}
	if innerWidth > 160 {
		innerWidth = 160
	}

	st := m.styles
	title := st.SheetTitle.Render(fmt.Sprintf("Running %s", m.scriptTitle))
	sep := st.SheetSep.Render(strings.Repeat("─", innerWidth))

	var lines []string
	lines = append(lines, title, sep)

	for _, line := range m.scriptOutput {
		// Truncate lines that exceed the inner width.
		r := []rune(line)
		if len(r) > innerWidth {
			line = string(r[:innerWidth-1]) + "…"
		}
		lines = append(lines, st.SheetDesc.Render(line))
	}

	// Bottom separator + status line.
	lines = append(lines, st.SheetSep.Render(strings.Repeat("─", innerWidth)))
	var statusLine string
	switch {
	case !m.scriptDone:
		statusLine = st.SheetDesc.Render(m.createWSSpinner.View() + "running…")
	case m.scriptErr != nil:
		errText := st.WarningMsg.Render(fmt.Sprintf("Error: %v", m.scriptErr))
		statusLine = errText + st.SheetDesc.Render("  ·  press any key")
	default:
		statusLine = st.SheetDesc.Render("Done  ·  press any key to dismiss")
	}
	lines = append(lines, statusLine)

	return st.SheetBox.Render(strings.Join(lines, "\n"))
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

// kittyKeycodeMatchesPrefix reports whether the Kitty-decoded keycode and
// modifier represent the same key as the configured prefix string.
//
// When the Kitty keyboard protocol is active, Ctrl+letter keys arrive as
// ESC[<codepoint>;<modifier>u instead of being routed through tea.KeyMsg, so
// matchesPrefixKey never sees them. This function bridges that gap by
// decoding the Kitty modifier bitmask and comparing against the prefix.
//
// Currently supports "ctrl+<letter>" prefixes (the only default and most
// common user choice). The Kitty modifier encoding: modifier = 1 + bitmask
// where bit2 = ctrl, bit1 = alt, bit0 = shift.
func (m *Model) kittyKeycodeMatchesPrefix(keycode, modifier int) bool {
	prefix := strings.ToLower(m.keybinds.Prefix)
	if !strings.HasPrefix(prefix, "ctrl+") {
		return false
	}
	letter := strings.TrimPrefix(prefix, "ctrl+")
	if len(letter) != 1 {
		return false
	}
	// Kitty modifier: bit2 (value 4) = ctrl; no other modifier bits set.
	modBits := modifier - 1
	ctrlOnly := modBits == 4
	// keycode must match the letter's codepoint (case-insensitive).
	expectedCode := int(letter[0])
	return ctrlOnly && (keycode == expectedCode || keycode == expectedCode-32)
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
