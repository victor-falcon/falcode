package ui

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
)

// AgentStatus represents the current state of a command pane that is running
// an agent (or any non-interactive command). It is derived from events received
// via the FIFO status pipe that the OpenCode falcode plugin writes to.
type AgentStatus int

const (
	// AgentStatusIdle means no activity yet — no icon shown.
	AgentStatusIdle AgentStatus = iota
	// AgentStatusWorking means the agent is actively processing (spinner).
	AgentStatusWorking
	// AgentStatusPermission means the agent is waiting for a permission grant ("!").
	AgentStatusPermission
	// AgentStatusQuestion means the agent finished its turn and is waiting
	// for the user to reply ("?").
	AgentStatusQuestion
	// AgentStatusDone means the agent has emitted an explicit completion/idle
	// event for the current turn.
	AgentStatusDone
)

// PaneStatusMsg is dispatched by the FIFO reader goroutine when the agent
// reports a status change via the named pipe.
type PaneStatusMsg struct {
	Key    PaneKey
	Status AgentStatus
}

// agentPipeEvent is the JSON envelope written by the OpenCode falcode plugin.
type agentPipeEvent struct {
	Type   string `json:"type"`
	Status string `json:"status,omitempty"`
}

// agentPipeDir returns the per-process directory used for all status FIFOs.
// The directory name includes the current PID so that multiple concurrent
// falcode instances never share pipes.
func agentPipeDir() string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("falcode-%d", os.Getpid()))
}

// agentPipePath returns the canonical FIFO path for a given pane key.
func agentPipePath(key PaneKey) string {
	return filepath.Join(agentPipeDir(), fmt.Sprintf("ws%d-tab%d.fifo", key.Workspace, key.Tab))
}

// createAgentPipe creates the per-process temp directory (if needed) and the
// FIFO file for the given pane key. Returns the path to the FIFO.
func createAgentPipe(key PaneKey) (string, error) {
	dir := agentPipeDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create agent pipe dir: %w", err)
	}
	path := agentPipePath(key)
	// Remove a stale FIFO from a previous run if present.
	_ = os.Remove(path)
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		return "", fmt.Errorf("mkfifo %s: %w", path, err)
	}
	return path, nil
}

// removeAgentPipe deletes the FIFO at the given path.
func removeAgentPipe(path string) {
	_ = os.Remove(path)
}

// readAgentPipe opens the FIFO at path (using O_RDWR to avoid blocking on open
// when no writer is attached yet) and scans it line-by-line for JSON events.
// Each event is translated to an AgentStatus and dispatched via send as a
// PaneStatusMsg. The goroutine exits when the file is closed or an EOF/error
// is encountered (which happens when the child process and its plugin exit).
func readAgentPipe(path string, key PaneKey, send func(tea.Msg)) {
	// Open with O_RDWR so the kernel does not block waiting for a writer.
	// The write end will be opened by the OpenCode plugin when it starts.
	f, err := os.OpenFile(path, os.O_RDWR, os.ModeNamedPipe)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var evt agentPipeEvent
		if err := json.Unmarshal(line, &evt); err != nil {
			continue
		}
		var status AgentStatus
		switch evt.Type {
		case "status":
			// "busy"/"running" update the working spinner. "idle" is treated as a
			// neutral state here; completion notifications are driven by the
			// explicit "idle" event emitted after the turn fully settles.
			switch evt.Status {
			case "busy", "running":
				status = AgentStatusWorking
			case "idle":
				status = AgentStatusIdle
			default:
				status = AgentStatusIdle
			}
		case "idle":
			status = AgentStatusDone
		case "permission":
			status = AgentStatusPermission
		case "question":
			status = AgentStatusQuestion
		default:
			continue
		}
		send(PaneStatusMsg{Key: key, Status: status})
	}
}
