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

// RemoveRef runs `git worktree remove --force <path>` from repoRoot to
// deregister the worktree from git, then deletes the local branch with
// `git branch -D <branch>` (best-effort). It does NOT remove the directory
// from disk — call RemoveFolder for that.
func RemoveRef(repoRoot, worktreePath, branch string) error {
	cmd := exec.Command("git", "worktree", "remove", "--force", worktreePath)
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree remove: %s: %w", strings.TrimSpace(string(out)), err)
	}

	// Best-effort local branch deletion; ignore errors (e.g. detached HEAD).
	if branch != "" && branch != "(detached)" && branch != "(bare)" {
		del := exec.Command("git", "branch", "-D", branch)
		del.Dir = repoRoot
		del.Run() //nolint:errcheck
	}

	return nil
}

// RemoveFolder removes the worktree directory from disk and cleans up the
// now-empty parent bucket directory created by Create. This is intentionally
// separate from RemoveRef so callers can report progress between the two steps.
func RemoveFolder(worktreePath string) error {
	// Ensure the worktree folder is fully removed. git worktree remove --force
	// may leave it behind when it contains untracked files not in the index.
	if err := os.RemoveAll(worktreePath); err != nil {
		return fmt.Errorf("remove worktree folder: %w", err)
	}

	// Remove the parent directory if it is now empty (the per-repo bucket
	// directory created by Create, e.g. ~/.falcode/worktrees/{folderName}).
	parent := filepath.Dir(worktreePath)
	if entries, err := os.ReadDir(parent); err == nil && len(entries) == 0 {
		if err := os.Remove(parent); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove worktree parent folder: %w", err)
		}
	}

	return nil
}

// Remove is a convenience wrapper that calls RemoveRef followed by RemoveFolder.
func Remove(repoRoot, worktreePath, branch string) error {
	if err := RemoveRef(repoRoot, worktreePath, branch); err != nil {
		return err
	}
	return RemoveFolder(worktreePath)
}

// PlannedPath returns the filesystem path falcode will use for a worktree.
func PlannedPath(repoRoot, worktreeName string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}

	folderName := filepath.Base(repoRoot)
	return filepath.Join(home, ".falcode", "worktrees", folderName, worktreeName), nil
}

// Create creates a new git worktree at ~/.falcode/worktrees/{folderName}/{worktreeName}.
// If branchName already exists locally it is checked out; otherwise it is created
// from the current HEAD. Returns the newly created Worktree on success.
func Create(repoRoot, worktreeName, branchName string) (*Worktree, error) {
	worktreePath, err := PlannedPath(repoRoot, worktreeName)
	if err != nil {
		return nil, err
	}

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

// FindWorktreeScript walks scriptPaths (relative to worktreePath) and returns
// the absolute path of the first existing file. Returns "" if none is found.
func FindWorktreeScript(worktreePath string, scriptPaths []string) string {
	for _, p := range scriptPaths {
		full := filepath.Join(worktreePath, p)
		if _, err := os.Stat(full); err == nil {
			return full
		}
	}
	return ""
}
