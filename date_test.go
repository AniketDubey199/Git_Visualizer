package main

import (
	"testing"
	"time"
)

func TestCountDaysSinceDate_O1_Vs_Loop(t *testing.T) {
	now := time.Date(2026, 10, 13, 14, 0, 0, 0, time.UTC)
	maxDays := 183

	// Loop baseline
	loopCountDays := func(date time.Time) int {
		days := 0
		normNow := normalizeToMidnightUTC(now)
		normDate := normalizeToMidnightUTC(date)
		for normDate.Before(normNow) {
			normDate = normDate.AddDate(0, 0, 1)
			days++
			if days > maxDays {
				return outofRange
			}
		}
		return days
	}

	for d := 0; d <= maxDays; d++ {
		target := now.AddDate(0, 0, -d)
		gotO1 := countDaysSinceDate(target, now, maxDays)
		gotLoop := loopCountDays(target)
		if gotO1 != gotLoop {
			t.Fatalf("Day %d mismatch: got O(1)=%d, loop=%d", d, gotO1, gotLoop)
		}
	}

	// Beyond maxDays
	tooOld := now.AddDate(0, 0, -200)
	if countDaysSinceDate(tooOld, now, maxDays) != outofRange {
		t.Fatalf("Expected outofRange for 200 days ago")
	}
}

func TestDSTTransitions(t *testing.T) {
	// America/New_York DST Spring Forward: 2024-03-10 (23-hour day)
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("America/New_York tz data not available")
	}

	beforeDST := time.Date(2024, 3, 9, 15, 0, 0, 0, loc)
	duringDST := time.Date(2024, 3, 10, 15, 0, 0, 0, loc)
	afterDST := time.Date(2024, 3, 11, 15, 0, 0, 0, loc)

	// In calendar days, afterDST is 2 days after beforeDST, and 1 day after duringDST
	days1 := countDaysSinceDate(beforeDST, afterDST, 10)
	if days1 != 2 {
		t.Errorf("Expected 2 calendar days across DST jump, got %d", days1)
	}

	days2 := countDaysSinceDate(duringDST, afterDST, 10)
	if days2 != 1 {
		t.Errorf("Expected 1 calendar day across DST jump, got %d", days2)
	}
}

func TestCalcOffset(t *testing.T) {
	// Sunday -> 7
	sun := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)
	if calcOffset(sun) != 7 {
		t.Errorf("Sunday offset expected 7, got %d", calcOffset(sun))
	}

	// Saturday -> 1
	sat := time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC)
	if calcOffset(sat) != 1 {
		t.Errorf("Saturday offset expected 1, got %d", calcOffset(sat))
	}
}
