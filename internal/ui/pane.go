package ui

import (
	"fmt"
	"os"
	"sync"

	"github.com/charmbracelet/bubbletea"
	"github.com/creack/pty"
	"github.com/hinshun/vt10x"
	"github.com/victor-falcon/falcode/internal/config"
)

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

// Pane manages a single PTY process and its VT100 terminal emulator.
type Pane struct {
	cfg *config.Tab
	key PaneKey
	dir string // working directory for the process

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
}

// NewPane creates a Pane but does not start it yet.
func NewPane(key PaneKey, cfg *config.Tab, dir string, cols, rows int) *Pane {
	return &Pane{
		cfg:  cfg,
		key:  key,
		dir:  dir,
		cols: cols,
		rows: rows,
		done: make(chan struct{}),
	}
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

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				p.mu.Lock()
				p.vt.Write(buf[:n])
				p.mu.Unlock()
				send(PaneOutputMsg{Key: p.key})
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
		close(p.done)
		send(PaneExitMsg{Key: p.key, Err: exitErr})
	}()

	return nil
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

// Stop terminates the PTY process.
func (p *Pane) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ptmx != nil {
		p.ptmx.Close()
		p.ptmx = nil
	}
}

// View renders the current VT screen state as an ANSI string.
func (p *Pane) View() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.vt == nil {
		return ""
	}

	return renderVT(p.vt, p.cols, p.rows)
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
