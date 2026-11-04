package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

// fakeAuth は、退会で使う Supabase Auth の2つの API の代わり。
type fakeAuth struct {
	mu      sync.Mutex
	status  int
	body    string
	checked []string
	deleted []string
}

func (f *fakeAuth) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/auth/v1/user":
		f.checked = append(f.checked, r.Header.Get("Authorization"))
		w.WriteHeader(f.status)
		_, _ = w.Write([]byte(f.body))
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/auth/v1/admin/users/"):
		f.deleted = append(f.deleted, strings.TrimPrefix(r.URL.Path, "/auth/v1/admin/users/"))
	default:
		http.Error(w, "unexpected", http.StatusTeapot)
	}
}

func setupFakeAuth(t *testing.T, status int, body string) (http.Handler, *fakeAuth) {
	t.Helper()
	f := &fakeAuth{status: status, body: body}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	h := setupConfig(t, func(cfg *config.Config) {
		cfg.SupabaseURL = srv.URL
		cfg.SupabaseServiceRoleKey = "service-role-test"
	})
	return h, f
}

func tokenWithSession(t *testing.T, id uuid.UUID, sessionID string) string {
	t.Helper()
	secret := os.Getenv("SUPABASE_JWT_SECRET")
	if secret == "" {
		secret = "test-jwt-secret-for-ci-only"
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": id.String(), "aud": "authenticated", "exp": time.Now().Add(time.Hour).Unix(), "session_id": sessionID,
	}).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func profileCount(t *testing.T, id uuid.UUID) int {
	t.Helper()
	var n int
	queryRowArgs(t, `SELECT count(*) FROM profiles WHERE id = $1`, []any{id}, &n)
	return n
}

func TestWithdrawWithActiveSession(t *testing.T) {
	h, f := setupFakeAuth(t, http.StatusOK, `{}`)
	id := uuid.New()
	tok := tokenWithSession(t, id, uuid.NewString())
	if rec := doJSON(t, h, http.MethodGet, "/v1/me", tok, nil); rec.Code != http.StatusOK {
		t.Fatalf("me %d %s", rec.Code, rec.Body.String())
	}
	rec := doJSON(t, h, http.MethodDelete, "/v1/me", tok, map[string]string{"confirm": "退会する"})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"auth_deleted":true`) {
		t.Fatalf("withdraw %d %s", rec.Code, rec.Body.String())
	}
	if len(f.checked) != 1 || f.checked[0] != "Bearer "+tok {
		t.Fatalf("checked %q", f.checked)
	}
	if len(f.deleted) != 1 || f.deleted[0] != id.String() {
		t.Fatalf("deleted %q", f.deleted)
	}
	if profileCount(t, id) != 0 {
		t.Fatal("profile remains")
	}
}

func TestWithdrawRejectedWithoutActiveSession(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		noSID  bool
		want   int
		calls  int
	}{
		{name: "セッションが削除済み", status: http.StatusForbidden, body: `{"code":403,"error_code":"session_not_found"}`, want: http.StatusUnauthorized, calls: 1},
		{name: "Ban 中", status: http.StatusForbidden, body: `{"code":403,"error_code":"user_banned"}`, want: http.StatusUnauthorized, calls: 1},
		{name: "session_id の無いトークン", status: http.StatusOK, body: `{}`, noSID: true, want: http.StatusUnauthorized},
		{name: "Auth に確かめられない", status: http.StatusBadGateway, body: `oops`, want: http.StatusServiceUnavailable, calls: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, f := setupFakeAuth(t, tc.status, tc.body)
			id := uuid.New()
			tok := tokenWithSession(t, id, uuid.NewString())
			if tc.noSID {
				tok = token(t, id)
			}
			if rec := doJSON(t, h, http.MethodGet, "/v1/me", tok, nil); rec.Code != http.StatusOK {
				t.Fatalf("me %d %s", rec.Code, rec.Body.String())
			}
			rec := doJSON(t, h, http.MethodDelete, "/v1/me", tok, map[string]string{"confirm": "退会する"})
			if rec.Code != tc.want {
				t.Fatalf("withdraw %d %s", rec.Code, rec.Body.String())
			}
			if len(f.checked) != tc.calls {
				t.Fatalf("checked %d times", len(f.checked))
			}
			if len(f.deleted) != 0 {
				t.Fatalf("auth user deleted %q", f.deleted)
			}
			if profileCount(t, id) != 1 {
				t.Fatal("profile deleted")
			}
		})
	}
}

// セッションの確認は既存の退会ガードの手前で行い、有効なセッションには従来どおり 409 を返す。
func TestWithdrawChecksSessionBeforeGuards(t *testing.T) {
	cases := []struct {
		name   string
		status int
		want   int
	}{
		{name: "有効なセッション", status: http.StatusOK, want: http.StatusConflict},
		{name: "失効したセッション", status: http.StatusForbidden, want: http.StatusUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, f := setupFakeAuth(t, tc.status, `{}`)
			id := uuid.New()
			tok := tokenWithSession(t, id, uuid.NewString())
			if rec := doJSON(t, h, http.MethodGet, "/v1/me", tok, nil); rec.Code != http.StatusOK {
				t.Fatalf("me %d %s", rec.Code, rec.Body.String())
			}
			t.Cleanup(func() { execSQLArgs(t, `DELETE FROM review_payments WHERE payer_id = $1`, id) })
			execSQLArgs(t, `INSERT INTO review_payments (id, payer_id, method, amount_yen, checkout_session_id, checkout_url, session_expires_at, payment_intent_id, status, authorized_at)
				VALUES (gen_random_uuid(), $1, 'card', 1000, $2, 'https://checkout.stripe.test/x', now(), $2, 'authorized', now())`, id, "pi_guard_"+id.String())
			rec := doJSON(t, h, http.MethodDelete, "/v1/me", tok, map[string]string{"confirm": "退会する"})
			if rec.Code != tc.want {
				t.Fatalf("withdraw %d %s", rec.Code, rec.Body.String())
			}
			if tc.want == http.StatusConflict && !strings.Contains(rec.Body.String(), "review_in_progress") {
				t.Fatalf("guard %s", rec.Body.String())
			}
			if len(f.checked) != 1 || len(f.deleted) != 0 {
				t.Fatalf("checked %d deleted %d", len(f.checked), len(f.deleted))
			}
		})
	}
}
