package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Worktree represents a single git worktree entry.
type Worktree struct {
	Path   string // absolute filesystem path
	Branch string // short branch name (e.g. "main", "feature-x")
	Head   string // full SHA
	IsMain bool   // true for the primary worktree
}

// Name returns a display-friendly label for the tab.
func (w *Worktree) Name() string {
	if w.IsMain {
		return filepath.Base(w.Path)
	}
	return w.Branch
}

// Discover runs `git worktree list --porcelain` in dir and returns all
// worktrees. The first entry (main worktree) has IsMain = true.
func Discover(dir string) ([]*Worktree, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}
	return parsePorcelain(string(out))
}

// parsePorcelain parses the --porcelain output format:
//
//	worktree /abs/path
//	HEAD <sha>
//	branch refs/heads/<name>
//	                        ← blank line separator
func parsePorcelain(output string) ([]*Worktree, error) {
	var worktrees []*Worktree

	blocks := strings.Split(strings.TrimSpace(output), "\n\n")
	for i, block := range blocks {
		if strings.TrimSpace(block) == "" {
			continue
		}
		wt := &Worktree{IsMain: i == 0}
		for _, line := range strings.Split(block, "\n") {
			switch {
			case strings.HasPrefix(line, "worktree "):
				wt.Path = strings.TrimPrefix(line, "worktree ")
			case strings.HasPrefix(line, "HEAD "):
				wt.Head = strings.TrimPrefix(line, "HEAD ")
			case strings.HasPrefix(line, "branch "):
				ref := strings.TrimPrefix(line, "branch ")
				// refs/heads/main → main
				wt.Branch = strings.TrimPrefix(ref, "refs/heads/")
			case line == "bare":
				wt.Branch = "(bare)"
			case line == "detached":
				wt.Branch = "(detached)"
			}
		}
		if wt.Path == "" {
			continue
		}
		if wt.Branch == "" {
			wt.Branch = filepath.Base(wt.Path)
		}
		worktrees = append(worktrees, wt)
	}

	if len(worktrees) == 0 {
		return nil, fmt.Errorf("no worktrees found in output")
	}
	return worktrees, nil
}

// Remove runs `git worktree remove --force <path>` from repoRoot.
func Remove(repoRoot, worktreePath string) error {
	cmd := exec.Command("git", "worktree", "remove", "--force", worktreePath)
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree remove: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
