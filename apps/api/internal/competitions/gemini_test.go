package competitions

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
