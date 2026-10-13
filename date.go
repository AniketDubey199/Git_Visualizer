package main

import (
	"time"
)

// normalizeToMidnightUTC converts any time.Time to midnight UTC using the calendar
// date (year, month, day) in the given time's location.
// By computing differences in UTC midnight space, we avoid Daylight Saving Time (DST)
// pitfalls (such as 23-hour or 25-hour days) and ensure every day is exactly 24 hours.
func normalizeToMidnightUTC(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// countDaysSinceDate computes the calendar day difference between `date` and `now` in O(1) time.
// Returns outofRange if the commit is older than maxDays or in the future.
func countDaysSinceDate(date time.Time, now time.Time, maxDays int) int {
	normDate := normalizeToMidnightUTC(date)
	normNow := normalizeToMidnightUTC(now)

	if normDate.After(normNow) {
		return 0 // Commit made today (or slight future clock skew within today)
	}

	// O(1) calendar day difference: in UTC, every calendar day is exactly 24 hours.
	diffDays := int(normNow.Sub(normDate) / (24 * time.Hour))
	if diffDays > maxDays {
		return outofRange
	}
	return diffDays
}

// calcOffset determines the column offset required to align days of the week
// such that Sunday through Saturday align properly in the terminal grid.
func calcOffset(t time.Time) int {
	switch t.Weekday() {
	case time.Sunday:
		return 7
	case time.Monday:
		return 6
	case time.Tuesday:
		return 5
	case time.Wednesday:
		return 4
	case time.Thursday:
		return 3
	case time.Friday:
		return 2
	case time.Saturday:
		return 1
	default:
		return 7
	}
}
