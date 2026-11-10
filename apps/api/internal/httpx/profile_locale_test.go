package httpx_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

// 表示言語の設定（本体 #521）。GET /v1/me で返り、PATCH /v1/me で変えられる。
// 既定は ja、ja / en 以外は 400、送らなければそのまま。

func TestProfileLocaleDefaultAndUpdate(t *testing.T) {
	h := setupConfig(t, func(c *config.Config) {})
	tok := token(t, uuid.New())

	rec := doJSON(t, h, http.MethodGet, "/v1/me", tok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/me = %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"locale":"ja"`) {
		t.Fatalf("default locale is not ja: %s", rec.Body.String())
	}

	body := map[string]any{"nickname": "はしる", "nickname_public": true, "event": "100m", "locale": "en"}
	if rec := doJSON(t, h, http.MethodPatch, "/v1/me", tok, body); rec.Code != http.StatusOK {
		t.Fatalf("PATCH locale en = %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/me", tok, nil)
	if !strings.Contains(rec.Body.String(), `"locale":"en"`) {
		t.Fatalf("locale did not change to en: %s", rec.Body.String())
	}

	// locale を送らなければそのまま。
	keep := map[string]any{"nickname": "はしる", "nickname_public": true, "event": "100m"}
	if rec := doJSON(t, h, http.MethodPatch, "/v1/me", tok, keep); rec.Code != http.StatusOK {
		t.Fatalf("PATCH without locale = %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/me", tok, nil)
	if !strings.Contains(rec.Body.String(), `"locale":"en"`) {
		t.Fatalf("locale changed without sending: %s", rec.Body.String())
	}
}

func TestProfileLocaleRejectsUnknown(t *testing.T) {
	h := setupConfig(t, func(c *config.Config) {})
	tok := token(t, uuid.New())

	body := map[string]any{"nickname": "はしる", "nickname_public": true, "event": "100m", "locale": "fr"}
	rec := doJSON(t, h, http.MethodPatch, "/v1/me", tok, body)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("PATCH locale fr = %d %s, want 422", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "locale") {
		t.Fatalf("error does not name locale: %s", rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/me", tok, nil)
	if !strings.Contains(rec.Body.String(), `"locale":"ja"`) {
		t.Fatalf("locale changed on invalid input: %s", rec.Body.String())
	}
}
