package httpx

import (
	"testing"
	"time"
)

func TestMonthStart(t *testing.T) {
	got := monthStart(time.Date(2026, 9, 20, 15, 4, 5, 0, time.FixedZone("JST", 9*3600)))
	if !got.Equal(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("month start %v", got)
	}
}
