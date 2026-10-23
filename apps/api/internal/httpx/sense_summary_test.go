package httpx_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/hinata/track-talk/apps/api/internal/jst"
	"github.com/jackc/pgx/v5/pgxpool"
)

type senseSummaryResponse struct {
	PendingNotes int `json:"pending_notes"`
	MinNotes     int `json:"min_notes"`
	Summary      *struct {
		FromDay   string `json:"from_day"`
		ToDay     string `json:"to_day"`
		NoteCount int    `json:"note_count"`
		Summary   struct {
			Overview string   `json:"overview"`
			Good     []string `json:"good"`
			Concerns []string `json:"concerns"`
			Next     []string `json:"next"`
		} `json:"summary"`
	} `json:"summary"`
}

// fakeGemini は、Gemini の代わりに reply を返し、届いたプロンプトを残す。
type fakeGemini struct {
	mu    sync.Mutex
	reply string
	// status が 0 でなければ、その HTTP ステータスで返す(無料枠の上限の 429 など)。
	status  int
	prompts []string
}

func (f *fakeGemini) serve(t *testing.T) string {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body struct {
			Contents []struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"contents"`
		}
		_ = json.Unmarshal(raw, &body)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.prompts = append(f.prompts, body.Contents[0].Parts[0].Text)
		if f.status != 0 {
			w.WriteHeader(f.status)
			_, _ = w.Write([]byte(`{"error":{"status":"RESOURCE_EXHAUSTED"}}`))
			return
		}
		payload, _ := json.Marshal(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]string{"text": f.reply}}}}}})
		_, _ = w.Write(payload)
	}))
	t.Cleanup(server.Close)
	return server.URL
}

// writeSenseNotes は、感覚メモを count 個、今日から daysBack 日前のようにさかのぼって1日1つずつ書く。
// 1つ目は種目つき、2つ目は「右のハムが張る」。
// API で書けるのは今日の分だけなので、今日の分は API で、それより前の日の分は DB に直接入れる。
func writeSenseNotes(t *testing.T, h http.Handler, tok string, daysBack, count int) {
	t.Helper()
	if rec := doJSON(t, h, http.MethodGet, "/v1/me", tok, nil); rec.Code != http.StatusOK {
		t.Fatalf("profile = %d", rec.Code)
	}
	author := tokenSubject(t, tok)
	for i := 0; i < count; i++ {
		back := daysBack + i
		day := time.Now().In(jst.Zone).AddDate(0, 0, -back).Format(time.DateOnly)
		body, event := fmt.Sprintf("感覚%d", i+1), any(nil)
		switch i {
		case 0:
			body, event = "接地で地面を押せた", "100m"
		case 1:
			body = "右のハムが張る"
		}
		if back == 0 {
			if rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes", tok, map[string]any{"body": body, "event": event}); rec.Code != http.StatusCreated {
				t.Fatalf("create note %d = %d", i+1, rec.Code)
			}
			continue
		}
		execSQLArgs(t, `INSERT INTO sense_notes (id, author_id, body, event, day) VALUES (gen_random_uuid(), $1, $2, $3, $4::date)`, author, body, event, day)
	}
}

// tokenSubject は、テスト用のトークンの持ち主(sub)を取り出す。
func tokenSubject(t *testing.T, tok string) uuid.UUID {
	t.Helper()
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatalf("token = %q", tok)
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	var claims struct {
		Sub string `json:"sub"`
	}
	if err := json.Unmarshal(raw, &claims); err != nil {
		t.Fatal(err)
	}
	return uuid.MustParse(claims.Sub)
}

func TestSenseSummaryIsHiddenUntilLaunch(t *testing.T) {
	h := setup(t)
	me := token(t, uuid.New())
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		if rec := doJSON(t, h, method, "/v1/me/sense-notes/summary", me, nil); rec.Code != http.StatusNotFound {
			t.Fatalf("%s while hidden = %d", method, rec.Code)
		}
	}
}

func TestSenseSummaryHistoryIsHiddenUntilLaunch(t *testing.T) {
	h := setup(t)
	if rec := doJSON(t, h, http.MethodGet, "/v1/me/sense-notes/summaries", token(t, uuid.New()), nil); rec.Code != http.StatusNotFound {
		t.Fatalf("history while hidden = %d", rec.Code)
	}
}

func TestSenseSummaryNeedsSixNotes(t *testing.T) {
	gemini := &fakeGemini{reply: `{"overview":"接地の感覚がつかめてきた1週間でした。","good":["地面を真下に押せた"],"concerns":["右のハムに張りがある"],"next":["腕振りを小さく速く"]}`}
	url := gemini.serve(t)
	h := setupConfig(t, func(c *config.Config) {
		c.SenseSummaryEnabled = true
		c.GeminiAPIKey = "test"
		c.GeminiEndpoint = url
	})
	me := token(t, uuid.New())
	other := token(t, uuid.New())

	var got senseSummaryResponse
	rec := doJSON(t, h, http.MethodGet, "/v1/me/sense-notes/summary", me, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if rec.Code != http.StatusOK || got.Summary != nil {
		t.Fatalf("empty get = %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes/summary", me, nil); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("no notes = %d", rec.Code)
	}
	if len(gemini.prompts) != 0 {
		t.Fatal("gemini was called without notes")
	}

	// 6個たまるまでは、まとめられない。あと何個かは、取得で分かる。
	writeSenseNotes(t, h, me, 0, 5)
	rec = doJSON(t, h, http.MethodGet, "/v1/me/sense-notes/summary", me, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.PendingNotes != 5 || got.MinNotes != 6 {
		t.Fatalf("progress = %s", rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/me/sense-notes/summary", me, nil)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "あと1個") {
		t.Fatalf("5 notes = %d %s", rec.Code, rec.Body.String())
	}
	if len(gemini.prompts) != 0 {
		t.Fatal("gemini was called with too few notes")
	}
	writeSenseNotes(t, h, other, 0, 6)
	execSQLArgs(t, `INSERT INTO sense_notes (id, author_id, body, day) VALUES (gen_random_uuid(), $1, '6個目', $2::date)`,
		tokenSubject(t, me), time.Now().In(jst.Zone).AddDate(0, 0, -5).Format(time.DateOnly))

	rec = doJSON(t, h, http.MethodPost, "/v1/me/sense-notes/summary", me, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create summary = %d %s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Summary == nil || got.Summary.NoteCount != 6 || got.Summary.Summary.Overview == "" || len(got.Summary.Summary.Good) != 1 {
		t.Fatalf("summary = %s", rec.Body.String())
	}
	if got.Summary.FromDay > got.Summary.ToDay {
		t.Fatalf("range = %s..%s", got.Summary.FromDay, got.Summary.ToDay)
	}
	prompt := gemini.prompts[0]
	if !strings.Contains(prompt, "[100m] 接地で地面を押せた") || !strings.Contains(prompt, "[種目なし] 右のハムが張る") || strings.Count(prompt, "[種目なし]") != 5 {
		t.Fatalf("prompt = %s", prompt)
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/me/sense-notes/summary", me, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Summary == nil || got.Summary.Summary.Concerns[0] != "右のハムに張りがある" {
		t.Fatalf("get = %s", rec.Body.String())
	}
	// 他の人には見えない。
	rec = doJSON(t, h, http.MethodGet, "/v1/me/sense-notes/summary", other, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Summary != nil {
		t.Fatalf("other sees = %s", rec.Body.String())
	}

	// まとめたあとに書いたメモだけを数える。
	rec = doJSON(t, h, http.MethodGet, "/v1/me/sense-notes/summary", me, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.PendingNotes != 0 {
		t.Fatalf("pending after summary = %s", rec.Body.String())
	}
	writeSenseNotes(t, h, me, 10, 1)
	rec = doJSON(t, h, http.MethodGet, "/v1/me/sense-notes/summary", me, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.PendingNotes != 1 || got.Summary == nil {
		t.Fatalf("pending after one more note = %s", rec.Body.String())
	}
	// 作った直後は、まとめていない感覚が足りないので作れない。時間の制限はないので、6個書けばすぐ作れる。
	if rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes/summary", me, nil); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("second summary with one new note = %d", rec.Code)
	}
	writeSenseNotes(t, h, me, 11, 5)
	rec = doJSON(t, h, http.MethodPost, "/v1/me/sense-notes/summary", me, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if rec.Code != http.StatusCreated || got.Summary == nil || got.Summary.NoteCount != 6 {
		t.Fatalf("second summary right after the first = %d %s", rec.Code, rec.Body.String())
	}
	// 他の人は別に数える。壊れた答えは 502 で、まとめを作ったことにならない。
	gemini.reply = "not json"
	if rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes/summary", other, nil); rec.Code != http.StatusBadGateway {
		t.Fatalf("broken reply = %d", rec.Code)
	}
	// 無料枠の上限に当たったときは 503 で、時間をおけば作れることを伝える。まとめを作ったことにはならない。
	gemini.status = http.StatusTooManyRequests
	if rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes/summary", other, nil); rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "summary_busy") {
		t.Fatalf("rate limited = %d %s", rec.Code, rec.Body.String())
	}
	gemini.status = 0
	// 配列で包まれた答えでも作れる。
	gemini.reply = `[{"overview":"ok","good":[],"concerns":[],"next":[]}]`
	if rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes/summary", other, nil); rec.Code != http.StatusCreated {
		t.Fatalf("other summary after failure = %d", rec.Code)
	}
}

// 同時に頼まれても、まとめは1回しか作らず、AI も1回しか呼ばない。
func TestSenseSummaryIsMadeOnceUnderConcurrency(t *testing.T) {
	ai := &fakeGemini{reply: `{"overview":"ok","good":[],"concerns":[],"next":[]}`}
	url := ai.serve(t)
	h := setupConfig(t, func(c *config.Config) {
		c.SenseSummaryEnabled = true
		c.GeminiAPIKey = "test"
		c.GeminiEndpoint = url
	})
	me := token(t, uuid.New())
	writeSenseNotes(t, h, me, 0, 6)
	var wg sync.WaitGroup
	var created atomic.Int32
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/v1/me/sense-notes/summary", nil)
			req.Header.Set("Authorization", "Bearer "+me)
			req.RemoteAddr = fmt.Sprintf("203.0.113.%d:1234", i+1)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code == http.StatusCreated {
				created.Add(1)
			}
		}(i)
	}
	wg.Wait()
	ai.mu.Lock()
	calls := len(ai.prompts)
	ai.mu.Unlock()
	if created.Load() != 1 || calls != 1 {
		t.Fatalf("created %d summaries with %d AI calls, want 1 and 1", created.Load(), calls)
	}
}

// 古いメモでも、6個たまっていればまとめられる。
func TestSenseSummaryCountsOldNotes(t *testing.T) {
	ai := &fakeGemini{reply: `{"overview":"ok","good":[],"concerns":[],"next":[]}`}
	url := ai.serve(t)
	h := setupConfig(t, func(c *config.Config) {
		c.SenseSummaryEnabled = true
		c.GeminiAPIKey = "test"
		c.GeminiEndpoint = url
	})
	id := uuid.New()
	me := token(t, id)
	writeSenseNotes(t, h, me, 0, 6)
	pool, err := pgxpool.New(context.Background(), testDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(context.Background(), `UPDATE sense_notes SET created_at = now() - interval '40 days' WHERE author_id = $1`, id); err != nil {
		t.Fatal(err)
	}

	var got senseSummaryResponse
	rec := doJSON(t, h, http.MethodGet, "/v1/me/sense-notes/summary", me, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.PendingNotes != 6 {
		t.Fatalf("old notes are not counted = %s", rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/me/sense-notes/summary", me, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if rec.Code != http.StatusCreated || got.Summary == nil || got.Summary.NoteCount != 6 || got.Summary.FromDay >= got.Summary.ToDay {
		t.Fatalf("summary of old notes = %d %s", rec.Code, rec.Body.String())
	}
}

// まとめは1日に3回まで。日付をさかのぼって書けば6個はすぐにたまるので、上限が無いと1人で
// Gemini を何度でも呼べて、無料枠(大会の取り込みと同じ鍵)を使い切ってしまう。
func TestSenseSummaryDailyLimit(t *testing.T) {
	ai := &fakeGemini{reply: `{"overview":"ok","good":[],"concerns":[],"next":[]}`}
	url := ai.serve(t)
	h := setupConfig(t, func(c *config.Config) {
		c.SenseSummaryEnabled = true
		c.GeminiAPIKey = "test"
		c.GeminiEndpoint = url
	})
	me := token(t, uuid.New())
	for i := 0; i < 3; i++ {
		writeSenseNotes(t, h, me, 6*i, 6)
		if rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes/summary", me, nil); rec.Code != http.StatusCreated {
			t.Fatalf("summary %d = %d %s", i+1, rec.Code, rec.Body.String())
		}
	}
	writeSenseNotes(t, h, me, 18, 6)
	rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes/summary", me, nil)
	if rec.Code != http.StatusTooManyRequests || !strings.Contains(rec.Body.String(), "1日に3回") {
		t.Fatalf("4th summary = %d %s", rec.Code, rec.Body.String())
	}
	ai.mu.Lock()
	calls := len(ai.prompts)
	ai.mu.Unlock()
	if calls != 3 {
		t.Fatalf("gemini calls = %d, want 3", calls)
	}
}

// 全員の1日の上限(SENSE_SUMMARY_DAILY_LIMIT)を超えたら、AI を呼ばずに「混み合っています」を返す。
// まとめには、大会の取り込みと別の軽いモデルを使う。
func TestSenseSummaryGlobalDailyLimit(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"{\"overview\":\"ok\"}"}]}}]}`))
	}))
	t.Cleanup(server.Close)
	h := setupConfig(t, func(c *config.Config) {
		c.SenseSummaryEnabled = true
		c.GeminiAPIKey = "test"
		c.GeminiEndpoint = server.URL + "/models/%s"
		c.SenseSummaryDailyLimit = 2
	})
	for i := 0; i < 2; i++ {
		tok := token(t, uuid.New())
		writeSenseNotes(t, h, tok, 0, 6)
		if rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes/summary", tok, nil); rec.Code != http.StatusCreated {
			t.Fatalf("summary %d = %d %s", i+1, rec.Code, rec.Body.String())
		}
	}
	third := token(t, uuid.New())
	writeSenseNotes(t, h, third, 0, 6)
	rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes/summary", third, nil)
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "summary_busy") {
		t.Fatalf("over the global limit = %d %s", rec.Code, rec.Body.String())
	}
	mu.Lock()
	defer mu.Unlock()
	if len(paths) != 2 {
		t.Fatalf("gemini calls = %d, want 2", len(paths))
	}
	if paths[0] != "/models/gemini-3.5-flash-lite" {
		t.Fatalf("model path = %s", paths[0])
	}
}

// 同時刻のメモも6個ずつ重複せずに使い、元の感覚は削除後もまとめから読める。
func TestSenseSummaryHistoryKeepsSixSourceNotes(t *testing.T) {
	ai := &fakeGemini{reply: `{"overview":"リズムをつかめました","good":[],"concerns":[],"next":[]}`}
	endpoint := ai.serve(t)
	h := setupConfig(t, func(c *config.Config) {
		c.SenseSummaryEnabled = true
		c.GeminiAPIKey = "test"
		c.GeminiEndpoint = endpoint
	})
	me, other := token(t, uuid.New()), token(t, uuid.New())
	writeSenseNotes(t, h, me, 0, 13)
	execSQLArgs(t, `UPDATE sense_notes SET created_at = '2026-01-01T00:00:00Z' WHERE author_id = $1`, tokenSubject(t, me))
	seen := map[uuid.UUID]bool{}
	for batch := 0; batch < 2; batch++ {
		rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes/summary", me, nil)
		var got struct {
			Summary struct {
				Notes []struct {
					ID   uuid.UUID `json:"id"`
					Body string    `json:"body"`
				} `json:"notes"`
			} `json:"summary"`
			Pending int `json:"pending_notes"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if rec.Code != http.StatusCreated || len(got.Summary.Notes) != 6 || got.Pending != 7-6*batch {
			t.Fatalf("batch %d = %d %s", batch, rec.Code, rec.Body.String())
		}
		for _, note := range got.Summary.Notes {
			if seen[note.ID] || note.Body == "" {
				t.Fatalf("duplicate or empty note: %+v", note)
			}
			seen[note.ID] = true
		}
	}
	execSQLArgs(t, `DELETE FROM sense_notes WHERE author_id = $1`, tokenSubject(t, me))
	rec := doJSON(t, h, http.MethodGet, "/v1/me/sense-notes/summaries", me, nil)
	var history struct {
		Summaries []struct {
			ID    uuid.UUID `json:"id"`
			Notes []struct {
				Body string `json:"body"`
			} `json:"notes"`
		} `json:"summaries"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &history)
	if rec.Code != http.StatusOK || len(history.Summaries) != 2 || len(history.Summaries[0].Notes) != 6 || history.Summaries[0].Notes[0].Body == "" {
		t.Fatalf("history after deletion = %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/me/sense-notes/summaries", other, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"summaries":[]`) {
		t.Fatalf("other history = %s", rec.Body.String())
	}
}

func TestSenseSummaryHistoryPagination(t *testing.T) {
	h := setupConfig(t, func(c *config.Config) { c.SenseSummaryEnabled = true })
	me := token(t, uuid.New())
	if rec := doJSON(t, h, http.MethodGet, "/v1/me", me, nil); rec.Code != http.StatusOK {
		t.Fatalf("profile = %d %s", rec.Code, rec.Body.String())
	}
	// 時刻が同じでも、IDを含むcursorで抜けなく読める。
	execSQLArgs(t, `INSERT INTO sense_note_summaries (id, author_id, from_day, to_day, note_count, summary, created_at)
	SELECT gen_random_uuid(), $1, '2026-10-01', '2026-10-06', 6, '{"overview":"以前のまとめ"}'::jsonb, '2026-10-06T00:00:00Z'
	FROM generate_series(1, 21)`, tokenSubject(t, me))
	var first struct {
		Summaries []struct {
			ID    uuid.UUID `json:"id"`
			Notes any       `json:"notes"`
		} `json:"summaries"`
		Cursor string `json:"next_cursor"`
	}
	rec := doJSON(t, h, http.MethodGet, "/v1/me/sense-notes/summaries", me, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &first)
	if rec.Code != http.StatusOK || len(first.Summaries) != 20 || first.Cursor == "" || first.Summaries[0].Notes != nil {
		t.Fatalf("first = %s", rec.Body.String())
	}
	var second struct {
		Summaries []struct {
			ID uuid.UUID `json:"id"`
		} `json:"summaries"`
		Cursor *string `json:"next_cursor"`
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/me/sense-notes/summaries?cursor="+first.Cursor, me, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &second)
	if rec.Code != http.StatusOK || len(second.Summaries) != 1 || second.Cursor != nil {
		t.Fatalf("second = %s", rec.Body.String())
	}
	for _, summary := range first.Summaries {
		if summary.ID == second.Summaries[0].ID {
			t.Fatal("duplicate summary")
		}
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/me/sense-notes/summaries?cursor=invalid", me, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid cursor = %d", rec.Code)
	}
}
