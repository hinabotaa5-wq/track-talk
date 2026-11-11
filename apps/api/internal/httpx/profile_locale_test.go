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

// マイページの「表示言語」の保存（本体 #535）。PATCH /v1/me/locale は言語だけを変え、
// ニックネーム・公開設定・種目には触れない。PATCH /v1/me に言語だけを送ると
// ニックネーム必須で 422 になり、本番で保存できなかった。
func TestProfileLocaleOnlyEndpoint(t *testing.T) {
	h := setupConfig(t, func(c *config.Config) {})
	tok := token(t, uuid.New())

	setup := map[string]any{"nickname": "はしる", "nickname_public": true, "event": "100m"}
	if rec := doJSON(t, h, http.MethodPatch, "/v1/me", tok, setup); rec.Code != http.StatusOK {
		t.Fatalf("PATCH /v1/me setup = %d %s", rec.Code, rec.Body.String())
	}

	// 以前の Web の送り方（言語だけを PATCH /v1/me）は 422 になる。原因の再現として残す。
	if rec := doJSON(t, h, http.MethodPatch, "/v1/me", tok, map[string]any{"locale": "en"}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("PATCH /v1/me locale only = %d %s, want 422", rec.Code, rec.Body.String())
	}

	for _, want := range []string{"en", "ja"} {
		rec := doJSON(t, h, http.MethodPatch, "/v1/me/locale", tok, map[string]any{"locale": want})
		if rec.Code != http.StatusOK {
			t.Fatalf("PATCH /v1/me/locale %s = %d %s", want, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), `"locale":"`+want+`"`) {
			t.Fatalf("response locale is not %s: %s", want, rec.Body.String())
		}
		rec = doJSON(t, h, http.MethodGet, "/v1/me", tok, nil)
		body := rec.Body.String()
		if !strings.Contains(body, `"locale":"`+want+`"`) {
			t.Fatalf("locale did not change to %s: %s", want, body)
		}
		for _, keep := range []string{`"nickname":"はしる"`, `"nickname_public":true`, `"event":"100m"`} {
			if !strings.Contains(body, keep) {
				t.Fatalf("PATCH /v1/me/locale changed other fields (%s missing): %s", keep, body)
			}
		}
	}

	for _, bad := range []map[string]any{{"locale": "fr"}, {"locale": ""}, {}} {
		rec := doJSON(t, h, http.MethodPatch, "/v1/me/locale", tok, bad)
		if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "locale") {
			t.Fatalf("PATCH /v1/me/locale %v = %d %s, want 422 naming locale", bad, rec.Code, rec.Body.String())
		}
	}
	rec := doJSON(t, h, http.MethodGet, "/v1/me", tok, nil)
	if !strings.Contains(rec.Body.String(), `"locale":"ja"`) {
		t.Fatalf("locale changed on invalid input: %s", rec.Body.String())
	}

	// 未ログインは 401。
	if rec := doJSON(t, h, http.MethodPatch, "/v1/me/locale", "", map[string]any{"locale": "en"}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("PATCH /v1/me/locale without token = %d, want 401", rec.Code)
	}
}
