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
