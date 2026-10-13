package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsGitRepo_DirectoryAndFile(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Regular repository with .git dir
	regRepo := filepath.Join(tempDir, "regular_repo")
	if err := os.MkdirAll(filepath.Join(regRepo, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if !isGitRepo(regRepo) {
		t.Errorf("Expected isGitRepo true for directory with .git dir")
	}

	// 2. Worktree/Submodule with .git file
	worktreeRepo := filepath.Join(tempDir, "worktree_repo")
	if err := os.MkdirAll(worktreeRepo, 0755); err != nil {
		t.Fatal(err)
	}
	gitFile := filepath.Join(worktreeRepo, ".git")
	if err := os.WriteFile(gitFile, []byte("gitdir: /path/to/parent/.git/worktrees/wt\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if !isGitRepo(worktreeRepo) {
		t.Errorf("Expected isGitRepo true for worktree/submodule with .git file")
	}

	// 3. Normal non-git directory
	nonGit := filepath.Join(tempDir, "non_git")
	if err := os.MkdirAll(nonGit, 0755); err != nil {
		t.Fatal(err)
	}
	if isGitRepo(nonGit) {
		t.Errorf("Expected isGitRepo false for empty directory")
	}
}

func TestScanGitFolder_SymlinkCycle(t *testing.T) {
	tempDir := t.TempDir()

	dirA := filepath.Join(tempDir, "dirA")
	if err := os.MkdirAll(dirA, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a recursive symlink cycle: dirA/loop -> dirA
	loopLink := filepath.Join(dirA, "loop")
	if err := os.Symlink(dirA, loopLink); err != nil {
		t.Skip("Symlinks not supported on this filesystem")
	}

	// Create a nested git repo inside dirA
	repoDir := filepath.Join(dirA, "myrepo")
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	// Run sequential crawler
	visited := make(map[string]bool)
	reposSeq := scanGitFolderSequential(dirA, visited)
	if len(reposSeq) != 1 {
		t.Fatalf("Expected 1 repo discovered sequentially despite symlink cycle, got %d", len(reposSeq))
	}

	// Run concurrent crawler with worker pool, channels and WaitGroup
	reposConc := scanGitFolderConcurrent(dirA, 4)
	if len(reposConc) != 1 {
		t.Fatalf("Expected 1 repo discovered concurrently despite symlink cycle, got %d", len(reposConc))
	}
}
