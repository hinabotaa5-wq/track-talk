package httpx_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

type senseSummaryResponse struct {
	Summary *struct {
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
	NextAvailableAt *string `json:"next_available_at"`
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

func TestSenseSummaryIsHiddenUntilLaunch(t *testing.T) {
	h := setup(t)
	me := token(t, uuid.New())
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		if rec := doJSON(t, h, method, "/v1/me/sense-notes/summary", me, nil); rec.Code != http.StatusNotFound {
			t.Fatalf("%s while hidden = %d", method, rec.Code)
		}
	}
}

func TestSenseSummaryOfTheWeek(t *testing.T) {
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
	if rec.Code != http.StatusOK || got.Summary != nil || got.NextAvailableAt != nil {
		t.Fatalf("empty get = %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes/summary", me, nil); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("no notes = %d", rec.Code)
	}
	if len(gemini.prompts) != 0 {
		t.Fatal("gemini was called without notes")
	}

	for _, note := range []map[string]any{{"body": "接地で地面を押せた", "event": "100m"}, {"body": "右のハムが張る"}} {
		if rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes", me, note); rec.Code != http.StatusCreated {
			t.Fatalf("create note = %d", rec.Code)
		}
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes", other, map[string]any{"body": "他の人のメモ"}); rec.Code != http.StatusCreated {
		t.Fatalf("other note = %d", rec.Code)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/me/sense-notes/summary", me, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create summary = %d %s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Summary == nil || got.Summary.NoteCount != 2 || got.Summary.Summary.Overview == "" || len(got.Summary.Summary.Good) != 1 || got.NextAvailableAt == nil {
		t.Fatalf("summary = %s", rec.Body.String())
	}
	if got.Summary.FromDay >= got.Summary.ToDay {
		t.Fatalf("range = %s..%s", got.Summary.FromDay, got.Summary.ToDay)
	}
	prompt := gemini.prompts[0]
	if !strings.Contains(prompt, "[100m] 接地で地面を押せた") || !strings.Contains(prompt, "[種目なし] 右のハムが張る") || strings.Contains(prompt, "他の人のメモ") {
		t.Fatalf("prompt = %s", prompt)
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/me/sense-notes/summary", me, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Summary == nil || got.Summary.Summary.Concerns[0] != "右のハムに張りがある" || got.NextAvailableAt == nil {
		t.Fatalf("get = %s", rec.Body.String())
	}
	// 他の人には見えない。
	rec = doJSON(t, h, http.MethodGet, "/v1/me/sense-notes/summary", other, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Summary != nil {
		t.Fatalf("other sees = %s", rec.Body.String())
	}

	// 1週間に1回。2回目は 429。
	if rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes/summary", me, nil); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second summary in a week = %d", rec.Code)
	}
	// 他の人は別に数える。壊れた答えは 502 で、回数に数えない。
	gemini.reply = "not json"
	if rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes/summary", other, nil); rec.Code != http.StatusBadGateway {
		t.Fatalf("broken reply = %d", rec.Code)
	}
	gemini.reply = `{"overview":"ok","good":[],"concerns":[],"next":[]}`
	if rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes/summary", other, nil); rec.Code != http.StatusCreated {
		t.Fatalf("other summary after failure = %d", rec.Code)
	}
}
