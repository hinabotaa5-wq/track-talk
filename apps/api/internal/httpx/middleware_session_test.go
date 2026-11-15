package httpx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

// sessionStub は、Supabase Auth の GET /auth/v1/user の代役。呼ばれた回数を数える。
type sessionStub struct {
	mu      sync.Mutex
	status  int
	body    string
	checked []string
}

func (s *sessionStub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.Method == http.MethodGet && r.URL.Path == "/auth/v1/user" {
		s.checked = append(s.checked, r.Header.Get("Authorization"))
		w.WriteHeader(s.status)
		_, _ = w.Write([]byte(s.body))
		return
	}
	http.Error(w, "unexpected", http.StatusTeapot)
}

func (s *sessionStub) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.checked)
}

const sessionTestSecret = "test-jwt-secret-for-ci-only"

func sessionTestVerifier(t testing.TB) *auth.Verifier {
	t.Helper()
	v, err := auth.NewVerifier(sessionTestSecret, "https://example.invalid", "authenticated")
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func sessionTestToken(t testing.TB, id uuid.UUID, sessionID, aal string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub": id.String(),
		"aud": "authenticated",
		"iss": "https://example.invalid/auth/v1",
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	if sessionID != "" {
		claims["session_id"] = sessionID
	}
	if aal != "" {
		claims["aal"] = aal
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(sessionTestSecret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func sessionTestAdmin(srv *httptest.Server) auth.Admin {
	return auth.Admin{BaseURL: srv.URL, APIKey: "service-role-test", HTTP: srv.Client()}
}

// writeCase は、書き込みメソッドごとの確認の振る舞いを確かめる。
func TestRequireAuthWriteSession(t *testing.T) {
	methods := []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}
	cases := []struct {
		name    string
		status  int
		body    string
		want    int
		code    string
		calls   int
		handler bool
	}{
		{name: "有効なセッション", status: http.StatusOK, body: `{}`, want: http.StatusNoContent, calls: 1, handler: true},
		{name: "セッションが削除済み", status: http.StatusForbidden, body: `{"code":403,"error_code":"session_not_found"}`,
			want: http.StatusUnauthorized, code: "session_inactive", calls: 1},
		{name: "Ban 中", status: http.StatusForbidden, body: `{"code":403,"error_code":"user_banned"}`,
			want: http.StatusUnauthorized, code: "session_inactive", calls: 1},
	}
	for _, tc := range cases {
		for _, method := range methods {
			t.Run(tc.name+"/"+method, func(t *testing.T) {
				stub := &sessionStub{status: tc.status, body: tc.body}
				srv := httptest.NewServer(stub)
				t.Cleanup(srv.Close)
				var called bool
				h := RequireAuth(sessionTestVerifier(t), nil, sessionTestAdmin(srv))(
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						called = true
						w.WriteHeader(http.StatusNoContent)
					}))
				tok := sessionTestToken(t, uuid.New(), uuid.NewString(), "")
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(method, "/v1/posts", nil)
				req.Header.Set("Authorization", "Bearer "+tok)
				h.ServeHTTP(rec, req)
				if rec.Code != tc.want {
					t.Fatalf("status %d want %d: %s", rec.Code, tc.want, rec.Body.String())
				}
				if tc.code != "" && !strings.Contains(rec.Body.String(), `"`+tc.code+`"`) {
					t.Fatalf("code %s not in %s", tc.code, rec.Body.String())
				}
				if got := stub.count(); got != tc.calls {
					t.Fatalf("auth checked %d times want %d", got, tc.calls)
				}
				if called != tc.handler {
					t.Fatalf("handler called %v want %v", called, tc.handler)
				}
			})
		}
	}
}

// 読み取り(GET／HEAD／OPTIONS)は、セッションが無効でも確認なしで通る。
func TestRequireAuthReadSkipsSessionCheck(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		t.Run(method, func(t *testing.T) {
			stub := &sessionStub{status: http.StatusForbidden, body: `{"error_code":"session_not_found"}`}
			srv := httptest.NewServer(stub)
			t.Cleanup(srv.Close)
			var called bool
			h := RequireAuth(sessionTestVerifier(t), nil, sessionTestAdmin(srv))(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					called = true
					w.WriteHeader(http.StatusNoContent)
				}))
			tok := sessionTestToken(t, uuid.New(), uuid.NewString(), "")
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(method, "/v1/me", nil)
			req.Header.Set("Authorization", "Bearer "+tok)
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusNoContent {
				t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
			}
			if !called {
				t.Fatal("handler not called")
			}
			if got := stub.count(); got != 0 {
				t.Fatalf("auth checked %d times want 0", got)
			}
		})
	}
}

// Auth に確かめられないときの書き込みは、進めず 503 で止める。
func TestRequireAuthWriteSessionAuthFailure(t *testing.T) {
	t.Run("想定外のステータス", func(t *testing.T) {
		stub := &sessionStub{status: http.StatusBadGateway, body: `oops`}
		srv := httptest.NewServer(stub)
		t.Cleanup(srv.Close)
		var called bool
		h := RequireAuth(sessionTestVerifier(t), nil, sessionTestAdmin(srv))(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusNoContent)
			}))
		tok := sessionTestToken(t, uuid.New(), uuid.NewString(), "")
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/posts", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), `"unavailable"`) {
			t.Fatalf("code unavailable not in %s", rec.Body.String())
		}
		if called {
			t.Fatal("handler called")
		}
		if got := stub.count(); got != 1 {
			t.Fatalf("auth checked %d times want 1", got)
		}
	})
	t.Run("接続不可", func(t *testing.T) {
		stub := &sessionStub{status: http.StatusOK, body: `{}`}
		srv := httptest.NewServer(stub)
		admin := sessionTestAdmin(srv)
		srv.Close()
		var called bool
		h := RequireAuth(sessionTestVerifier(t), nil, admin)(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusNoContent)
			}))
		tok := sessionTestToken(t, uuid.New(), uuid.NewString(), "")
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/posts", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
		}
		if called {
			t.Fatal("handler called")
		}
	})
}

// session_id の無いトークンの書き込みは 401 で止め、同じトークンの読み取りは従来どおり通る。
func TestRequireAuthWriteSessionWithoutSessionID(t *testing.T) {
	stub := &sessionStub{status: http.StatusOK, body: `{}`}
	srv := httptest.NewServer(stub)
	t.Cleanup(srv.Close)
	var called bool
	h := RequireAuth(sessionTestVerifier(t), nil, sessionTestAdmin(srv))(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusNoContent)
		}))
	tok := sessionTestToken(t, uuid.New(), "", "")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/posts", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("post status %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"session_inactive"`) {
		t.Fatalf("code session_inactive not in %s", rec.Body.String())
	}
	if called {
		t.Fatal("handler called for post")
	}
	if got := stub.count(); got != 0 {
		t.Fatalf("auth checked %d times want 0", got)
	}

	called = false
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("get status %d: %s", rec.Code, rec.Body.String())
	}
	if !called {
		t.Fatal("handler not called for get")
	}
	if got := stub.count(); got != 0 {
		t.Fatalf("auth checked %d times want 0", got)
	}
}

// 二段階認証が未完の書き込みは、従来どおり 403(mfa_required) が先に返り、セッションの確認はしない。
func TestRequireAuthMFARequiredBeforeSessionCheck(t *testing.T) {
	newEnrolledMFA := func(t *testing.T, id uuid.UUID) *MFAEnrollment {
		t.Helper()
		// 接続は張らず、覚えた enroll の結果だけを見る。Enrolled は覚えを見る前に
		// pool の有無だけ確かめるので、つながらない pool でよい。
		poolCfg, err := pgxpool.ParseConfig("postgres://127.0.0.1:1/nope?connect_timeout=1")
		if err != nil {
			t.Fatal(err)
		}
		pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(pool.Close)
		mfa := NewMFAEnrollment(pool)
		mfa.cache[id] = mfaEnrollEntry{enrolled: true, until: time.Now().Add(time.Hour)}
		return mfa
	}

	t.Run("aal1 は mfa_required が先", func(t *testing.T) {
		stub := &sessionStub{status: http.StatusInternalServerError, body: `oops`}
		srv := httptest.NewServer(stub)
		t.Cleanup(srv.Close)
		id := uuid.New()
		var called bool
		h := RequireAuth(sessionTestVerifier(t), newEnrolledMFA(t, id), sessionTestAdmin(srv))(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusNoContent)
			}))
		tok := sessionTestToken(t, id, uuid.NewString(), "aal1")
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/posts", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), `"mfa_required"`) {
			t.Fatalf("code mfa_required not in %s", rec.Body.String())
		}
		if called {
			t.Fatal("handler called")
		}
		if got := stub.count(); got != 0 {
			t.Fatalf("auth checked %d times want 0", got)
		}
	})
	t.Run("aal2 はセッションの確認まで進む", func(t *testing.T) {
		stub := &sessionStub{status: http.StatusOK, body: `{}`}
		srv := httptest.NewServer(stub)
		t.Cleanup(srv.Close)
		id := uuid.New()
		var called bool
		h := RequireAuth(sessionTestVerifier(t), newEnrolledMFA(t, id), sessionTestAdmin(srv))(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusNoContent)
			}))
		tok := sessionTestToken(t, id, uuid.NewString(), "aal2")
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/posts", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
		}
		if !called {
			t.Fatal("handler not called")
		}
		if got := stub.count(); got != 1 {
			t.Fatalf("auth checked %d times want 1", got)
		}
	})
}

// authAdmin が未設定なら、書き込みも従来どおり通す(警告ログのみ)。
func TestRequireAuthWriteSessionWithoutAdmin(t *testing.T) {
	var called bool
	h := RequireAuth(sessionTestVerifier(t), nil, auth.Admin{})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusNoContent)
		}))
	tok := sessionTestToken(t, uuid.New(), uuid.NewString(), "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/posts", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if !called {
		t.Fatal("handler not called")
	}
}
