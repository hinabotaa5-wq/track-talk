package competitions

import (
	"strings"
	"testing"
	"time"
)

func TestDueSourcesCapsRequestsAndSkipsFreshImports(t *testing.T) {
	var sources []Source
	for _, code := range []string{"01", "02", "03", "04", "05", "06", "07", "08"} {
		sources = append(sources, Source{Prefecture: code, PageURL: "https://example.com/" + code})
	}
	now := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	updated := map[string]time.Time{
		"01": time.Date(2026, 10, 1, 0, 0, 1, 0, japanTime),
		"03": time.Date(2026, 9, 30, 23, 59, 59, 0, japanTime),
	}
	due := DueSources(sources, updated, now, 7)
	if len(due) != 7 {
		t.Fatalf("len %d", len(due))
	}
	if due[0].Prefecture == "01" {
		t.Fatal("fresh import should be skipped")
	}
	seen := map[string]bool{}
	for _, source := range due {
		if source.Prefecture == "01" {
			t.Fatal("01 is fresh")
		}
		seen[source.Prefecture] = true
	}
	if !seen["03"] || !seen["08"] {
		t.Fatalf("expected the stale and never-imported sources, got %+v", due)
	}
	if due[0].Prefecture != "02" {
		t.Fatalf("never imported comes before an old import, got %s", due[0].Prefecture)
	}
}

func TestDueSourcesCountsGeminiRequests(t *testing.T) {
	sources := []Source{
		{Prefecture: "11", PageURL: "https://example.com/a", ExtraURLs: []string{"https://example.com/b", "https://example.com/c"}},
		{Prefecture: "18", PageURL: "https://example.com/d", ExtraURLs: []string{"https://example.com/e"}},
		{Prefecture: "04", PageURL: "https://example.com/f", PDFLinks: []string{"a", "b"}},
		{Prefecture: "02", PageURL: "https://example.com/g"},
	}
	if got := sources[2].Requests(); got != 1 {
		t.Fatalf("linked pdfs are one request, got %d", got)
	}
	now := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	updated := map[string]time.Time{
		"11": now.Add(-30 * 24 * time.Hour),
		"18": now.Add(-29 * 24 * time.Hour),
		"04": now.Add(-28 * 24 * time.Hour),
		"02": now.Add(-27 * 24 * time.Hour),
	}
	// 18 needs 2 but only 1 is left after 11, so it waits and the one-request 04 goes instead.
	due := DueSources(sources, updated, now, 4)
	var codes []string
	for _, source := range due {
		codes = append(codes, source.Prefecture)
	}
	if strings.Join(codes, ",") != "11,04" {
		t.Fatalf("due %v", codes)
	}
	due = DueSources(sources, updated, now, 5)
	codes = nil
	for _, source := range due {
		codes = append(codes, source.Prefecture)
	}
	if strings.Join(codes, ",") != "11,18" {
		t.Fatalf("due %v", codes)
	}
}

func TestSourceURLs(t *testing.T) {
	s1 := Source{Prefecture: "40", PageURL: "https://example.com/a"}
	if urls := s1.URLs(); len(urls) != 1 || urls[0] != "https://example.com/a" {
		t.Fatalf("expected single url, got %v", urls)
	}
	s2 := Source{Prefecture: "43", PageURL: "https://example.com/a", ExtraURLs: []string{"https://example.com/b"}}
	if urls := s2.URLs(); len(urls) != 2 || urls[0] != "https://example.com/a" || urls[1] != "https://example.com/b" {
		t.Fatalf("expected two urls, got %v", urls)
	}
}

func TestImportWindowStart(t *testing.T) {
	cases := []struct {
		now  time.Time
		want time.Time
	}{
		{time.Date(2026, 10, 14, 3, 0, 0, 0, japanTime), time.Date(2026, 10, 1, 0, 0, 0, 0, japanTime)},
		{time.Date(2026, 4, 1, 0, 0, 0, 0, japanTime), time.Date(2026, 4, 1, 0, 0, 0, 0, japanTime)},
		{time.Date(2026, 9, 30, 23, 0, 0, 0, japanTime), time.Date(2026, 4, 1, 0, 0, 0, 0, japanTime)},
		{time.Date(2027, 2, 1, 0, 0, 0, 0, japanTime), time.Date(2026, 10, 1, 0, 0, 0, 0, japanTime)},
		{time.Date(2026, 3, 31, 12, 0, 0, 0, japanTime), time.Date(2025, 10, 1, 0, 0, 0, 0, japanTime)},
		// 2026-09-30 15:00 UTC is already October 1 in Japan.
		{time.Date(2026, 9, 30, 15, 0, 0, 0, time.UTC), time.Date(2026, 10, 1, 0, 0, 0, 0, japanTime)},
	}
	for _, c := range cases {
		if got := ImportWindowStart(c.now); !got.Equal(c.want) {
			t.Errorf("ImportWindowStart(%v) = %v, want %v", c.now, got, c.want)
		}
	}
}

func TestDueSourcesImportsOncePerWindowAndRetriesFailures(t *testing.T) {
	sources := []Source{
		{Prefecture: "01", PageURL: "https://example.com/01"},
		{Prefecture: "02", PageURL: "https://example.com/02"},
		{Prefecture: "03", PageURL: "https://example.com/03"},
	}
	updated := map[string]time.Time{
		"01": time.Date(2026, 10, 2, 9, 0, 0, 0, japanTime), // imported in the October window
		"02": time.Date(2026, 4, 20, 9, 0, 0, 0, japanTime), // only the April window
	}
	// 03 was never imported or failed every time: it stays due until it succeeds.
	for _, now := range []time.Time{
		time.Date(2026, 10, 3, 0, 0, 0, 0, japanTime),
		time.Date(2026, 12, 25, 0, 0, 0, 0, japanTime),
	} {
		var codes []string
		for _, source := range DueSources(sources, updated, now, 15) {
			codes = append(codes, source.Prefecture)
		}
		if strings.Join(codes, ",") != "03,02" {
			t.Fatalf("at %v due %v", now, codes)
		}
	}
	// In November nothing new opened, and 02 imported in October is no longer due.
	updated["02"] = time.Date(2026, 10, 5, 9, 0, 0, 0, japanTime)
	updated["03"] = time.Date(2026, 10, 6, 9, 0, 0, 0, japanTime)
	if due := DueSources(sources, updated, time.Date(2026, 11, 1, 0, 0, 0, 0, japanTime), 15); len(due) != 0 {
		t.Fatalf("due %v", due)
	}
	// The next spring everything is due again.
	if due := DueSources(sources, updated, time.Date(2027, 4, 1, 0, 0, 0, 0, japanTime), 15); len(due) != 3 {
		t.Fatalf("due %v", due)
	}
}
