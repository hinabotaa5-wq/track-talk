package competitions

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestExtractReadsJSONAndStopsOnRateLimit(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-goog-api-key") == "" {
			t.Fatal("missing api key")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"candidates": []map[string]any{{
				"content": map[string]any{
					"parts": []map[string]string{{
						"text": `{"meets":[{"name":"記録会","starts_on":"2026-04-18","ends_on":"","location":"白波","guidelines_url":"","district_url":""}]}`,
					}},
				},
			}},
		})
	}))
	defer ok.Close()
	client := Client{Key: "test", HTTP: ok.Client()}
	original := generateEndpoint
	generateEndpoint = ok.URL
	defer func() { generateEndpoint = original }()

	drafts, err := client.Extract(context.Background(), Page{SeasonYear: 2026, Body: "日: 4/18"})
	if err != nil {
		t.Fatal(err)
	}
	if len(drafts) != 1 || drafts[0].Name != "記録会" || drafts[0].StartsOn != "2026-04-18" {
		t.Fatalf("drafts %+v", drafts)
	}

	limited := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"status":"RESOURCE_EXHAUSTED"}}`))
	}))
	defer limited.Close()
	generateEndpoint = limited.URL
	client.HTTP = limited.Client()
	_, err = client.Extract(context.Background(), Page{SeasonYear: 2026})
	if err == nil || !strings.Contains(err.Error(), ErrRateLimited.Error()) {
		t.Fatalf("rate limit: %v", err)
	}
}

// 大会がないページで meets が null や省略で返っても、パース失敗にせず「大会なし」として読む。
func TestParseDraftsAcceptsEmptyMeetsObject(t *testing.T) {
	for _, raw := range []string{`{"meets":null}`, `{}`, `{"meets":[]}`, "```json\n{\"meets\": null}\n```"} {
		drafts, err := parseDrafts(raw)
		if err != nil || len(drafts) != 0 {
			t.Fatalf("%q: %v %v", raw, drafts, err)
		}
	}
	drafts, err := parseDrafts(`[{"name":"A","starts_on":"2026-10-17"}]`)
	if err != nil || len(drafts) != 1 {
		t.Fatalf("array: %v %v", drafts, err)
	}
}

func TestGenerateJSONOutputCapIsOptIn(t *testing.T) {
	var received []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			GenerationConfig map[string]any `json:"generationConfig"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		received = append(received, body.GenerationConfig)
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"{}"}]}}]}`))
	}))
	defer server.Close()
	for _, limit := range []int{0, 2048} {
		client := Client{Key: "test", Endpoint: server.URL, MaxOutputTokens: limit}
		if _, err := client.GenerateJSON(context.Background(), "synthetic"); err != nil {
			t.Fatal(err)
		}
	}
	if _, ok := received[0]["maxOutputTokens"]; ok {
		t.Fatal("competition default was capped")
	}
	if received[1]["maxOutputTokens"] != float64(2048) {
		t.Fatalf("config=%v", received[1])
	}
}

func TestGenerateRetriesOnlyTransientFailures(t *testing.T) {
	old := retryWait
	defer func() { retryWait = old }()
	var delays []time.Duration
	retryWait = func(ctx context.Context, d time.Duration) error { delays = append(delays, d); return ctx.Err() }
	for _, tc := range []struct {
		status  int
		payload string
		want    error
		calls   int
	}{
		{503, "high demand", ErrTransient, 3},
		{429, "", ErrRateLimited, 1},
		{503, "RESOURCE_EXHAUSTED", ErrRateLimited, 1},
		{401, "secret-key", ErrAuthentication, 1},
		{403, "API key not valid secret-key", ErrAuthentication, 1},
		{400, "API_KEY_INVALID secret-key", ErrAuthentication, 1},
		{500, "secret-key", nil, 1},
	} {
		t.Run(strconv.Itoa(tc.status)+tc.payload, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.WriteHeader(tc.status)
				io.WriteString(w, tc.payload)
			}))
			defer srv.Close()
			_, err := (Client{Endpoint: srv.URL}).GenerateJSON(context.Background(), "test")
			if err == nil || (tc.want != nil && !errors.Is(err, tc.want)) || calls != tc.calls || strings.Contains(err.Error(), "secret-key") {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
		})
	}
	if len(delays) != 2 || delays[0] != 15*time.Second || delays[1] != 30*time.Second {
		t.Fatal(delays)
	}
}

type timeoutTransport struct {
	calls  int
	cancel context.CancelFunc
}

func (rt *timeoutTransport) RoundTrip(*http.Request) (*http.Response, error) {
	rt.calls++
	if rt.cancel != nil {
		rt.cancel()
	}
	return nil, context.DeadlineExceeded
}
func TestGenerateTimeoutAndParentCancellation(t *testing.T) {
	old := retryWait
	defer func() { retryWait = old }()
	retryWait = func(ctx context.Context, _ time.Duration) error { return ctx.Err() }
	rt := &timeoutTransport{}
	_, err := (Client{HTTP: &http.Client{Transport: rt}}).GenerateJSON(context.Background(), "test")
	if !errors.Is(err, ErrTransient) || rt.calls != 3 {
		t.Fatalf("%d %v", rt.calls, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rt = &timeoutTransport{cancel: cancel}
	_, err = (Client{HTTP: &http.Client{Transport: rt}}).GenerateJSON(ctx, "test")
	if !errors.Is(err, context.Canceled) || rt.calls != 1 {
		t.Fatalf("%d %v", rt.calls, err)
	}
}
func TestWaitCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !errors.Is(Wait(ctx, time.Hour), context.Canceled) {
		t.Fatal("not canceled")
	}
}
func TestGenerateRecoversAfter503(t *testing.T) {
	old := retryWait
	defer func() { retryWait = old }()
	retryWait = func(context.Context, time.Duration) error { return nil }
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(503)
			return
		}
		io.WriteString(w, `{"candidates":[{"content":{"parts":[{"text":"{}"}]}}]}`)
	}))
	defer srv.Close()
	text, err := (Client{Endpoint: srv.URL}).GenerateJSON(context.Background(), "test")
	if err != nil || text != "{}" || calls != 2 {
		t.Fatalf("%s %v %d", text, err, calls)
	}
}
