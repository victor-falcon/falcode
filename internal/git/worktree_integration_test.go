package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitEnv() []string {
	return append(os.Environ(),
		"GIT_AUTHOR_NAME=Falcode Tests",
		"GIT_AUTHOR_EMAIL=tests@falcode.local",
		"GIT_COMMITTER_NAME=Falcode Tests",
		"GIT_COMMITTER_EMAIL=tests@falcode.local",
	)
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = gitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func setupRepo(t *testing.T) string {
	t.Helper()

	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}

	runGit(t, repo, "init")
	runGit(t, repo, "branch", "-M", "main")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "initial commit")

	return repo
}

func TestDiscoverAndHasUncommittedChangesIntegration(t *testing.T) {
	repo := setupRepo(t)
	featurePath := filepath.Join(t.TempDir(), "feature-worktree")
	runGit(t, repo, "worktree", "add", "-b", "feature-discover", featurePath)

	worktrees, err := Discover(repo)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if len(worktrees) != 2 {
		t.Fatalf("len(Discover()) = %d, want 2", len(worktrees))
	}
	if !worktrees[0].IsMain {
		t.Fatalf("first worktree IsMain = false, want true")
	}
	resolvedRepo, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatalf("EvalSymlinks(repo) error = %v", err)
	}
	resolvedPath, err := filepath.EvalSymlinks(worktrees[0].Path)
	if err != nil {
		t.Fatalf("EvalSymlinks(worktree path) error = %v", err)
	}
	if resolvedPath != resolvedRepo {
		t.Fatalf("main worktree path = %q, want %q", resolvedPath, resolvedRepo)
	}

	var feature *Worktree
	for _, wt := range worktrees {
		if wt.Branch == "feature-discover" {
			feature = wt
			break
		}
	}
	if feature == nil {
		t.Fatalf("Discover() did not include feature-discover: %+v", worktrees)
	}
	if HasUncommittedChanges(repo) {
		t.Fatalf("HasUncommittedChanges(repo) = true, want false")
	}
	if HasUncommittedChanges(feature.Path) {
		t.Fatalf("HasUncommittedChanges(feature) = true, want false")
	}

	if err := os.WriteFile(filepath.Join(feature.Path, "dirty.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatalf("write dirty file: %v", err)
	}
	if !HasUncommittedChanges(feature.Path) {
		t.Fatalf("HasUncommittedChanges(feature) = false, want true after edit")
	}
}

func TestCreateCreatesNewAndExistingBranchWorktrees(t *testing.T) {
	repo := setupRepo(t)
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv("HOME", home)

	newWT, err := Create(repo, "feature-new-wt", "feature-new")
	if err != nil {
		t.Fatalf("Create(new branch) error = %v", err)
	}
	if newWT.Branch != "feature-new" {
		t.Fatalf("newWT.Branch = %q, want %q", newWT.Branch, "feature-new")
	}
	if newWT.Head == "" {
		t.Fatalf("newWT.Head = empty, want commit sha")
	}
	if current := runGit(t, newWT.Path, "branch", "--show-current"); current != "feature-new" {
		t.Fatalf("new worktree branch = %q, want %q", current, "feature-new")
	}
	if !branchExists(repo, "feature-new") {
		t.Fatalf("branchExists(feature-new) = false, want true")
	}

	runGit(t, repo, "branch", "feature-existing")
	existingWT, err := Create(repo, "feature-existing-wt", "feature-existing")
	if err != nil {
		t.Fatalf("Create(existing branch) error = %v", err)
	}
	if current := runGit(t, existingWT.Path, "branch", "--show-current"); current != "feature-existing" {
		t.Fatalf("existing worktree branch = %q, want %q", current, "feature-existing")
	}
	if !strings.HasPrefix(existingWT.Path, filepath.Join(home, ".falcode", "worktrees", filepath.Base(repo))+string(os.PathSeparator)) {
		t.Fatalf("existingWT.Path = %q, want it under HOME worktree bucket", existingWT.Path)
	}
}

func TestRemoveDeletesWorktreeFolderAndBranchIntegration(t *testing.T) {
	repo := setupRepo(t)
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv("HOME", home)

	wt, err := Create(repo, "feature-remove-wt", "feature-remove")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(wt.Path, "untracked.txt"), []byte("data\n"), 0o644); err != nil {
		t.Fatalf("write untracked file: %v", err)
	}

	parent := filepath.Dir(wt.Path)
	if err := Remove(repo, wt.Path, wt.Branch); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if _, err := os.Stat(wt.Path); !os.IsNotExist(err) {
		t.Fatalf("worktree path still exists after Remove(): err=%v", err)
	}
	if branchExists(repo, wt.Branch) {
		t.Fatalf("branchExists(%q) = true, want false", wt.Branch)
	}
	if _, err := os.Stat(parent); !os.IsNotExist(err) {
		t.Fatalf("parent worktree bucket still exists: %v", err)
	}
}

func TestRemoveRefRemovesGitRegistrationIntegration(t *testing.T) {
	repo := setupRepo(t)
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv("HOME", home)

	wt, err := Create(repo, "feature-remove-ref-wt", "feature-remove-ref")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := RemoveRef(repo, wt.Path, wt.Branch); err != nil {
		t.Fatalf("RemoveRef() error = %v", err)
	}

	list := runGit(t, repo, "worktree", "list", "--porcelain")
	if strings.Contains(list, wt.Path) {
		t.Fatalf("worktree list still contains removed path %q\n%s", wt.Path, list)
	}
	if branchExists(repo, wt.Branch) {
		t.Fatalf("branchExists(%q) = true, want false", wt.Branch)
	}
	if _, err := os.Stat(wt.Path); err != nil && !os.IsNotExist(err) {
		t.Fatalf("stat after RemoveRef() error = %v", err)
	}

	if err := RemoveFolder(wt.Path); err != nil {
		t.Fatalf("RemoveFolder() error = %v", err)
	}
}

func TestHasUncommittedChangesReturnsTrueForInvalidPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), fmt.Sprintf("missing-%d", os.Getpid()))
	if !HasUncommittedChanges(missing) {
		t.Fatalf("HasUncommittedChanges(missing) = false, want true")
	}
}
