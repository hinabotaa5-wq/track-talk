package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hinata/track-talk/apps/api/internal/config"
)

func TestPublicAPIUsesOnlyApprovedIdentityDisclosure(t *testing.T) {
	var failClosed atomic.Bool
	reader := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer synthetic-reader" {
			t.Errorf("unexpected request to identity service: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case strings.HasPrefix(r.URL.Path, "/internal/registration/"):
			w.WriteHeader(http.StatusNoContent)
		case strings.HasPrefix(r.URL.Path, "/internal/disclosures/") && !failClosed.Load():
			_, _ = w.Write([]byte(`{"required":true,"registered":true,"legal_name":"合成テスト","address":"テスト住所","phone":"000-0000-0000"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(reader.Close)
	h := setupConfig(t, func(cfg *config.Config) {
		cfg.IdentityReaderURL = reader.URL
		cfg.IdentityReaderToken = "synthetic-reader"
	})
	coach, tc, plan := planCoach(t, h, 3)
	path := "/v1/coaching/plans/" + plan
	rec := doJSON(t, h, http.MethodGet, path, "", nil)
	if rec.Code != 200 || rec.Header().Get("Cache-Control") != "no-store" || !strings.Contains(rec.Body.String(), "テスト住所") {
		t.Fatalf("public disclosure %d %s", rec.Code, rec.Body.String())
	}
	for _, endpoint := range []struct{ method, path string }{
		{http.MethodGet, "/v1/coach/me/commerce"},
		{http.MethodPut, "/v1/coach/me/commerce/identity"},
		{http.MethodPost, "/v1/admin/coaching/coaches/" + coach.String() + "/commerce-identity"},
	} {
		if rec := doJSON(t, h, endpoint.method, endpoint.path, tc, nil); rec.Code != http.StatusNotFound {
			t.Fatalf("old identity path still routed: %s %d", endpoint.path, rec.Code)
		}
	}
	failClosed.Store(true)
	rec = doJSON(t, h, http.MethodGet, path, "", nil)
	if rec.Code != 404 || strings.Contains(rec.Body.String(), "テスト住所") {
		t.Fatalf("invalidated disclosure %d %s", rec.Code, rec.Body.String())
	}
}
