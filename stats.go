package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

const outofRange = 99999
const daysInLastSixMonths = 183
const weeksInLastSixMonths = 26

// RepoResult stores the processed commit histogram and error for a single repository.
type RepoResult struct {
	RepoPath string
	Commits  map[int]int
	Total    int
	Err      error
}

// parseEmails splits comma-separated emails and converts them to a lowercase lookup set.
func parseEmails(emailStr string) map[string]struct{} {
	parts := strings.Split(emailStr, ",")
	set := make(map[string]struct{}, len(parts))
	for _, p := range parts {
		cleaned := strings.ToLower(strings.TrimSpace(p))
		if cleaned != "" {
			set[cleaned] = struct{}{}
		}
	}
	return set
}

// fillCommits inspects a single repository and populates a thread-local commits map.
// Crucially:
// 1. It returns an error instead of calling panic(err).
// 2. It accepts emailSet for multi-email matching.
// 3. It uses O(1) DST-safe date calculation (countDaysSinceDate).
// 4. It deduplicates commits by hash to avoid double-counting rebased, cherry-picked, or multi-branch commits.
func fillCommits(repoPath string, emailSet map[string]struct{}, maxDays int, now time.Time, seenCommits *sync.Map) (map[int]int, int, error) {
	localCommits := make(map[int]int)

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to open repo %s: %w", repoPath, err)
	}

	ref, err := repo.Head()
	if err != nil {
		// Empty repo or detached without head is not fatal
		return localCommits, 0, nil
	}

	iterator, err := repo.Log(&git.LogOptions{From: ref.Hash()})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get commit log for %s: %w", repoPath, err)
	}
	defer iterator.Close()

	offset := calcOffset(now)
	totalMatched := 0

	err = iterator.ForEach(func(c *object.Commit) error {
		// Deduplicate commits by hash across branches/repos if seenCommits is provided
		if seenCommits != nil {
			if _, loaded := seenCommits.LoadOrStore(c.Hash, struct{}{}); loaded {
				return nil
			}
		}

		authorEmail := strings.ToLower(strings.TrimSpace(c.Author.Email))
		if _, matches := emailSet[authorEmail]; !matches {
			return nil
		}

		// O(1) DST-safe calendar difference
		days := countDaysSinceDate(c.Author.When, now, maxDays)
		if days == outofRange {
			return nil
		}

		days += offset
		localCommits[days]++
		totalMatched++
		return nil
	})

	if err != nil {
		// Non-fatal error: some commits in history may be unreadable, but we return whatever was gathered
		return localCommits, totalMatched, fmt.Errorf("warning during iteration of %s: %w", repoPath, err)
	}

	return localCommits, totalMatched, nil
}

// processRepositoriesSequential processes all repos sequentially (single-threaded baseline).
func processRepositoriesSequential(emailStr string, repos []string, maxDays int, now time.Time) map[int]int {
	emailSet := parseEmails(emailStr)
	totalCommits := make(map[int]int, maxDays)
	for i := maxDays; i > 0; i-- {
		totalCommits[i] = 0
	}

	var seenCommits sync.Map

	for _, path := range repos {
		localMap, _, err := fillCommits(path, emailSet, maxDays, now, &seenCommits)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Notice: %v\n", err)
		}
		for day, count := range localMap {
			totalCommits[day] += count
		}
	}

	return totalCommits
}

// processRepositoriesConcurrent processes all repositories concurrently using a worker pool.
// Why this architecture?
// 1. Git packfile decompression and commit log walking are CPU- and I/O-intensive.
//    Running per-repo in parallel yields near-linear speedups on multi-core machines.
// 2. Each worker maintains its OWN thread-local map. No shared map mutations happen,
//    eliminating any possibility of Go's `fatal error: concurrent map writes` without
//    any mutex contention!
// 3. Results are aggregated deterministically by the collector.
func processRepositoriesConcurrent(emailStr string, repos []string, maxDays int, workers int, now time.Time) map[int]int {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if workers > len(repos) && len(repos) > 0 {
		workers = len(repos)
	}

	emailSet := parseEmails(emailStr)
	totalCommits := make(map[int]int, maxDays)
	for i := maxDays; i > 0; i-- {
		totalCommits[i] = 0
	}

	if len(repos) == 0 {
		return totalCommits
	}

	jobs := make(chan string, len(repos))
	results := make(chan RepoResult, len(repos))
	var seenCommits sync.Map
	var wg sync.WaitGroup

	// Launch worker pool
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				localMap, total, err := fillCommits(path, emailSet, maxDays, now, &seenCommits)
				results <- RepoResult{
					RepoPath: path,
					Commits:  localMap,
					Total:    total,
					Err:      err,
				}
			}
		}()
	}

	// Feed jobs
	for _, path := range repos {
		jobs <- path
	}
	close(jobs)

	// Close results channel once all workers finish
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collector merges results safely into master commits map
	for res := range results {
		if res.Err != nil {
			fmt.Fprintf(os.Stderr, "Notice: %v\n", res.Err)
		}
		for day, count := range res.Commits {
			totalCommits[day] += count
		}
	}

	return totalCommits
}

// stats is the standard entry point for CLI visualization.
func stats(emailStr string, workers int) {
	filePath := getDotFilePath()
	repos := parseFileToSlice(filePath)

	if len(repos) == 0 {
		fmt.Printf("No Git repositories configured in %s.\nUse -add <directory> to register repositories.\n", filePath)
		return
	}

	fmt.Printf("Analyzing %d repositories using %d concurrent worker(s) for [%s]...\n", len(repos), workers, emailStr)
	start := time.Now()
	commits := processRepositoriesConcurrent(emailStr, repos, daysInLastSixMonths, workers, time.Now())
	duration := time.Since(start)

	totalCount := 0
	for _, c := range commits {
		totalCount += c
	}

	fmt.Printf("Completed in %v | Total commits in last 6 months: %d\n\n", duration, totalCount)
	printCommitStats(commits)
}
