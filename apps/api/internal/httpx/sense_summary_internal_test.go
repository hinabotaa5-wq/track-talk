package httpx

import (
	"strings"
	"testing"
	"time"
)

func TestParseSenseSummary(t *testing.T) {
	got, err := parseSenseSummary("```json\n{\"overview\":\" 流れ \",\"good\":[\"a\",\"\",\"b\",\"c\",\"d\"],\"next\":[\"" + strings.Repeat("あ", 120) + "\"]}\n```")
	if err != nil {
		t.Fatal(err)
	}
	if got.Overview != "流れ" || len(got.Good) != 3 || got.Good[1] != "b" || got.Concerns == nil || len([]rune(got.Next[0])) != maxSenseSummaryItem {
		t.Fatalf("got = %+v", got)
	}
	if _, err := parseSenseSummary(`{"overview":"","good":["a"]}`); err == nil {
		t.Fatal("empty overview accepted")
	}
	if _, err := parseSenseSummary("まとめです"); err == nil {
		t.Fatal("text accepted")
	}
}

func TestSenseSummaryNextAt(t *testing.T) {
	last := time.Date(2026, 10, 13, 3, 0, 0, 0, time.UTC)
	if next := senseSummaryNextAt(last, last.Add(6*24*time.Hour)); next == nil || !next.Equal(last.Add(7*24*time.Hour)) {
		t.Fatalf("6 days later = %v", next)
	}
	if next := senseSummaryNextAt(last, last.Add(7*24*time.Hour)); next != nil {
		t.Fatalf("7 days later = %v", next)
	}
}
