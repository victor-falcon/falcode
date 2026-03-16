package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorktreeName(t *testing.T) {
	mainWT := &Worktree{Path: "/tmp/project", Branch: "main", IsMain: true}
	if got := mainWT.Name(); got != "project" {
		t.Fatalf("Name() = %q, want %q", got, "project")
	}

	featureWT := &Worktree{Path: "/tmp/project-feature", Branch: "feature-x", IsMain: false}
	if got := featureWT.Name(); got != "feature-x" {
		t.Fatalf("Name() = %q, want %q", got, "feature-x")
	}
}

func TestParsePorcelain(t *testing.T) {
	output := `worktree /tmp/repo
HEAD abc123
branch refs/heads/main

worktree /tmp/repo-feature
HEAD def456
branch refs/heads/feature-x

worktree /tmp/repo-detached
HEAD fedcba
detached

worktree /tmp/repo-bare
HEAD 999999
bare

HEAD no-path
branch refs/heads/ignored

worktree /tmp/repo-fallback
HEAD aaa111`

	got, err := parsePorcelain(output)
	if err != nil {
		t.Fatalf("parsePorcelain() error = %v", err)
	}

	if len(got) != 5 {
		t.Fatalf("len(worktrees) = %d, want 5", len(got))
	}
	if !got[0].IsMain {
		t.Fatalf("first worktree IsMain = false, want true")
	}
	if got[1].Branch != "feature-x" {
		t.Fatalf("feature branch = %q, want %q", got[1].Branch, "feature-x")
	}
	if got[2].Branch != "(detached)" {
		t.Fatalf("detached branch = %q, want %q", got[2].Branch, "(detached)")
	}
	if got[3].Branch != "(bare)" {
		t.Fatalf("bare branch = %q, want %q", got[3].Branch, "(bare)")
	}
	if got[4].Branch != "repo-fallback" {
		t.Fatalf("fallback branch = %q, want %q", got[4].Branch, "repo-fallback")
	}
}

func TestParsePorcelainEmptyOutputReturnsError(t *testing.T) {
	if _, err := parsePorcelain(" \n\n "); err == nil {
		t.Fatalf("parsePorcelain() error = nil, want error")
	}
}

func TestPlannedPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	got, err := PlannedPath("/tmp/my-repo", "feature-x")
	if err != nil {
		t.Fatalf("PlannedPath() error = %v", err)
	}
	want := filepath.Join(home, ".falcode", "worktrees", "my-repo", "feature-x")
	if got != want {
		t.Fatalf("PlannedPath() = %q, want %q", got, want)
	}
}

func TestFindWorktreeScript(t *testing.T) {
	root := t.TempDir()
	second := filepath.Join(root, "worktree.sh")
	if err := os.WriteFile(second, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}

	got := FindWorktreeScript(root, []string{"falcode.sh", "worktree.sh"})
	if got != second {
		t.Fatalf("FindWorktreeScript() = %q, want %q", got, second)
	}

	missing := FindWorktreeScript(root, []string{"falcode.sh", "setup.sh"})
	if missing != "" {
		t.Fatalf("FindWorktreeScript() = %q, want empty string", missing)
	}
}
