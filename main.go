package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
)

func main() {
	var (
		folder     string
		email      string
		workers    int
		asJSON     bool
		serveWeb   bool
		port       int
		listRepos  bool
		clearRepos bool
	)

	flag.StringVar(&folder, "add", "", "Add a new folder to scan recursively for git repositories")
	flag.StringVar(&email, "email", "your@email.com", "Comma-separated email address(es) to filter commits by")
	flag.IntVar(&workers, "workers", runtime.NumCPU(), "Number of concurrent workers for processing repos")
	flag.BoolVar(&asJSON, "json", false, "Output commit statistics as JSON to stdout")
	flag.BoolVar(&serveWeb, "serve", false, "Start web server with interactive commit heatmap and JSON API")
	flag.IntVar(&port, "port", 8080, "Port for web server (used with -serve)")
	flag.BoolVar(&listRepos, "list", false, "List all repositories currently registered in the state file")
	flag.BoolVar(&clearRepos, "clear", false, "Clear all repositories registered in the state file")

	flag.Parse()

	dotFile := getDotFilePath()

	if clearRepos {
		if err := os.Remove(dotFile); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error clearing %s: %v\n", dotFile, err)
			os.Exit(1)
		}
		fmt.Printf("Cleared registered repositories from %s\n", dotFile)
		return
	}

	if listRepos {
		repos := parseFileToSlice(dotFile)
		if len(repos) == 0 {
			fmt.Printf("No repositories currently registered in %s\n", dotFile)
			return
		}
		fmt.Printf("Registered repositories in %s (%d total):\n", dotFile, len(repos))
		for i, r := range repos {
			fmt.Printf("  %3d. %s\n", i+1, r)
		}
		return
	}

	if folder != "" {
		scan(folder)
		return
	}

	if serveWeb {
		startServer(email, workers, port)
		return
	}

	if asJSON {
		outputJSON(email, workers)
		return
	}

	stats(email, workers)
}
