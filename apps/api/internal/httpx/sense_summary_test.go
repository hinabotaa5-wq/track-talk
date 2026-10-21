package httpx_test

import (
	"context"
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
	mu      sync.Mutex
	reply   string
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
		payload, _ := json.Marshal(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]string{"text": f.reply}}}}}})
		_, _ = w.Write(payload)
	}))
	t.Cleanup(server.Close)
	return server.URL
}

// writeSenseNotes は、感覚メモを count 個、今日から daysBack 日前のようにさかのぼって1日1つずつ書く。
// 1つ目は種目つき、2つ目は「右のハムが張る」。
func writeSenseNotes(t *testing.T, h http.Handler, tok string, daysBack, count int) {
	t.Helper()
	for i := 0; i < count; i++ {
		day := time.Now().In(jst.Zone).AddDate(0, 0, -(daysBack + i)).Format(time.DateOnly)
		note := map[string]any{"body": fmt.Sprintf("感覚%d", i+1), "day": day}
		switch i {
		case 0:
			note = map[string]any{"body": "接地で地面を押せた", "event": "100m", "day": day}
		case 1:
			note = map[string]any{"body": "右のハムが張る", "day": day}
		}
		if rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes", tok, note); rec.Code != http.StatusCreated {
			t.Fatalf("create note %d = %d", i+1, rec.Code)
		}
	}
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
	if rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes", me, map[string]any{"body": "6個目", "day": time.Now().In(jst.Zone).AddDate(0, 0, -5).Format(time.DateOnly)}); rec.Code != http.StatusCreated {
		t.Fatalf("sixth note = %d", rec.Code)
	}

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
	gemini.reply = `{"overview":"ok","good":[],"concerns":[],"next":[]}`
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
