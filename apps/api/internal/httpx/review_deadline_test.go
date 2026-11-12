package httpx

import (
	"testing"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/jst"
)

func jstAt(day, hour, minute int) time.Time {
	return time.Date(2026, 11, day, hour, minute, 0, 0, jst.Zone)
}

// 返却期限は申込みから24時間。夜間も止まらず、日付や時刻に関係なく同じ。
func TestReviewDueAtIs24Hours(t *testing.T) {
	for _, start := range []time.Time{jstAt(2, 2, 0), jstAt(2, 12, 0), jstAt(2, 22, 45), jstAt(2, 23, 30), jstAt(30, 23, 59)} {
		if got := reviewDueAt(start); !got.Equal(start.Add(24 * time.Hour)) {
			t.Fatalf("%v → %v", start, got)
		}
	}
}

func TestReviewDeadlineText(t *testing.T) {
	if got := reviewDeadlineText(jstAt(3, 7, 30).UTC()); got != "11月3日 7:30" {
		t.Fatal(got)
	}
}
