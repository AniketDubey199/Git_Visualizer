package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CrawlerConfig configures the repository filesystem crawler.
type CrawlerConfig struct {
	FollowSymlinks bool
	MaxDepth       int
}

// isGitRepo checks if a given directory is a valid git repository:
// 1. Standard repo: contains a .git/ directory
// 2. Git worktree or submodule: contains a .git file containing "gitdir: ..."
// 3. Bare repository: contains HEAD, refs/, and objects/
func isGitRepo(dir string) bool {
	gitPath := filepath.Join(dir, ".git")
	info, err := os.Stat(gitPath)
	if err == nil {
		if info.IsDir() {
			return true
		}
		// In submodules and worktrees, .git is a text file pointing to the actual gitdir:
		// "gitdir: ../../.git/modules/submodule"
		if info.Mode().IsRegular() {
			data, readErr := os.ReadFile(gitPath)
			if readErr == nil && bytes.HasPrefix(bytes.TrimSpace(data), []byte("gitdir:")) {
				return true
			}
		}
	}

	// Check if this is a bare git repository
	headPath := filepath.Join(dir, "HEAD")
	refsPath := filepath.Join(dir, "refs")
	objsPath := filepath.Join(dir, "objects")

	headStat, err1 := os.Stat(headPath)
	refsStat, err2 := os.Stat(refsPath)
	objsStat, err3 := os.Stat(objsPath)

	if err1 == nil && err2 == nil && err3 == nil {
		if !headStat.IsDir() && refsStat.IsDir() && objsStat.IsDir() {
			return true
		}
	}

	return false
}

// shouldSkipDir checks whether a directory should be skipped to prevent scanning
// large dependency directories, caches, or hidden non-git directories.
func shouldSkipDir(name string) bool {
	switch name {
	case "vendor", "node_modules", "target", ".cache", ".npm", ".cargo", ".venv", ".git":
		return true
	}
	// Skip general hidden folders except if root
	if strings.HasPrefix(name, ".") {
		return true
	}
	return false
}

// scanGitFolder crawls directory trees recursively to find git repositories.
// It handles:
// - Permission errors gracefully (logs warning/debug instead of log.Fatal)
// - Symlink loops / recursion cycles using canonical real path tracking
// - Worktrees and submodules where .git is a file
func scanGitFolder(folder string, visited map[string]bool) []string {
	var repos []string

	canonicalPath, err := filepath.EvalSymlinks(folder)
	if err != nil {
		// Non-fatal: unreadable path or dangling symlink
		return repos
	}

	canonicalPath = filepath.Clean(canonicalPath)
	if visited[canonicalPath] {
		// Prevent symlink loops and duplicate scanning
		return repos
	}
	visited[canonicalPath] = true

	// Check if the current directory is a git repository
	if isGitRepo(canonicalPath) {
		repos = append(repos, canonicalPath)
		// Do not recurse into nested subfolders of an already-discovered repository
		// (submodules can still be discovered if inspected at top-level or configured)
		return repos
	}

	entries, err := os.ReadDir(canonicalPath)
	if err != nil {
		// Non-fatal: permission denied or unreadable directory.
		// A crawler must NOT abort the entire crawl because of one permission error!
		return repos
	}

	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() {
			continue
		}

		if shouldSkipDir(name) {
			continue
		}

		childPath := filepath.Join(canonicalPath, name)
		found := scanGitFolder(childPath, visited)
		repos = append(repos, found...)
	}

	return repos
}

// recursiveScanFolder starts the recursive search for git repositories.
func recursiveScanFolder(folder string) []string {
	visited := make(map[string]bool)
	return scanGitFolder(folder, visited)
}

// scan discovers git repositories in folder and adds them to the state file.
func scan(folder string) {
	absFolder, err := filepath.Abs(folder)
	if err != nil {
		absFolder = folder
	}

	fmt.Printf("Scanning directory for Git repositories: %s\n", absFolder)
	repositories := recursiveScanFolder(absFolder)

	if len(repositories) == 0 {
		fmt.Println("No Git repositories found.")
		return
	}

	fmt.Printf("\nFound %d Git repositories:\n", len(repositories))
	for _, repo := range repositories {
		fmt.Printf("  • %s\n", repo)
	}

	filePath := getDotFilePath()
	if err := addNewSliceElementToFile(filePath, repositories); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving repositories to %s: %v\n", filePath, err)
		return
	}

	fmt.Printf("\nSuccessfully saved to %s\n", filePath)
}
