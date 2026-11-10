package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/competitions"
)

func partialSchedulePage(meet string) string {
	return `<!doctype html><html><body>
<h2>2026年競技日程</h2>
<table>
<thead><tr><th>日</th><th>競技会</th><th>会場</th></tr></thead>
<tbody>
<tr><td>4/18</td><td>` + meet + `</td><td>テスト</td></tr>
</tbody>
</table>
</body></html>`
}

func partialGeminiServer(t *testing.T, failMarker string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body strings.Builder
		_, _ = body.WriteString(r.URL.Path)
		buf := make([]byte, 1<<20)
		n, _ := r.Body.Read(buf)
		payload := string(buf[:n])
		if failMarker != "" && strings.Contains(payload, failMarker) {
			// 再試行しない失敗にして、テストを速く終わらせる。
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"candidates":[{"content":{"parts":[{"text":%q}]}}]}`,
			`{"meets":[{"name":"GOOD記録会","starts_on":"2026-04-18","ends_on":"","location":"テスト","guidelines_url":"","district_url":""}]}`)
	}))
}

// Issue #460 (b): 複数 URL の県は、取得できない URL があっても成功分を取り込む。
func TestReadMeetsKeepsSuccessfulURLsWhenFetchFails(t *testing.T) {
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(partialSchedulePage("GOOD記録会")))
	}))
	defer good.Close()
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer bad.Close()
	gemini := partialGeminiServer(t, "")
	defer gemini.Close()

	source := competitions.Source{Prefecture: "99", PageURL: good.URL, ExtraURLs: []string{bad.URL}}
	out, err := readMeets(context.Background(), competitions.Client{Key: "test", Endpoint: gemini.URL}, source, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.meets) != 1 || out.meets[0].Name != "GOOD記録会" {
		t.Fatalf("meets %+v", out.meets)
	}
	if len(out.succeeded) != 1 || len(out.failed) != 1 || len(out.skipped) != 0 {
		t.Fatalf("outcome %+v", out)
	}
	if out.failed[0] != bad.URL {
		t.Fatalf("failed %v", out.failed)
	}
	if out.geminiCalls != 1 {
		t.Fatalf("gemini calls %d", out.geminiCalls)
	}
}

// Issue #460 (b): Gemini の読み取りに失敗した URL だけを切り離し、ほかの URL は取り込む。
func TestReadMeetsKeepsSuccessfulURLsWhenExtractFails(t *testing.T) {
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(partialSchedulePage("GOOD記録会")))
	}))
	defer good.Close()
	badPage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(partialSchedulePage("BAD記録会")))
	}))
	defer badPage.Close()
	gemini := partialGeminiServer(t, "BAD記録会")
	defer gemini.Close()

	source := competitions.Source{Prefecture: "99", PageURL: good.URL, ExtraURLs: []string{badPage.URL}}
	out, err := readMeets(context.Background(), competitions.Client{Key: "test", Endpoint: gemini.URL}, source, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.meets) != 1 || out.meets[0].Name != "GOOD記録会" {
		t.Fatalf("meets %+v", out.meets)
	}
	if len(out.succeeded) != 1 || len(out.failed) != 1 {
		t.Fatalf("outcome %+v", out)
	}
	if out.failed[0] != badPage.URL {
		t.Fatalf("failed %v", out.failed)
	}
	if out.geminiCalls != 2 {
		t.Fatalf("gemini calls %d", out.geminiCalls)
	}
}

// Issue #460: 表がなくて Gemini を呼ばない URL は飛ばし、残り見積もりに数えない。
func TestReadMeetsSkipsURLsWithoutScheduleTable(t *testing.T) {
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(partialSchedulePage("GOOD記録会")))
	}))
	defer good.Close()
	empty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body><h2>2026年競技日程</h2><p>まだ公開されていません</p></body></html>`))
	}))
	defer empty.Close()
	gemini := partialGeminiServer(t, "")
	defer gemini.Close()

	source := competitions.Source{Prefecture: "99", PageURL: good.URL, ExtraURLs: []string{empty.URL}}
	out, err := readMeets(context.Background(), competitions.Client{Key: "test", Endpoint: gemini.URL}, source, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.meets) != 1 {
		t.Fatalf("meets %+v", out.meets)
	}
	if len(out.succeeded) != 1 || len(out.skipped) != 1 || len(out.failed) != 0 {
		t.Fatalf("outcome %+v", out)
	}
	if out.geminiCalls != 1 {
		t.Fatalf("gemini calls %d", out.geminiCalls)
	}
}

// Issue #460: 残りの読み直しは失敗した URL だけを読み、成功分を取り込む。
func TestReadMeetsRetriesOnlyFailedURLs(t *testing.T) {
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(partialSchedulePage("GOOD記録会")))
	}))
	defer good.Close()
	badPage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(partialSchedulePage("BAD記録会")))
	}))
	defer badPage.Close()
	failBAD := true
	gemini := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1<<20)
		n, _ := r.Body.Read(buf)
		payload := string(buf[:n])
		name := "GOOD記録会"
		if strings.Contains(payload, "BAD記録会") {
			if failBAD {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			name = "BAD記録会"
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"candidates":[{"content":{"parts":[{"text":%q}]}}]}`,
			`{"meets":[{"name":"`+name+`","starts_on":"2026-04-18","ends_on":"","location":"テスト","guidelines_url":"","district_url":""}]}`)
	}))
	defer gemini.Close()

	source := competitions.Source{Prefecture: "99", PageURL: good.URL, ExtraURLs: []string{badPage.URL}}
	client := competitions.Client{Key: "test", Endpoint: gemini.URL}
	first, err := readMeets(context.Background(), client, source, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.failed) != 1 || len(first.meets) != 1 {
		t.Fatalf("first %+v", first)
	}
	failBAD = false
	second, err := readMeets(context.Background(), client, source, first.failed)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.meets) != 1 || second.meets[0].Name != "BAD記録会" {
		t.Fatalf("retry meets %+v", second.meets)
	}
	if len(second.succeeded) != 1 || len(second.failed) != 0 {
		t.Fatalf("retry %+v", second)
	}
	if second.geminiCalls != 1 {
		t.Fatalf("retry gemini calls %d", second.geminiCalls)
	}
}

// Issue #460 受け入れ基準: 末尾サマリに県ごとの結果が一覧できる。
// Issue #515: サマリに論理呼び出し数とリトライ込み実呼び出し数の両方を出す。
func TestImportResultsSummary(t *testing.T) {
	var r importResults
	r.recordOutcome("14", competitions.AttemptTransient, fmt.Errorf("gemini temporary failure: request timeout"), 3, 4)
	r.recordOutcome("18", competitions.AttemptPartial, nil, 9, 10)
	r.recordOutcome("21", competitions.AttemptSkipped, fmt.Errorf("取り込み元をスキップ"), 0, 0)
	summary := r.summary(3, 15)
	for _, want := range []string{"targets=3", "success=1", "partial=1", "transient=1", "skipped=1", "rate_limited=0", "gemini_calls=12/15", "http_calls=14", "14:", "18:", "21:"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("summary missing %q:\n%s", want, summary)
		}
	}
	if r.failed() {
		t.Fatal("partial success should not fail the job")
	}
}

// Issue #515: 回数制限は permanent ではなく rate_limited に分け、失敗として残る。
func TestImportResultsRateLimited(t *testing.T) {
	var r importResults
	r.recordOutcome("25", competitions.AttemptRateLimited, competitions.ErrRateLimited, 1, 1)
	summary := r.summary(1, 15)
	for _, want := range []string{"rate_limited=1", "rate_limited=yes", "25: rate_limited"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("summary missing %q:\n%s", want, summary)
		}
	}
	if strings.Contains(summary, "permanent_failure=yes") {
		t.Fatalf("rate limit should not be reported as permanent:\n%s", summary)
	}
	if !r.failed() {
		t.Fatal("rate limit should fail the job")
	}
}

// Issue #515: 1回の実行が全体上限（コード20分）を超えない時間配分になっていること。
func TestTimeBudgetsFitOverall(t *testing.T) {
	if SourceTimeout >= OverallTimeout {
		t.Fatalf("source %v must be shorter than overall %v", SourceTimeout, OverallTimeout)
	}
	// 1回の論理呼び出しの worst case（HTTP上限×試行回数＋待ち）が県の上限に収まること。
	worstCall := time.Duration(competitions.GeminiMaxAttempts)*competitions.GeminiHTTPTimeout + competitions.GeminiRetryWait
	if worstCall >= SourceTimeout {
		t.Fatalf("worst gemini call %v must fit in source timeout %v", worstCall, SourceTimeout)
	}
	// 再試行なしなら論理数の上限が実数の上限に収まること（平常時は枠を超えない）。
	if competitions.DailyRequestLimit > competitions.MaxRealCallsPerDay {
		t.Fatalf("logical limit %d must fit in real cap %d", competitions.DailyRequestLimit, competitions.MaxRealCallsPerDay)
	}
	// リトライ嵐でも実数の上限が論理数×試行回数で頭打ちになる設計であること。
	if competitions.MaxRealCallsPerDay > competitions.DailyRequestLimit*competitions.GeminiMaxAttempts {
		t.Fatalf("real cap %d exceeds worst-case logical amplification %d", competitions.MaxRealCallsPerDay, competitions.DailyRequestLimit*competitions.GeminiMaxAttempts)
	}
}
