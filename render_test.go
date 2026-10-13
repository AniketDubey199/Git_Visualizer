package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// captureOutput captures stdout during a function run.
func captureOutput(f func()) string {
	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestPrintCellFormatting(t *testing.T) {
	tests := []struct {
		name     string
		val      int
		today    bool
		contains string
	}{
		{"zero commits", 0, false, "-"},
		{"single digit", 4, false, "4"},
		{"double digit", 42, false, "42"},
		{"three digits", 128, false, "128"},
		{"today indicator", 5, true, "5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := captureOutput(func() {
				printCell(tt.val, tt.today)
			})
			if !strings.Contains(out, tt.contains) {
				t.Errorf("Expected output to contain %q, got %q", tt.contains, out)
			}
		})
	}
}

func TestPrintDayCol(t *testing.T) {
	for d := 0; d <= 6; d++ {
		out := captureOutput(func() {
			printDayCol(d)
		})
		if len(strings.TrimSpace(out)) == 0 {
			t.Errorf("Expected non-empty day label for day %d", d)
		}
	}
}
