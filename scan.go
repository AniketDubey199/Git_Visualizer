package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

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

// scanGitFolderSequential is the single-threaded crawler baseline.
func scanGitFolderSequential(folder string, visited map[string]bool) []string {
	var repos []string

	canonicalPath, err := filepath.EvalSymlinks(folder)
	if err != nil {
		return repos
	}

	canonicalPath = filepath.Clean(canonicalPath)
	if visited[canonicalPath] {
		return repos
	}
	visited[canonicalPath] = true

	if isGitRepo(canonicalPath) {
		repos = append(repos, canonicalPath)
		return repos
	}

	entries, err := os.ReadDir(canonicalPath)
	if err != nil {
		return repos
	}

	for _, entry := range entries {
		if !entry.IsDir() || shouldSkipDir(entry.Name()) {
			continue
		}
		childPath := filepath.Join(canonicalPath, entry.Name())
		repos = append(repos, scanGitFolderSequential(childPath, visited)...)
	}

	return repos
}

// scanGitFolderConcurrent implements high-performance concurrent filesystem crawling using:
// 1. goroutines (`go ...`)
// 2. channels (`chan string`) for tasks and discovered repositories
// 3. sync.WaitGroup to coordinate in-flight directory scans
// 4. sync.Mutex to protect concurrent visited paths map and prevent symlink loops
func scanGitFolderConcurrent(rootFolder string, maxWorkers int) []string {
	if maxWorkers <= 0 {
		maxWorkers = runtime.NumCPU()
	}

	canonicalRoot, err := filepath.EvalSymlinks(rootFolder)
	if err != nil {
		canonicalRoot = rootFolder
	}
	canonicalRoot = filepath.Clean(canonicalRoot)

	var (
		visitedMu    sync.Mutex
		visited      = make(map[string]bool)
		workCh       = make(chan string, 10000)
		resultsCh    = make(chan string, 1000)
		wg           sync.WaitGroup
		activeTasks  int64
		repos        []string
		collectorDone = make(chan struct{})
	)

	// Collector goroutine: gathers repos from resultsCh into slice
	go func() {
		defer close(collectorDone)
		for r := range resultsCh {
			repos = append(repos, r)
		}
	}()

	// Worker function: processes directory paths concurrently
	worker := func() {
		for dir := range workCh {
			func() {
				defer func() {
					// Decrement active task counter and WaitGroup
					if atomic.AddInt64(&activeTasks, -1) == 0 {
						// When no more active directory scans are pending, close workCh
						close(workCh)
					}
					wg.Done()
				}()

				canonical, err := filepath.EvalSymlinks(dir)
				if err != nil {
					return
				}
				canonical = filepath.Clean(canonical)

				// Thread-safe visited check to avoid symlink cycles
				visitedMu.Lock()
				if visited[canonical] {
					visitedMu.Unlock()
					return
				}
				visited[canonical] = true
				visitedMu.Unlock()

				// If directory is a git repo, record it and stop recursing down this branch
				if isGitRepo(canonical) {
					resultsCh <- canonical
					return
				}

				entries, err := os.ReadDir(canonical)
				if err != nil {
					// Gracefully ignore permission errors without crashing
					return
				}

				for _, entry := range entries {
					if !entry.IsDir() || shouldSkipDir(entry.Name()) {
						continue
					}

					childPath := filepath.Join(canonical, entry.Name())
					visitedMu.Lock()
					alreadySeen := visited[childPath]
					visitedMu.Unlock()

					if alreadySeen {
						continue
					}

					atomic.AddInt64(&activeTasks, 1)
					wg.Add(1)
					select {
					case workCh <- childPath:
					default:
						// If channel buffer is full, process inline or spawn helper
						go func(p string) {
							workCh <- p
						}(childPath)
					}
				}
			}()
		}
	}

	// Initial task
	atomic.AddInt64(&activeTasks, 1)
	wg.Add(1)
	workCh <- canonicalRoot

	// Spawn worker pool goroutines
	var workerWg sync.WaitGroup
	for i := 0; i < maxWorkers; i++ {
		workerWg.Add(1)
		go func() {
			defer workerWg.Done()
			worker()
		}()
	}

	// Wait for all directory work to finish
	workerWg.Wait()
	close(resultsCh)
	<-collectorDone

	return repos
}

// recursiveScanFolder starts concurrent or sequential scanning.
func recursiveScanFolder(folder string, workers int) []string {
	if workers <= 1 {
		visited := make(map[string]bool)
		return scanGitFolderSequential(folder, visited)
	}
	return scanGitFolderConcurrent(folder, workers)
}

// scan discovers git repositories in folder and adds them to the state file.
func scan(folder string, workers int) {
	absFolder, err := filepath.Abs(folder)
	if err != nil {
		absFolder = folder
	}

	fmt.Printf("Scanning directory for Git repositories using %d worker(s): %s\n", workers, absFolder)
	repositories := recursiveScanFolder(absFolder, workers)

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
