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
