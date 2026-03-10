package git

import (
	"fmt"
	"os"
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

// Create creates a new git worktree at ~/.falcode/worktrees/{folderName}/{worktreeName}.
// If branchName already exists locally it is checked out; otherwise it is created
// from the current HEAD. Returns the newly created Worktree on success.
func Create(repoRoot, worktreeName, branchName string) (*Worktree, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home dir: %w", err)
	}

	folderName := filepath.Base(repoRoot)
	worktreePath := filepath.Join(home, ".falcode", "worktrees", folderName, worktreeName)

	// Ensure the parent directory exists; git worktree add creates the leaf but
	// not intermediate directories.
	if err := os.MkdirAll(filepath.Dir(worktreePath), 0o755); err != nil {
		return nil, fmt.Errorf("create worktree parent dir: %w", err)
	}

	// Decide whether to check out an existing branch or create a new one.
	var gitCmd *exec.Cmd
	if branchExists(repoRoot, branchName) {
		gitCmd = exec.Command("git", "worktree", "add", worktreePath, branchName)
	} else {
		gitCmd = exec.Command("git", "worktree", "add", "-b", branchName, worktreePath)
	}
	gitCmd.Dir = repoRoot
	if out, err := gitCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("git worktree add: %s: %w", strings.TrimSpace(string(out)), err)
	}

	// Resolve HEAD of the newly created worktree.
	headOut, _ := exec.Command("git", "-C", worktreePath, "rev-parse", "HEAD").Output()
	head := strings.TrimSpace(string(headOut))

	return &Worktree{
		Path:   worktreePath,
		Branch: branchName,
		Head:   head,
		IsMain: false,
	}, nil
}

// branchExists returns true when branchName is a local branch in repoRoot.
func branchExists(repoRoot, branchName string) bool {
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branchName)
	cmd.Dir = repoRoot
	return cmd.Run() == nil
}

// HasUncommittedChanges reports whether the worktree at path has any tracked or
// untracked changes (using git status --porcelain).
func HasUncommittedChanges(worktreePath string) bool {
	out, err := exec.Command("git", "-C", worktreePath, "status", "--porcelain").Output()
	if err != nil {
		// If git fails (e.g. detached HEAD, bare), assume dirty to be safe.
		return true
	}
	return strings.TrimSpace(string(out)) != ""
}
