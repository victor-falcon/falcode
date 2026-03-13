package notify

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/victor-falcon/falcode/internal/config"
)

// Event identifies which agent state triggered an OS notification.
type Event int

const (
	// EventIdle is fired when the agent finishes a task (becomes idle or answers a question).
	EventIdle Event = iota
	// EventPermission is fired when the agent requests a permission decision.
	EventPermission
	// EventQuestion is fired when the agent is waiting for a user reply.
	EventQuestion
)

// Send shows a native OS notification for the given agent event.
// worktreeName is the branch / worktree label (e.g. "feature-x" or "main").
// projectName is the repository name (e.g. "my-project").
//
// Platform dispatch:
//   - macOS: osascript display notification — zero external dependencies,
//     works inside any terminal emulator or multiplexer (Ghostty, Zellij, tmux…)
//   - macOS (terminal-notifier): uses terminal-notifier CLI when configured and
//     available in PATH; falls back silently to osascript if not found.
//   - Linux: notify-send, if available via PATH
//
// Fire-and-forget goroutine; errors are silently discarded.
func Send(event Event, worktreeName, projectName string, notif *config.NotificationsConfig) {
	switch event {
	case EventIdle:
		if !notif.GetNotifyOnIdle() {
			return
		}
	case EventPermission:
		if !notif.GetNotifyOnPermission() {
			return
		}
	case EventQuestion:
		if !notif.GetNotifyOnQuestion() {
			return
		}
	}
	go sendAsync(event, worktreeName, projectName, notif)
}

func sendAsync(event Event, worktreeName, projectName string, notif *config.NotificationsConfig) {
	var body string
	switch event {
	case EventIdle:
		body = "Agent is done"
	case EventPermission:
		body = "Agent needs permission"
	case EventQuestion:
		body = "Agent has a question"
	}

	// subtitle: "<worktree> / <project>"
	subtitle := worktreeName + " / " + projectName

	switch runtime.GOOS {
	case "darwin":
		if notif.GetProvider() == "terminal-notifier" {
			if path, err := exec.LookPath("terminal-notifier"); err == nil {
				args := []string{
					"-title", "falcode",
					"-subtitle", subtitle,
					"-message", body,
				}
				if app := notif.GetActivateApp(); app != "" {
					args = append(args, "-activate", app)
				}
				//nolint:errcheck
				exec.Command(path, args...).Run()
				return
			}
			// terminal-notifier not found in PATH — fall through to osascript.
		}
		// osascript is always available on macOS — no external dependencies.
		// Notifications appear associated with the calling terminal app (e.g.
		// Ghostty), fully decoupled from the multiplexer (Zellij, tmux, etc.).
		script := fmt.Sprintf(
			`display notification %s with title "falcode" subtitle %s`,
			appleScriptQuote(body),
			appleScriptQuote(subtitle),
		)
		//nolint:errcheck
		exec.Command("osascript", "-e", script).Run()

	case "linux":
		if path, err := exec.LookPath("notify-send"); err == nil {
			//nolint:errcheck
			exec.Command(path, "--app-name=falcode", subtitle, body).Run()
		}
	}
}

// appleScriptQuote wraps s in AppleScript double-quotes, escaping any
// embedded double-quote characters.
func appleScriptQuote(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}
