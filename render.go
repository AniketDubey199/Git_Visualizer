package main

import (
	"fmt"
	"sort"
	"time"
)

type column []int

// printCell prints a single heatmap cell with ANSI styling based on commit count and 'today' flag.
// Fixed switch evaluation order: val >= 100 is checked BEFORE val >= 10 to ensure
// numbers with 3+ digits receive the correct padding and don't get matched by val >= 10 first.
func printCell(val int, today bool) {
	// Color styling (ANSI 256 / 16 color shades)
	escape := "\033[0;37;30m"
	switch {
	case val > 0 && val < 5:
		escape = "\033[1;30;47m" // light gray
	case val >= 5 && val < 10:
		escape = "\033[1;30;43m" // yellow
	case val >= 10:
		escape = "\033[1;30;42m" // green
	}

	if today {
		escape = "\033[1;37;45m" // magenta highlight for today
	}

	if val == 0 {
		fmt.Print(escape + "  - " + "\033[0m")
		return
	}

	// Width formatting: check most specific (>=100) first!
	str := "  %d "
	switch {
	case val >= 100:
		str = "%d "
	case val >= 10:
		str = " %d "
	}

	fmt.Printf("%s"+str+"\033[0m", escape, val)
}

// printCommitStats renders the terminal heatmap.
func printCommitStats(commits map[int]int) {
	keys := sortMapIntoSlice(commits)
	cols := buildCols(keys, commits)
	printCells(cols)
}

// sortMapIntoSlice returns sorted keys of a map.
func sortMapIntoSlice(m map[int]int) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}

// buildCols constructs a map of columns from days.
func buildCols(keys []int, commits map[int]int) map[int]column {
	cols := make(map[int]column)
	col := column{}

	for _, k := range keys {
		week := (k - 1) / 7
		dayinweek := (k - 1) % 7

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

// printCells renders the 7 rows of days across columns.
func printCells(cols map[int]column) {
	printMonths()
	for j := 6; j >= 0; j-- {
		for i := weeksInLastSixMonths + 1; i >= 0; i-- {
			if i == weeksInLastSixMonths+1 {
				printDayCol(j)
			}
			if col, ok := cols[i]; ok {
				if i == 0 && j == calcOffset(time.Now())-1 {
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
		fmt.Print("\n")
	}
}

// printMonths prints the header row containing month abbreviations.
func printMonths() {
	week := normalizeToMidnightUTC(time.Now()).AddDate(0, 0, -daysInLastSixMonths)
	month := week.Month()
	fmt.Print("       ")
	for {
		if week.Month() != month {
			fmt.Printf("%s ", week.Month().String()[:3])
			month = week.Month()
		} else {
			fmt.Print("    ")
		}

		week = week.AddDate(0, 0, 7) // AddDate respects calendar weeks without DST hour drift
		if week.After(time.Now()) {
			break
		}
	}
	fmt.Print("\n")
}

// printDayCol outputs the day label for row j.
// Uses fmt.Print to ensure safe constant formatting that passes go vet.
func printDayCol(day int) {
	out := "       "
	switch day {
	case 6:
		out = "  Sun  "
	case 5:
		out = "  Mon  "
	case 4:
		out = "  Tue  "
	case 3:
		out = "  Wed  "
	case 2:
		out = "  Thu  "
	case 1:
		out = "  Fri  "
	case 0:
		out = "  Sat  "
	}

	fmt.Print(out)
}
