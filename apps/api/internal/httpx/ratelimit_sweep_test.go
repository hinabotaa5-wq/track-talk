package httpx

import (
	"testing"
	"time"
)

func TestRateLimiterSweepDropsIdleIPs(t *testing.T) {
	l := NewRateLimiter()
	now := time.Now()
	l.writes["198.51.100.1"] = []time.Time{now.Add(-2 * time.Minute)}
	l.writes["198.51.100.2"] = []time.Time{now.Add(-10 * time.Second)}
	l.reports["198.51.100.1"] = []time.Time{now.Add(-2 * time.Hour)}
	l.inquiries["198.51.100.3"] = []time.Time{now.Add(-30 * time.Minute)}
	l.sweep(now)
	if _, ok := l.writes["198.51.100.1"]; ok {
		t.Fatal("idle write entry kept")
	}
	if len(l.writes["198.51.100.2"]) != 1 || len(l.reports) != 0 || len(l.inquiries["198.51.100.3"]) != 1 {
		t.Fatalf("writes %v reports %v inquiries %v", l.writes, l.reports, l.inquiries)
	}
}
