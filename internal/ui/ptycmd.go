package ui

import (
	"os"
	"os/exec"
	"strings"
)

// ptyCmd wraps exec.Cmd and adds the env vars needed for a proper terminal.
type ptyCmd struct {
	Cmd *exec.Cmd
}

// envVarsToUnset lists environment variables that should be stripped from the
// child PTY environment. These are injected by the parent terminal's shell
// integration and cause key-binding conflicts inside falcode (e.g.
// GHOSTTY_SHELL_FEATURES triggers readline mappings that translate Ctrl+B into
// a left-arrow sequence, intercepting falcode's prefix key).
var envVarsToUnset = []string{
	"GHOSTTY_SHELL_FEATURES",
}

func newPtyCmd(shell string, args []string, dir string) *ptyCmd {
	var cmd *exec.Cmd
	if len(args) > 0 {
		cmd = exec.Command(shell, args...)
	} else {
		cmd = exec.Command(shell)
	}
	cmd.Dir = dir
	cmd.Env = append(filteredEnv(),
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
	)
	return &ptyCmd{Cmd: cmd}
}

// filteredEnv returns os.Environ() with envVarsToUnset removed.
func filteredEnv() []string {
	env := os.Environ()
	filtered := env[:0:len(env)]
	for _, e := range env {
		keep := true
		for _, unset := range envVarsToUnset {
			if strings.HasPrefix(e, unset+"=") {
				keep = false
				break
			}
		}
		if keep {
			filtered = append(filtered, e)
		}
	}
	return filtered
}
