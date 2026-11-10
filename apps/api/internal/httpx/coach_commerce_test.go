package httpx_test

import (
	"bytes"
	"log"
	"log/slog"
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

// 表記不要のコーチの電話番号は、専用サービスが誤って返しても公開 API から出さない。
// 表記要でも、氏名・住所・電話番号がそろわない応答は公開しない。ログにも平文を残さない。
func TestPublicAPINeverExposesUnapprovedPhone(t *testing.T) {
	var logs bytes.Buffer
	prevSlog, prevLog := slog.Default(), log.Writer()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	log.SetOutput(&logs)
	t.Cleanup(func() {
		slog.SetDefault(prevSlog)
		log.SetOutput(prevLog)
	})
	const phone = "000-1111-2222"
	var body atomic.Value
	reader := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/internal/registration/"):
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/internal/disclosures":
			_, _ = w.Write([]byte(`[]`))
		case strings.HasPrefix(r.URL.Path, "/internal/disclosures/"):
			_, _ = w.Write([]byte(body.Load().(string)))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(reader.Close)
	h := setupConfig(t, func(cfg *config.Config) {
		cfg.IdentityReaderURL = reader.URL
		cfg.IdentityReaderToken = "synthetic-reader"
	})
	_, _, plan := planCoach(t, h, 3)
	path := "/v1/coaching/plans/" + plan

	body.Store(`{"required":false,"registered":true,"legal_name":"合成テスト","address":"テスト住所","phone":"` + phone + `"}`)
	rec := doJSON(t, h, http.MethodGet, path, "", nil)
	if rec.Code != 200 || strings.Contains(rec.Body.String(), phone) || strings.Contains(rec.Body.String(), "テスト住所") {
		t.Fatalf("exempt coach identity reached the public API: %d", rec.Code)
	}

	body.Store(`{"required":true,"registered":true,"legal_name":"合成テスト","address":"テスト住所"}`)
	rec = doJSON(t, h, http.MethodGet, path, "", nil)
	if strings.Contains(rec.Body.String(), "テスト住所") {
		t.Fatalf("disclosure without a confirmed phone was published: %d", rec.Code)
	}

	body.Store(`{"required":true,"registered":true,"legal_name":"合成テスト","address":"テスト住所","phone":"` + phone + `"}`)
	if rec := doJSON(t, h, http.MethodGet, path, "", nil); rec.Code != 200 || !strings.Contains(rec.Body.String(), phone) {
		t.Fatalf("confirmed disclosure was not published: %d", rec.Code)
	}
	if strings.Contains(logs.String(), phone) || strings.Contains(logs.String(), "テスト住所") {
		t.Fatal("identity plaintext appeared in logs")
	}
}
