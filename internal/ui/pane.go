package ui

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbletea"
	"github.com/creack/pty"
	"github.com/hinshun/vt10x"
	"github.com/victor-falcon/falcode/internal/config"
)

// maxScrollback is the maximum number of rows kept in the scrollback buffer.
const maxScrollback = 1000

// PaneKey uniquely identifies a pane as (workspaceIndex, tabIndex).
type PaneKey struct {
	Workspace int
	Tab       int
}

// PaneOutputMsg is sent whenever the PTY produces output.
type PaneOutputMsg struct{ Key PaneKey }

// PaneExitMsg is sent when the child process exits.
type PaneExitMsg struct {
	Key PaneKey
	Err error
}

type selectionCell struct {
	Row int
	Col int
}

type SelectionRange struct {
	Start selectionCell
	End   selectionCell
}

func (r SelectionRange) Contains(row, col int) bool {
	cell := selectionCell{Row: row, Col: col}
	return compareSelectionCell(r.Start, cell) <= 0 && compareSelectionCell(cell, r.End) <= 0
}

func compareSelectionCell(a, b selectionCell) int {
	if a.Row < b.Row {
		return -1
	}
	if a.Row > b.Row {
		return 1
	}
	if a.Col < b.Col {
		return -1
	}
	if a.Col > b.Col {
		return 1
	}
	return 0
}

// Pane manages a single PTY process and its VT100 terminal emulator.
type Pane struct {
	cfg            *config.Tab
	key            PaneKey
	dir            string // working directory for the process
	statusPipePath string // FIFO path for agent status events (empty = none)

	mu      sync.Mutex
	ptmx    *os.File
	cmd     interface{ Wait() error }
	vt      vt10x.Terminal
	cols    int
	rows    int
	started bool
	exited  bool
	done    chan struct{}
	exitErr error

	// scrollback holds rows that have scrolled off the top of the VT screen.
	// Only populated while the inner terminal is NOT in alt-screen mode.
	scrollback   [][]vt10x.Glyph
	scrollOffset int           // rows scrolled above the live view (0 = live)
	pendingRow0  []vt10x.Glyph // snapshot of row 0 taken before each vt.Write

	// notify is a size-1 channel used to coalesce PTY output notifications.
	// The PTY read goroutine signals it non-blocking; a dedicated sender
	// goroutine drains it and calls send(PaneOutputMsg{}) so that the read
	// loop never blocks on bubbletea's message channel.
	notify chan struct{}

	selStart  *selectionCell
	selEnd    *selectionCell
	selecting bool
}

// NewPane creates a Pane but does not start it yet.
func NewPane(key PaneKey, cfg *config.Tab, dir string, cols, rows int) *Pane {
	return &Pane{
		cfg:    cfg,
		key:    key,
		dir:    dir,
		cols:   cols,
		rows:   rows,
		done:   make(chan struct{}),
		notify: make(chan struct{}, 1),
	}
}

// SetStatusPipe sets the FIFO path for agent status events. Must be called
// before Start().
func (p *Pane) SetStatusPipe(path string) {
	p.statusPipePath = path
}

// Start launches the PTY process and begins streaming its output.
// send is the bubbletea send function for dispatching messages.
func (p *Pane) Start(send func(tea.Msg)) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.started {
		return nil
	}

	p.vt = vt10x.New(vt10x.WithSize(p.cols, p.rows))

	// Determine the shell binary for wrapping commands.
	sh := os.Getenv("SHELL")
	if sh == "" {
		sh = "/bin/sh"
	}

	var cmd *ptyCmd
	if p.cfg.IsInteractive() {
		// Interactive shell — run $SHELL with no arguments.
		cmd = newPtyCmd(sh, nil, p.dir)
	} else {
		// Configured command — run through the shell so that PATH resolution
		// and simple shell syntax work (e.g. "opencode", "lazygit").
		cmd = newPtyCmd(sh, []string{"-c", p.cfg.Command}, p.dir)
	}

	// Inject the agent status pipe path so that tools like the OpenCode
	// falcode plugin can report their status back, regardless of whether this
	// is an interactive or command pane.
	if p.statusPipePath != "" {
		cmd.Cmd.Env = append(cmd.Cmd.Env, "FALCODE_STATUS_PIPE="+p.statusPipePath)
	}

	ptmx, err := pty.StartWithSize(cmd.Cmd, &pty.Winsize{
		Rows: uint16(p.rows),
		Cols: uint16(p.cols),
	})
	if err != nil {
		return fmt.Errorf("pty start: %w", err)
	}

	p.ptmx = ptmx
	p.cmd = cmd.Cmd
	p.started = true

	notify := p.notify

	// Sender goroutine: drains the notify channel and calls send() so that the
	// PTY read loop below is never blocked waiting on bubbletea's message queue.
	go func() {
		for range notify {
			send(PaneOutputMsg{Key: p.key})
		}
	}()

	// If a status pipe was configured, start a reader goroutine that will
	// relay agent status events into the bubbletea event loop.
	if p.statusPipePath != "" {
		go readAgentPipe(p.statusPipePath, p.key, send)
	}

	// PTY read goroutine: reads raw bytes, feeds them into the VT emulator, then
	// signals the sender goroutine via a non-blocking send into the size-1
	// notify channel. If a notification is already pending the select falls
	// through immediately — the vt10x state has already been updated, so the
	// next send will render the latest state. This ensures the goroutine never
	// blocks on bubbletea's message channel, which prevents the PTY kernel
	// buffer from filling and stalling lazygit's timer-triggered refreshes.
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				p.mu.Lock()
				p.captureBeforeWrite()
				p.vt.Write(buf[:n])
				p.captureAfterWrite()
				p.mu.Unlock()
				select {
				case notify <- struct{}{}:
				default: // notification already pending; vt state is up-to-date
				}
			}
			if err != nil {
				break
			}
		}
		exitErr := cmd.Cmd.Wait()
		p.mu.Lock()
		p.exitErr = exitErr
		p.exited = true
		p.mu.Unlock()
		close(notify) // unblocks the sender goroutine
		close(p.done)
		send(PaneExitMsg{Key: p.key, Err: exitErr})
	}()

	return nil
}

// MarkStopped puts the pane into a stopped state without running the process.
// Used when auto_run is false — the pane appears as if it ran and exited,
// so the "press Enter to restart" banner is shown immediately.
func (p *Pane) MarkStopped() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.started = true
	p.exited = true
	close(p.done)
}

// Write forwards raw bytes to the PTY's stdin.
func (p *Pane) Write(data []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ptmx != nil {
		p.ptmx.Write(data) //nolint:errcheck
	}
}

// Resize updates the PTY and VT emulator dimensions.
func (p *Pane) Resize(cols, rows int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cols = cols
	p.rows = rows
	if p.vt != nil {
		p.vt.Resize(cols, rows)
	}
	if p.ptmx != nil {
		pty.Setsize(p.ptmx, &pty.Winsize{ //nolint:errcheck
			Rows: uint16(rows),
			Cols: uint16(cols),
		})
	}
}

// Stop terminates the PTY process and removes the agent status pipe if any.
func (p *Pane) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ptmx != nil {
		p.ptmx.Close()
		p.ptmx = nil
	}
	if p.statusPipePath != "" {
		removeAgentPipe(p.statusPipePath)
		p.statusPipePath = ""
	}
}

// View renders the current VT screen state as an ANSI string.
// When the user has scrolled up, it renders from the scrollback buffer
// blended with the live VT screen.
func (p *Pane) View() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.vt == nil {
		return ""
	}

	sel := p.selectionRangeLocked()
	if p.scrollOffset > 0 {
		return renderVTWithScrollback(p.scrollback, p.vt, p.scrollOffset, p.cols, p.rows, sel)
	}
	return renderVT(p.vt, p.cols, p.rows, sel)
}

func (p *Pane) StartSelection(row, col int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	cell, ok := p.selectionCellFromDisplayLocked(row, col)
	if !ok {
		p.clearSelectionLocked()
		return
	}
	p.selStart = &cell
	p.selEnd = &cell
	p.selecting = true
}

func (p *Pane) UpdateSelection(row, col int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.selecting || p.selStart == nil {
		return
	}
	cell, ok := p.selectionCellFromDisplayLocked(row, col)
	if !ok {
		return
	}
	p.selEnd = &cell
}

func (p *Pane) EndSelection(row, col int) string {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.selecting || p.selStart == nil {
		return ""
	}
	if cell, ok := p.selectionCellFromDisplayLocked(row, col); ok {
		p.selEnd = &cell
	}
	p.selecting = false
	return p.selectedTextLocked()
}

func (p *Pane) ClearSelection() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.clearSelectionLocked()
}

func (p *Pane) Selecting() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.selecting
}

func (p *Pane) selectionRangeLocked() *SelectionRange {
	if p.selStart == nil || p.selEnd == nil {
		return nil
	}
	start := *p.selStart
	end := *p.selEnd
	if compareSelectionCell(start, end) > 0 {
		start, end = end, start
	}
	sel := SelectionRange{Start: start, End: end}
	return &sel
}

func (p *Pane) selectedTextLocked() string {
	sel := p.selectionRangeLocked()
	if sel == nil {
		return ""
	}

	lines := make([]string, 0, sel.End.Row-sel.Start.Row+1)
	for row := sel.Start.Row; row <= sel.End.Row; row++ {
		startCol := 0
		endCol := p.cols - 1
		if row == sel.Start.Row {
			startCol = sel.Start.Col
		}
		if row == sel.End.Row {
			endCol = sel.End.Col
		}
		if startCol > endCol {
			startCol, endCol = endCol, startCol
		}

		var sb strings.Builder
		for col := startCol; col <= endCol; col++ {
			cell, ok := p.virtualCellLocked(row, col)
			if !ok {
				continue
			}
			ch := cell.Char
			if ch == 0 {
				ch = ' '
			}
			sb.WriteRune(ch)
		}
		lines = append(lines, strings.TrimRight(sb.String(), " "))
	}

	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

func (p *Pane) virtualCellLocked(row, col int) (vt10x.Glyph, bool) {
	if p.vt == nil || col < 0 || col >= p.cols || row < 0 {
		return vt10x.Glyph{}, false
	}
	sbLen := len(p.scrollback)
	if row < sbLen {
		sbRow := p.scrollback[row]
		if col < len(sbRow) {
			return sbRow[col], true
		}
		return vt10x.Glyph{}, true
	}

	vtRow := row - sbLen
	if vtRow < 0 || vtRow >= p.rows {
		return vt10x.Glyph{}, false
	}
	return p.vt.Cell(col, vtRow), true
}

func (p *Pane) selectionCellFromDisplayLocked(row, col int) (selectionCell, bool) {
	if p.cols <= 0 || p.rows <= 0 {
		return selectionCell{}, false
	}
	maxRow := p.rows - 1
	if p.scrollOffset > 0 && maxRow > 0 {
		maxRow--
	}
	if maxRow < 0 {
		return selectionCell{}, false
	}
	if row < 0 {
		row = 0
	}
	if row > maxRow {
		row = maxRow
	}
	if col < 0 {
		col = 0
	}
	if col >= p.cols {
		col = p.cols - 1
	}
	return selectionCell{Row: p.displayRowToVirtualLocked(row), Col: col}, true
}

func (p *Pane) displayRowToVirtualLocked(row int) int {
	scrollOffset := p.scrollOffset
	sbLen := len(p.scrollback)
	if scrollOffset > sbLen {
		scrollOffset = sbLen
	}
	return sbLen - scrollOffset + row
}

func (p *Pane) clearSelectionLocked() {
	p.selStart = nil
	p.selEnd = nil
	p.selecting = false
}

// ============================================================
// Scrollback helpers (called with p.mu held)
// ============================================================

// captureBeforeWrite snapshots row 0 of the VT screen before a write so that
// captureAfterWrite can detect whether the screen scrolled.
// Must be called with p.mu held.
func (p *Pane) captureBeforeWrite() {
	if p.vt == nil || p.vt.Mode()&vt10x.ModeAltScreen != 0 {
		p.pendingRow0 = nil
		return
	}
	p.pendingRow0 = p.vtSnapshotRow(0)
}

// captureAfterWrite compares the current row 0 with the pre-write snapshot.
// If they differ, the old row 0 scrolled off the top and is appended to the
// scrollback buffer. Must be called with p.mu held.
func (p *Pane) captureAfterWrite() {
	if p.pendingRow0 == nil || p.vt == nil || p.vt.Mode()&vt10x.ModeAltScreen != 0 {
		p.pendingRow0 = nil
		return
	}
	afterRow0 := p.vtSnapshotRow(0)
	if vtRowsEqualByChar(p.pendingRow0, afterRow0) {
		// Row 0 unchanged — no scroll.
		p.pendingRow0 = nil
		return
	}
	// Row 0 changed: the old row 0 scrolled off the top.
	p.appendScrollbackRow(p.pendingRow0)
	p.pendingRow0 = nil
}

// vtSnapshotRow returns a copy of row y from the VT screen.
// Must be called with p.mu held.
func (p *Pane) vtSnapshotRow(y int) []vt10x.Glyph {
	row := make([]vt10x.Glyph, p.cols)
	for x := 0; x < p.cols; x++ {
		row[x] = p.vt.Cell(x, y)
	}
	return row
}

// appendScrollbackRow adds a scrolled-off row to the buffer.
// If the buffer exceeds maxScrollback, the oldest rows are evicted and
// scrollOffset is adjusted to keep the viewed content stable.
// Must be called with p.mu held.
func (p *Pane) appendScrollbackRow(row []vt10x.Glyph) {
	p.scrollback = append(p.scrollback, row)
	if len(p.scrollback) > maxScrollback {
		removed := len(p.scrollback) - maxScrollback
		p.scrollback = p.scrollback[removed:]
		if p.scrollOffset > 0 {
			p.scrollOffset -= removed
			if p.scrollOffset < 0 {
				p.scrollOffset = 0
			}
		}
	}
	// If the user is currently scrolled up, increment the offset to keep the
	// viewed content pinned in place while new output arrives at the bottom.
	if p.scrollOffset > 0 {
		p.scrollOffset++
		if p.scrollOffset > len(p.scrollback) {
			p.scrollOffset = len(p.scrollback)
		}
	}
}

// vtRowsEqualByChar compares two rows by character content only (ignoring
// color/attribute differences), used for scroll detection.
func vtRowsEqualByChar(a, b []vt10x.Glyph) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Char != b[i].Char {
			return false
		}
	}
	return true
}

// ============================================================
// Scrollback public API
// ============================================================

// Scroll adjusts the scroll offset by delta rows. Positive delta scrolls up
// (towards older content); negative scrolls back towards the live view.
func (p *Pane) Scroll(delta int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.scrollOffset += delta
	if p.scrollOffset < 0 {
		p.scrollOffset = 0
	}
	max := len(p.scrollback)
	if p.scrollOffset > max {
		p.scrollOffset = max
	}
}

// ExitScroll returns to the live view by resetting the scroll offset.
func (p *Pane) ExitScroll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.scrollOffset = 0
}

// ScrollOffset returns the current scroll offset (0 = live view).
func (p *Pane) ScrollOffset() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.scrollOffset
}

// ScrollbackLen returns how many rows are currently in the scrollback buffer.
func (p *Pane) ScrollbackLen() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.scrollback)
}

// InAltScreen reports whether the inner VT is currently in alt-screen mode.
func (p *Pane) InAltScreen() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.vt == nil {
		return false
	}
	return p.vt.Mode()&vt10x.ModeAltScreen != 0
}

// Started reports whether the PTY has been launched.
func (p *Pane) Started() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.started
}

// Exited reports whether the child process has terminated.
func (p *Pane) Exited() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.exited
}

// IsInteractive reports whether this pane runs an interactive shell
// (i.e. no fixed command — a Console tab).
func (p *Pane) IsInteractive() bool {
	return p.cfg.IsInteractive()
}

// CursorInfo returns the cursor column, row (0-indexed), and visibility as
// tracked by the VT emulator. The outer renderer uses this to reposition the
// terminal cursor at the correct cell after each frame, instead of leaving it
// at the end of the last rendered line.
func (p *Pane) CursorInfo() (col, row int, visible bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.vt == nil {
		return 0, 0, false
	}
	cur := p.vt.Cursor()
	return cur.X, cur.Y, p.vt.CursorVisible()
}

// MouseMode returns the mouse-tracking mode flags currently active in the
// inner VT emulator. Callers can check against vt10x.ModeMouseButton,
// ModeMouseMotion, ModeMouseMany, etc. to determine which mouse events the
// child process has requested.
func (p *Pane) MouseMode() vt10x.ModeFlag {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.vt == nil {
		return 0
	}
	return p.vt.Mode() & vt10x.ModeMouseMask
}
