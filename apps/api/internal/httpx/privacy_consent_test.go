package httpx_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestOverseasConsent(t *testing.T) {
	h := setup(t)
	user := token(t, uuid.New())

	overseasConsent := func() bool {
		t.Helper()
		rec := doJSON(t, h, http.MethodGet, "/v1/me", user, nil)
		if rec.Code != 200 {
			t.Fatalf("me %d %s", rec.Code, rec.Body.String())
		}
		var me struct {
			OverseasConsent *bool `json:"overseas_consent"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &me)
		if me.OverseasConsent == nil {
			t.Fatalf("overseas_consent がない: %s", rec.Body.String())
		}
		return *me.OverseasConsent
	}

	if overseasConsent() {
		t.Fatal("同意する前から true")
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/me/consents/overseas", "", nil); rec.Code != 401 {
		t.Fatalf("anon %d", rec.Code)
	}
	for range 2 {
		if rec := doJSON(t, h, http.MethodPost, "/v1/me/consents/overseas", user, nil); rec.Code != 204 {
			t.Fatalf("consent %d %s", rec.Code, rec.Body.String())
		}
	}
	if !overseasConsent() {
		t.Fatal("同意したのに false")
	}
}

// 登録画面で同意した版を添えて記録する。いまの版と違えば記録しない。
func TestOverseasConsentWithVersion(t *testing.T) {
	h := setup(t)
	user := token(t, uuid.New())

	if rec := doJSON(t, h, http.MethodPost, "/v1/me/consents/overseas", user, map[string]string{"version": "overseas-2000-01"}); rec.Code != 409 {
		t.Fatalf("old version %d %s", rec.Code, rec.Body.String())
	}
	rec := doJSON(t, h, http.MethodGet, "/v1/me", user, nil)
	var me struct {
		OverseasConsent bool `json:"overseas_consent"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &me)
	if me.OverseasConsent {
		t.Fatal("古い版の同意で true になった")
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/me/consents/overseas", user, map[string]string{"version": "overseas-2026-10"}); rec.Code != 204 {
		t.Fatalf("current version %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/me", user, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &me)
	if !me.OverseasConsent {
		t.Fatal("いまの版で同意したのに false")
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/me/consents/overseas", user, map[string]any{"unknown": 1}); rec.Code != 400 {
		t.Fatalf("unknown field %d", rec.Code)
	}
}
