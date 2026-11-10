package competitions

import (
	"strings"
	"testing"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/jst"
)

// Issue #460 (a): 一時失敗した県があっても、後続の未試行の県が予算内に届く。
func TestDueSourcesWithAttemptsDefersFailuresAndLargeSources(t *testing.T) {
	sources := []Source{
		{Prefecture: "14", PageURL: "https://example.com/14"},
		{Prefecture: "18", PageURL: "https://example.com/18", ExtraURLs: []string{
			"https://example.com/18-2", "https://example.com/18-3", "https://example.com/18-4",
			"https://example.com/18-5", "https://example.com/18-6", "https://example.com/18-7",
			"https://example.com/18-8", "https://example.com/18-9", "https://example.com/18-10",
			"https://example.com/18-11", "https://example.com/18-12",
		}},
		{Prefecture: "20", PageURL: "https://example.com/20"},
		{Prefecture: "21", PageURL: "https://example.com/21"},
		{Prefecture: "22", PageURL: "https://example.com/22", ExtraURLs: []string{"https://example.com/22-2"}},
		{Prefecture: "23", PageURL: "https://example.com/23", ExtraURLs: []string{
			"https://example.com/23-2", "https://example.com/23-3", "https://example.com/23-4",
		}},
		{Prefecture: "25", PageURL: "https://example.com/25"},
	}
	if got := sources[1].Requests(); got != 12 {
		t.Fatalf("fukui cost %d", got)
	}
	now := time.Date(2026, 11, 9, 6, 0, 0, 0, jst.Zone)
	lastTry := time.Date(2026, 11, 8, 7, 0, 0, 0, jst.Zone)
	attempts := map[string]Attempt{
		"14": {Outcome: AttemptTransient, AttemptedAt: lastTry, ConsecutiveFailures: 1, RequestsMade: 1},
		"20": {Outcome: AttemptTransient, AttemptedAt: lastTry, ConsecutiveFailures: 3, RequestsMade: 1},
		"21": {Outcome: AttemptSkipped, AttemptedAt: lastTry},
	}
	var codes []string
	for _, source := range DueSourcesWithAttempts(sources, nil, attempts, now, 15) {
		codes = append(codes, source.Prefecture)
	}
	// 福井(12回)は1県で予算の大半を使うので後回し。岐阜(準備中スキップ)は今の期間は終わり。
	// 失敗の少ない14が20より先だが、どちらも未試行の22・23・25のあと。
	if strings.Join(codes, ",") != "22,23,25,14,20" {
		t.Fatalf("due %v", codes)
	}
}

// Issue #460 受け入れ基準: マージ後の最初の定期実行（試行の記録なし）でも、22 以降が対象に入る。
func TestDueSourcesFirstRunIncludesLaterPrefectures(t *testing.T) {
	sources := []Source{
		{Prefecture: "14", PageURL: "https://example.com/14"},
		{Prefecture: "18", PageURL: "https://example.com/18", ExtraURLs: []string{
			"https://example.com/18-2", "https://example.com/18-3", "https://example.com/18-4",
			"https://example.com/18-5", "https://example.com/18-6", "https://example.com/18-7",
			"https://example.com/18-8", "https://example.com/18-9", "https://example.com/18-10",
			"https://example.com/18-11", "https://example.com/18-12",
		}},
		{Prefecture: "20", PageURL: "https://example.com/20"},
		{Prefecture: "21", PageURL: "https://example.com/21"},
		{Prefecture: "22", PageURL: "https://example.com/22", ExtraURLs: []string{"https://example.com/22-2"}},
		{Prefecture: "23", PageURL: "https://example.com/23", ExtraURLs: []string{
			"https://example.com/23-2", "https://example.com/23-3", "https://example.com/23-4",
		}},
		{Prefecture: "25", PageURL: "https://example.com/25"},
	}
	now := time.Date(2026, 11, 9, 6, 0, 0, 0, jst.Zone)
	var codes []string
	for _, source := range DueSourcesWithAttempts(sources, nil, nil, now, 15) {
		codes = append(codes, source.Prefecture)
	}
	found22 := false
	for _, code := range codes {
		if code == "22" {
			found22 = true
		}
		if code == "18" {
			t.Fatalf("large source should be deferred on the first run, due %v", codes)
		}
	}
	if !found22 {
		t.Fatalf("22 or later should be due on the first run, due %v", codes)
	}
}

// 部分成功の県は、残りの URL の分だけ見積もり直して取り直す。
func TestDueSourcesWithAttemptsRetriesPartialRemainders(t *testing.T) {
	sources := []Source{
		{Prefecture: "18", PageURL: "https://example.com/18", ExtraURLs: []string{
			"https://example.com/18-2", "https://example.com/18-3",
		}},
		{Prefecture: "22", PageURL: "https://example.com/22"},
	}
	now := time.Date(2026, 11, 9, 6, 0, 0, 0, jst.Zone)
	attempted := time.Date(2026, 11, 9, 5, 0, 0, 0, jst.Zone)
	updated := map[string]time.Time{
		"18": attempted, // 部分成功で rows が触れた
	}
	attempts := map[string]Attempt{
		"18": {Outcome: AttemptPartial, AttemptedAt: attempted, FailedURLs: []string{"https://example.com/18-3"}, RequestsMade: 2},
	}
	partial := attempts["18"]
	if got := sources[0].EffectiveCost(&partial, ImportWindowStart(now)); got != 1 {
		t.Fatalf("partial cost %d", got)
	}
	var codes []string
	for _, source := range DueSourcesWithAttempts(sources, updated, attempts, now, 15) {
		codes = append(codes, source.Prefecture)
	}
	// 未試行の22が先、残り1件の18があと。どちらも予算に入る。
	if strings.Join(codes, ",") != "22,18" {
		t.Fatalf("due %v", codes)
	}
}

// Issue #515: 回数制限は transient/permanent と同じく後回しになり、未試行の後になる。
func TestDueSourcesWithAttemptsDefersRateLimited(t *testing.T) {
	sources := []Source{
		{Prefecture: "26", PageURL: "https://example.com/26"},
		{Prefecture: "27", PageURL: "https://example.com/27"},
		{Prefecture: "28", PageURL: "https://example.com/28"},
	}
	now := time.Date(2026, 11, 9, 6, 0, 0, 0, jst.Zone)
	lastTry := time.Date(2026, 11, 8, 7, 0, 0, 0, jst.Zone)
	attempts := map[string]Attempt{
		"26": {Outcome: AttemptRateLimited, AttemptedAt: lastTry, ConsecutiveFailures: 1, RequestsMade: 1},
		"27": {Outcome: AttemptTransient, AttemptedAt: lastTry, ConsecutiveFailures: 1, RequestsMade: 1},
	}
	var codes []string
	for _, source := range DueSourcesWithAttempts(sources, nil, attempts, now, 15) {
		codes = append(codes, source.Prefecture)
	}
	// 未試行の28が先で、制限と一時失敗は後回し。どちらも試行として残る。
	if strings.Join(codes, ",") != "28,26,27" {
		t.Fatalf("due %v", codes)
	}
}

// 期間が変われば試行の記録は仕切り直しになる。
func TestDueSourcesWithAttemptsIgnoresPreviousWindow(t *testing.T) {
	sources := []Source{
		{Prefecture: "14", PageURL: "https://example.com/14"},
	}
	now := time.Date(2026, 10, 2, 6, 0, 0, 0, jst.Zone)
	attempts := map[string]Attempt{
		"14": {Outcome: AttemptTransient, AttemptedAt: time.Date(2026, 9, 20, 6, 0, 0, 0, jst.Zone), ConsecutiveFailures: 5},
	}
	due := DueSourcesWithAttempts(sources, nil, attempts, now, 15)
	if len(due) != 1 || due[0].Prefecture != "14" {
		t.Fatalf("previous window attempts should not deprioritize, due %+v", due)
	}
}
