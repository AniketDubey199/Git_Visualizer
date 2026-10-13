package main

import (
	"bufio"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

// getDotFilePath returns the path to the repos state file.
// Priority:
// 1. $GOGIT_STATS_FILE environment variable
// 2. ~/.gogitlocalstats (if writable or exists)
// 3. ./.gogitlocalstats in current working directory
func getDotFilePath() string {
	if envPath := os.Getenv("GOGIT_STATS_FILE"); envPath != "" {
		return envPath
	}

	home, err := os.UserHomeDir()
	if err != nil {
		usr, errUsr := user.Current()
		if errUsr == nil {
			home = usr.HomeDir
		}
	}

	if home != "" {
		target := filepath.Join(home, ".gogitlocalstats")
		// Test if home directory is writable
		if f, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE, 0644); err == nil {
			_ = f.Close()
			return target
		} else if _, statErr := os.Stat(target); statErr == nil {
			// If it already exists and can be read
			return target
		}
	}

	// Fallback to local working directory
	return ".gogitlocalstats"
}

// parseFileToSlice reads lines from the given file, returning unique, non-empty, clean paths.
// Complexity is O(N) using a map for deduplication instead of O(N^2) slice lookups.
func parseFileToSlice(filePath string) []string {
	f, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		fmt.Fprintf(os.Stderr, "Warning: unable to read %s: %v\n", filePath, err)
		return nil
	}
	defer f.Close()

	var lines []string
	seen := make(map[string]struct{})
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cleanPath := filepath.Clean(line)
		if _, exists := seen[cleanPath]; !exists {
			seen[cleanPath] = struct{}{}
			lines = append(lines, cleanPath)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: error scanning %s: %v\n", filePath, err)
	}

	return lines
}

// joinSlices merges two slices in O(N+M) using a hash set, preserving order.
func joinSlices(newPaths []string, existingPaths []string) []string {
	seen := make(map[string]struct{}, len(existingPaths)+len(newPaths))
	var result []string

	for _, p := range existingPaths {
		clean := filepath.Clean(strings.TrimSpace(p))
		if clean == "" {
			continue
		}
		if _, exists := seen[clean]; !exists {
			seen[clean] = struct{}{}
			result = append(result, clean)
		}
	}

	for _, p := range newPaths {
		clean := filepath.Clean(strings.TrimSpace(p))
		if clean == "" {
			continue
		}
		if _, exists := seen[clean]; !exists {
			seen[clean] = struct{}{}
			result = append(result, clean)
		}
	}

	return result
}

// dumpStringSliceToFiles writes repo paths atomically to disk using a temporary file and rename.
// It uses standard 0644 permissions (read/write by owner, read by others) instead of 0755 executable.
func dumpStringSliceToFiles(repos []string, targetPath string) error {
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	tmpFile, err := os.CreateTemp(dir, ".gogitlocalstats-tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpName := tmpFile.Name()
	defer func() {
		// Clean up temp file in case rename didn't happen
		_ = os.Remove(tmpName)
	}()

	writer := bufio.NewWriter(tmpFile)
	for _, repo := range repos {
		if _, err := writer.WriteString(repo + "\n"); err != nil {
			tmpFile.Close()
			return fmt.Errorf("failed to write to temp file: %w", err)
		}
	}

	if err := writer.Flush(); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to flush temp file: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Atomic rename replaces the target file safely
	if err := os.Rename(tmpName, targetPath); err != nil {
		return fmt.Errorf("failed to atomically rename temp file to %s: %w", targetPath, err)
	}

	return nil
}

// addNewSliceElementToFile merges new repositories into the storage file and persists them.
func addNewSliceElementToFile(filePath string, newRepos []string) error {
	existingRepos := parseFileToSlice(filePath)
	merged := joinSlices(newRepos, existingRepos)
	return dumpStringSliceToFiles(merged, filePath)
}
