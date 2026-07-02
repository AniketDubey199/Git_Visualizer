package main

import (
	"fmt"
	"sort"
	"time"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

const outofRange = 99999
const daysInLastSixMonth = 183
const weeksInLastSixMonth = 93

type column []int

func stats(email string) {
	commits := processRepositories(email)
	printCommitStats(commits)
}

// gives the start time of the day
func getBeginningOfTheDay(t time.Time) time.Time {
	year, month, date := t.Date()
	startOfDay := time.Date(year, month, date, 0, 0, 0, 0, t.Location())

	return startOfDay
}

// gives the beginning of the day
func countDaysSinceDate(date time.Time) int {
	days := 0
	now := getBeginningOfTheDay(time.Now())

	for date.Before(now) {
		date = date.Add(time.Hour * 24)
		days++
		if days > daysInLastSixMonth {
			return outofRange
		}
	}
	return days
}

// from the given repo found in 'path' , gets the commit and puts them in the
// 'commits' map , returning it when completed
func fillCommits(email string, path string, commits map[int]int) map[int]int {
	repo, err := git.PlainOpen(path)
	if err != nil {
		panic(err)
	}

	ref, err := repo.Head()
	if err != nil {
		panic(err)
	}

	iterator, err := repo.Log(&git.LogOptions{From: ref.Hash()})
	if err != nil {
		panic(err)
	}

	offset := calcOffest()
	err = iterator.ForEach(func(c *object.Commit) error {
		daysalgo := countDaysSinceDate(c.Author.When) + offset

		if c.Author.Email != email {
			return nil
		}

		if daysalgo != outofRange {
			commits[daysalgo]++
		}
		return nil
	})
	if err != nil {
		panic(err)
	}

	return commits
}

// given the email give the commits of last 6 months
func processRepositories(email string) map[int]int {
	filePath := getDotFilePath()
	repos := parseFileToSlice(filePath)

	daysInMap := daysInLastSixMonth

	commits := make(map[int]int, daysInMap)
	for i := daysInMap; i > 0; i-- {
		commits[i] = 0
	}

	for _, path := range repos {
		commits = fillCommits(email, path, commits)
	}

	return commits
}

// determines and returns the amount of days missing to fill last row of the stats groph
func calcOffest() int {
	var offset int
	weekdays := time.Now().Weekday()

	switch weekdays {
	case time.Sunday:
		offset = 7
	case time.Monday:
		offset = 6
	case time.Tuesday:
		offset = 5
	case time.Wednesday:
		offset = 4
	case time.Thursday:
		offset = 3
	case time.Friday:
		offset = 2
	case time.Saturday:
		offset = 1
	}

	return offset
}

// given a cell value prints it with a different format
// based on the value amoutn , and on the 'today' flag
func printCell(val int, today bool) {
	escape := "\033[0;37;30m"
	switch {
	case val > 0 && val < 5:
		escape = "\033[1;30;47m"
	case val >= 5 && val < 10:
		escape = "\033[1;30;43m"
	case val >= 10:
		escape = "\033[1;30;42m"
	}

	if today {
		escape = "\033[1;37;45m"
	}

	if val == 0 {
		fmt.Printf(escape + " - " + "\033[0m")
		return
	}

	str := " %d "
	switch {
	case val >= 10:
		str = " %d "
	case val >= 100:
		str = "%d "
	}

	fmt.Printf(escape+str+"\033[0m", val)

}

// prints commit stats
func printCommitStats(commits map[int]int) {
	keys := sortMapIntoSlice(commits)
	cols := buildCols(keys, commits)
	printCells(cols)
}

// returns a slice of indexes of a map , ordered
func sortMapIntoSlice(m map[int]int) []int {
	var keys []int
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	return keys
}

// generates a map with rows and columns ready to be printed to screen
func buildCols(keys []int, commits map[int]int) map[int]column {
	cols := make(map[int]column)
	col := column{}

	for _, k := range keys {
		week := int(k / 7)
		dayinweek := k % 7

		if dayinweek == 0 {
			col = column{}
		}
		col = append(col, commits[k])

		if dayinweek == 6 {
			cols[week] = col
		}
	}
	return cols
}

// print cells of the graph
func printCells(cols map[int]column) {
	printMonths()

	for j := 6; j >= 0; j-- {
		for i := weeksInLastSixMonth + 1; i >= 0; i-- {
			if i == weeksInLastSixMonth+1 {
				printDayCol(j)
			}
			if col, ok := cols[i]; ok {
				if i == 0 && j == calcOffest()-1 {
					printCell(col[j], true)
					continue
				} else {
					if len(col) > j {
						printCell(col[j], false)
						continue
					}
				}
			}
			printCell(0, false)
		}
		fmt.Printf("\n")
	}
}

// prints the month name in first line
func printMonths() {
	week := getBeginningOfTheDay(time.Now()).Add(-(daysInLastSixMonth * time.Hour * 24))
	month := week.Month()
	fmt.Printf("	")
	for {
		if week.Month() != month {
			fmt.Printf("%s ", week.Month().String()[:3])
		} else {
			fmt.Printf("	")
		}
		week = week.Add(7 * time.Hour * 24)
		if week.After(time.Now()) {
			break
		}
	}

	fmt.Printf("/n")
}

// given the day number print the day name
func printDayCol(day int) {
	out := "	"
	switch day {
	case 1:
		out = " Mon "
	case 3:
		out = " Wed "
	case 5:
		out = " Fri "
	}

	fmt.Printf(out)
}
