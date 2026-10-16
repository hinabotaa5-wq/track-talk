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
	n := senseNoteFrom([16]byte{1}, "x", nil, time.Date(2026, 10, 16, 3, 4, 5, 123456000, time.UTC), time.Now())
	at, id, ok := decodeSenseNoteCursor(encodeSenseNoteCursor(n))
	if !ok || !at.Equal(n.CreatedAt) || *id != n.ID {
		t.Fatalf("round trip failed: %v %v %v", at, id, ok)
	}
	if n.Day != "2026-10-16" {
		t.Fatalf("day = %s", n.Day)
	}
	if _, _, ok := decodeSenseNoteCursor("bad"); ok {
		t.Fatal("bad cursor accepted")
	}
}
