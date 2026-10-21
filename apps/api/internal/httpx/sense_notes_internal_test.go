package httpx

import (
	"testing"
	"time"
)

func TestSenseNoteStreak(t *testing.T) {
	// 2026-10-16 08:00 JST
	now := time.Date(2026, 10, 15, 23, 0, 0, 0, time.UTC)
	day := func(d int) time.Time { return time.Date(2026, 10, d, 0, 0, 0, 0, time.UTC) }
	cases := []struct {
		name string
		days []time.Time
		want int
	}{
		{"none", nil, 0},
		{"today only", []time.Time{day(16)}, 1},
		{"yesterday keeps the streak", []time.Time{day(15), day(14)}, 2},
		{"gap ends the streak", []time.Time{day(16), day(15), day(13)}, 2},
		{"two days ago is broken", []time.Time{day(14), day(13)}, 0},
	}
	for _, tc := range cases {
		if got := senseNoteStreak(tc.days, now); got != tc.want {
			t.Errorf("%s: got %d want %d", tc.name, got, tc.want)
		}
	}
}

func TestSenseNoteCursorRoundTrip(t *testing.T) {
	n := senseNoteFrom([16]byte{1}, "x", nil, time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 16, 3, 4, 5, 123456000, time.UTC), time.Now())
	day, at, id, ok := decodeSenseNoteCursor(encodeSenseNoteCursor(n))
	if !ok || !at.Equal(n.CreatedAt) || *id != n.ID || day.Format(time.DateOnly) != "2026-10-15" {
		t.Fatalf("round trip failed: %v %v %v %v", day, at, id, ok)
	}
	if n.Day != "2026-10-15" {
		t.Fatalf("day = %s", n.Day)
	}
	if _, _, _, ok := decodeSenseNoteCursor("bad"); ok {
		t.Fatal("bad cursor accepted")
	}
}

func TestParseSenseNoteDay(t *testing.T) {
	now := time.Date(2026, 10, 16, 20, 0, 0, 0, time.UTC) // 日本時間では 10/17 5:00
	str := func(v string) *string { return &v }
	for _, tc := range []struct {
		name string
		in   *string
		want string
		ok   bool
	}{
		{"omitted is today in Tokyo", nil, "2026-10-17", true},
		{"blank is today", str(" "), "2026-10-17", true},
		{"today", str("2026-10-17"), "2026-10-17", true},
		{"yesterday", str("2026-10-16"), "", false},
		{"past day", str("2026-10-01"), "", false},
		{"tomorrow", str("2026-10-18"), "", false},
		{"too old", str("2025-10-01"), "", false},
		{"garbage", str("10/01"), "", false},
	} {
		day, ok := parseSenseNoteDay(tc.in, now)
		if ok != tc.ok || (ok && day.Format(time.DateOnly) != tc.want) {
			t.Errorf("%s: got %v %v", tc.name, day, ok)
		}
	}
}
