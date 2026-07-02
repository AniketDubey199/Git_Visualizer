package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"strings"
)

// get the dot file from the repositories , if not creates it
func getDotFilePath() string {
	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}

	dotFile := usr.HomeDir + "/.gogitlocalstats"

	return dotFile
}

// opens a file by finding its directory , if not present then creates the file

func openFile(filePath string) *os.File {
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_RDWR, 0755)
	if err != nil {
		if os.IsNotExist(err) {
			_, err := os.Create(filePath)
			if err != nil {
				panic(err)
			}
		} else {
			panic(err)
		}

	}
	return f
}

// now it pareses line from the path of the file and stores in slices of string
func parseFileToSlice(filepath string) []string {
	f := openFile(filepath)
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		if err != io.EOF {
			panic(err)
		}
	}
	return lines
}

// here we will check that the slice has value or not and return true for respectively.
func sliceContains(slice []string, value string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

// joins new elements to existing slides
func joinSlices(new []string, existing []string) []string {
	for _, i := range new {
		if !sliceContains(existing, i) {
			existing = append(existing, i)
		}
	}
	return existing
}

// overwriting the existing content in the filepath file
func dumpStringSliceToFiles(repos []string, filepath string) {
	content := strings.Join(repos, "\n")
	ioutil.WriteFile(filepath, []byte(content), 0755)
}

// slices that contains the path of repos , store then in filesystem
func addNewSliceElementToFile(filepath string, newRepo []string) {
	existingRepos := parseFileToSlice(filepath)
	repos := joinSlices(newRepo, existingRepos)

	dumpStringSliceToFiles(repos, filepath)
}

// starts the recursive search of git repositories living in folder subtree
func recursiveScanFolder(folder string) []string {
	return scanGitFolder(make([]string, 0), folder)
}

// scans a new folder for git repositories
func scan(folder string) {
	fmt.Printf("Found Folders:\n\n")
	repositories := recursiveScanFolder(folder)
	filepath := getDotFilePath()
	addNewSliceElementToFile(filepath, repositories)
	fmt.Printf("\n\nSuccessfully Added\n\n")
}

func scanGitFolder(folders []string, folder string) []string {
	folder = strings.TrimSuffix(folder, "/")

	f, err := os.Open(folder)
	if err != nil {
		log.Fatal(err)
	}
	files, err := f.Readdir(-1)
	f.Close()
	if err != nil {
		log.Fatal(err)
	}

	var path string

	for _, file := range files {
		if file.IsDir() {
			path = folder + "/" + file.Name()
			if file.Name() == ".git" {
				path = strings.TrimSuffix(path, "./git")
				fmt.Println(path)
				folders = append(folders, path)
				continue
			}
			if file.Name() == "vendor" || file.Name() == "node_modules" {
				continue
			}
			folders = scanGitFolder(folders, path)
		}
	}
	return folders
}
