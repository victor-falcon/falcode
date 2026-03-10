package ui

import (
	"os"
	"os/exec"
)

// ptyCmd wraps exec.Cmd and adds the env vars needed for a proper terminal.
type ptyCmd struct {
	Cmd *exec.Cmd
}

func newPtyCmd(shell string, args []string, dir string) *ptyCmd {
	var cmd *exec.Cmd
	if len(args) > 0 {
		cmd = exec.Command(shell, args...)
	} else {
		cmd = exec.Command(shell)
	}
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
	)
	return &ptyCmd{Cmd: cmd}
}
