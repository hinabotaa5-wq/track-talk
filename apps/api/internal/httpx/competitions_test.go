package httpx

import (
	"testing"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/jst"
)

func TestParsePrefectureCode(t *testing.T) {
	code, ok := parsePrefectureCode("46")
	if !ok || code != "46" {
		t.Fatalf("46: got %q %v", code, ok)
	}
	code, ok = parsePrefectureCode("01")
	if !ok || code != "01" {
		t.Fatalf("01: got %q %v", code, ok)
	}
	code, ok = parsePrefectureCode("1")
	if !ok || code != "01" {
		t.Fatalf("1: got %q %v", code, ok)
	}
	code, ok = parsePrefectureCode("00")
	if !ok || code != "00" {
		t.Fatalf("00: got %q %v", code, ok)
	}
	code, ok = parsePrefectureCode("0")
	if !ok || code != "00" {
		t.Fatalf("0: got %q %v", code, ok)
	}
	if _, ok := parsePrefectureCode(""); ok {
		t.Fatal("empty should fail")
	}
	if _, ok := parsePrefectureCode("48"); ok {
		t.Fatal("48 should fail")
	}
}

func TestSeasonMonthRange(t *testing.T) {
	october := time.Date(2026, time.October, 11, 20, 0, 0, 0, time.UTC)
	if got := currentSeasonYear(october); got != 2026 {
		t.Fatalf("october season: got %d", got)
	}
	start, end := seasonMonthRange(2026, 10)
	if !start.Equal(time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)) || !end.Equal(time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("october range: %s %s", start, end)
	}

	january := time.Date(2027, time.January, 15, 0, 0, 0, 0, jst.Zone)
	if got := currentSeasonYear(january); got != 2026 {
		t.Fatalf("january season: got %d", got)
	}
	start, end = seasonMonthRange(2026, 1)
	if !start.Equal(time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC)) || !end.Equal(time.Date(2027, time.February, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("january range: %s %s", start, end)
	}
}

func TestSeasonRange(t *testing.T) {
	start, end := seasonRange(2026)
	if !start.Equal(time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)) || !end.Equal(time.Date(2027, time.April, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("season range: %s %s", start, end)
	}
}
