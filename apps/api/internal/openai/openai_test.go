package openai

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGenerateJSON(t *testing.T) {
	var gotAuth, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		gotAuth, gotBody = r.Header.Get("Authorization"), string(raw)
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"overview\":\"ok\"}"}}]}`))
	}))
	defer server.Close()

	out, err := Client{Key: "k", APIBase: server.URL}.GenerateJSON(context.Background(), "こんにちは")
	if err != nil || out != `{"overview":"ok"}` {
		t.Fatalf("out = %q err = %v", out, err)
	}
	if gotAuth != "Bearer k" || !strings.Contains(gotBody, `"model":"`+DefaultModel+`"`) || !strings.Contains(gotBody, "json_object") {
		t.Fatalf("auth = %q body = %s", gotAuth, gotBody)
	}
}

func TestGenerateJSONErrors(t *testing.T) {
	status, body := http.StatusTooManyRequests, `{"error":{"message":"slow down"}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()
	c := Client{Key: "k", APIBase: server.URL}
	if _, err := c.GenerateJSON(context.Background(), "x"); err == nil {
		t.Fatal("429 accepted")
	}
	status, body = http.StatusOK, `{"choices":[]}`
	if _, err := c.GenerateJSON(context.Background(), "x"); err == nil {
		t.Fatal("empty choices accepted")
	}
}
