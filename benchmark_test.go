package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// setupMockRepos creates N temporary git repositories with commits for benchmarking.
func setupMockRepos(tb testing.TB, repoCount int, commitsPerRepo int) ([]string, string) {
	tb.Helper()
	tempDir := tb.TempDir()
	var repoPaths []string
	authorEmail := "bench@example.com"

	now := time.Now()

	for r := 0; r < repoCount; r++ {
		dir := filepath.Join(tempDir, fmt.Sprintf("repo_%d", r))
		repo, err := git.PlainInit(dir, false)
		if err != nil {
			tb.Fatalf("failed to init repo: %v", err)
		}

		w, err := repo.Worktree()
		if err != nil {
			tb.Fatalf("failed to get worktree: %v", err)
		}

		for c := 0; c < commitsPerRepo; c++ {
			filename := fmt.Sprintf("file_%d.txt", c)
			filePath := filepath.Join(dir, filename)
			if err := os.WriteFile(filePath, []byte(fmt.Sprintf("content %d", c)), 0644); err != nil {
				tb.Fatalf("failed to write file: %v", err)
			}

			if _, err := w.Add(filename); err != nil {
				tb.Fatalf("failed to stage file: %v", err)
			}

			commitTime := now.AddDate(0, 0, -(c % 150))
			_, err = w.Commit(fmt.Sprintf("Commit %d in repo %d", c, r), &git.CommitOptions{
				Author: &object.Signature{
					Name:  "Benchmarker",
					Email: authorEmail,
					When:  commitTime,
				},
			})
			if err != nil {
				tb.Fatalf("failed to commit: %v", err)
			}
		}

		repoPaths = append(repoPaths, dir)
	}

	return repoPaths, authorEmail
}

func BenchmarkProcessRepositoriesSequential(b *testing.B) {
	repos, email := setupMockRepos(b, 8, 20)
	now := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res := processRepositoriesSequential(email, repos, daysInLastSixMonths, now)
		if len(res) == 0 {
			b.Fatal("unexpected empty result")
		}
	}
}

func BenchmarkProcessRepositoriesConcurrent_4Workers(b *testing.B) {
	repos, email := setupMockRepos(b, 8, 20)
	now := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res := processRepositoriesConcurrent(email, repos, daysInLastSixMonths, 4, now)
		if len(res) == 0 {
			b.Fatal("unexpected empty result")
		}
	}
}

func BenchmarkProcessRepositoriesConcurrent_8Workers(b *testing.B) {
	repos, email := setupMockRepos(b, 8, 20)
	now := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res := processRepositoriesConcurrent(email, repos, daysInLastSixMonths, 8, now)
		if len(res) == 0 {
			b.Fatal("unexpected empty result")
		}
	}
}

func BenchmarkScanGitFolderSequential(b *testing.B) {
	repos, _ := setupMockRepos(b, 16, 2)
	rootDir := filepath.Dir(repos[0])

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		visited := make(map[string]bool)
		found := scanGitFolderSequential(rootDir, visited)
		if len(found) != 16 {
			b.Fatalf("expected 16 repos, got %d", len(found))
		}
	}
}

func BenchmarkScanGitFolderConcurrent_4Workers(b *testing.B) {
	repos, _ := setupMockRepos(b, 16, 2)
	rootDir := filepath.Dir(repos[0])

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		found := scanGitFolderConcurrent(rootDir, 4)
		if len(found) != 16 {
			b.Fatalf("expected 16 repos, got %d", len(found))
		}
	}
}
