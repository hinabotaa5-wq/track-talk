package httpx

import (
	"strings"
	"testing"
	"time"
)

func TestScrubFreeText(t *testing.T) {
	got := scrubFreeText("連絡は runner@example.com か @haruto_run、電話 090-1234-5678、https://example.com/a?b=1 まで。はるとが痛めた", "はると")
	for _, leaked := range []string{"runner@example.com", "haruto_run", "090-1234-5678", "example.com/a", "はると"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("%q leaked in %q", leaked, got)
		}
	}
	for _, kept := range []string{"[メール]", "[ID]", "[番号]", "[リンク]", "[名前]", "痛めた"} {
		if !strings.Contains(got, kept) {
			t.Fatalf("%q missing in %q", kept, got)
		}
	}
	if got := scrubFreeText("膝の外側が痛い 3週間", "a"); got != "膝の外側が痛い 3週間" {
		t.Fatalf("plain text changed: %q", got)
	}
}

func TestMonthStart(t *testing.T) {
	got := monthStart(time.Date(2026, 9, 20, 15, 4, 5, 0, time.FixedZone("JST", 9*3600)))
	if !got.Equal(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("month start %v", got)
	}
}
