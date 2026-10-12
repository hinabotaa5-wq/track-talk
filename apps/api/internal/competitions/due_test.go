package competitions

import (
	"testing"
	"time"
)

func TestDueSourcesCapsAtSevenAndSkipsFreshImports(t *testing.T) {
	var sources []Source
	for _, code := range []string{"01", "02", "03", "04", "05", "06", "07", "08"} {
		sources = append(sources, Source{Prefecture: code, PageURL: "https://example.com/" + code})
	}
	now := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	updated := map[string]time.Time{
		"01": now.Add(-24 * time.Hour),
		"03": now.Add(-8 * 24 * time.Hour),
	}
	due := DueSources(sources, updated, now)
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
		t.Fatalf("never imported comes before a week-old import, got %s", due[0].Prefecture)
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
