package biztime

import (
	"testing"
	"time"
)

func TestParseBusinessDate(t *testing.T) {
	date, err := Parse("2026-07-23")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if date.String() != "2026-07-23" {
		t.Fatalf("date = %q, want 2026-07-23", date.String())
	}
}

func TestFromTimeUsesCalendarDateOfInput(t *testing.T) {
	timestamp := time.Date(2026, 7, 23, 23, 59, 0, 0, time.FixedZone("EDT", -4*60*60))
	date := FromTime(timestamp)
	if date.String() != "2026-07-23" {
		t.Fatalf("date = %q, want 2026-07-23", date.String())
	}
}
