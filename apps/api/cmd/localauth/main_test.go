package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestValidateTransport(t *testing.T) {
	for _, tc := range []struct {
		name      string
		cert      string
		key       string
		allowHTTP bool
		wantError bool
	}{
		{name: "reject HTTP by default", wantError: true},
		{name: "explicit local HTTP", allowHTTP: true},
		{name: "TLS certificate pair", cert: "cert.pem", key: "key.pem"},
		{name: "TLS with local HTTP allowed", cert: "cert.pem", key: "key.pem", allowHTTP: true},
		{name: "missing key", cert: "cert.pem", wantError: true},
		{name: "missing certificate", key: "key.pem", wantError: true},
		{name: "no HTTP fallback for partial TLS", cert: "cert.pem", allowHTTP: true, wantError: true},
		{name: "no HTTP fallback for key only", key: "key.pem", allowHTTP: true, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateTransport(tc.cert, tc.key, tc.allowHTTP)
			if (err != nil) != tc.wantError {
				t.Fatalf("validateTransport() = %v, wantError %v", err, tc.wantError)
			}
		})
	}
}

type memStore struct {
	mu       sync.Mutex
	byEmail  map[string]user
	byID     map[uuid.UUID]user
	refresh  map[string]uuid.UUID
	recovery map[string]uuid.UUID
}

func newMemStore() *memStore {
	return &memStore{
		byEmail:  map[string]user{},
		byID:     map[uuid.UUID]user{},
		refresh:  map[string]uuid.UUID{},
		recovery: map[string]uuid.UUID{},
	}
}

func (m *memStore) CreateUser(_ context.Context, email, hash string, id uuid.UUID) (user, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.byEmail[email]; ok {
		return user{}, errDuplicate
	}
	u := user{ID: id, Email: email, PasswordHash: hash, CreatedAt: time.Now()}
	m.byEmail[email] = u
	m.byID[id] = u
	return u, nil
}

func (m *memStore) UserByEmail(_ context.Context, email string) (user, error) {
	u, ok := m.byEmail[email]
	if !ok {
		return user{}, errNotFound
	}
	return u, nil
}

func (m *memStore) UserByID(_ context.Context, id uuid.UUID) (user, error) {
	u, ok := m.byID[id]
	if !ok {
		return user{}, errNotFound
	}
	return u, nil
}

func (m *memStore) UpdatePassword(_ context.Context, id uuid.UUID, hash string) error {
	u, ok := m.byID[id]
	if !ok {
		return errNotFound
	}
	u.PasswordHash = hash
	m.byID[id] = u
	m.byEmail[u.Email] = u
	return nil
}

func (m *memStore) UpdateEmail(_ context.Context, id uuid.UUID, email string) error {
	u, ok := m.byID[id]
	if !ok {
		return errNotFound
	}
	delete(m.byEmail, u.Email)
	u.Email = email
	m.byID[id] = u
	m.byEmail[email] = u
	return nil
}

func (m *memStore) DeleteUser(_ context.Context, id uuid.UUID) error {
	if u, ok := m.byID[id]; ok {
		delete(m.byEmail, u.Email)
		delete(m.byID, id)
	}
	return nil
}

func (m *memStore) SaveRefresh(_ context.Context, userID uuid.UUID, hash string, _ time.Time) error {
	m.refresh[hash] = userID
	return nil
}

func (m *memStore) ConsumeRefresh(_ context.Context, hash string) (uuid.UUID, error) {
	id, ok := m.refresh[hash]
	if !ok {
		return uuid.Nil, errNotFound
	}
	delete(m.refresh, hash)
	return id, nil
}

func (m *memStore) SaveRecovery(_ context.Context, userID uuid.UUID, hash string, _ time.Time) error {
	m.recovery[hash] = userID
	return nil
}

func (m *memStore) ConsumeRecovery(_ context.Context, hash string) (uuid.UUID, error) {
	id, ok := m.recovery[hash]
	if !ok {
		return uuid.Nil, errNotFound
	}
	delete(m.recovery, hash)
	return id, nil
}

func testServer() *server {
	return &server{
		store:       newMemStore(),
		secret:      []byte("test-secret"),
		externalURL: "http://localhost:9998/auth/v1",
		serviceKey:  "service-key",
	}
}

func do(t *testing.T, s *server, method, path, body, token string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)
	var out map[string]any
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("json: %v body=%s", err, rec.Body.String())
		}
	}
	return rec.Code, out
}

func TestSignupLoginRefreshAndGetUser(t *testing.T) {
	s := testServer()
	code, _ := do(t, s, http.MethodPost, "/auth/v1/signup", `{"email":"a@example.com","password":"password1"}`, "")
	if code != http.StatusOK {
		t.Fatalf("signup: %d", code)
	}
	code, sess := do(t, s, http.MethodPost, "/auth/v1/token?grant_type=password", `{"email":"a@example.com","password":"password1"}`, "")
	if code != http.StatusOK {
		t.Fatalf("login: %d %v", code, sess)
	}
	access, _ := sess["access_token"].(string)
	refresh, _ := sess["refresh_token"].(string)
	if access == "" || refresh == "" {
		t.Fatalf("tokens missing: %v", sess)
	}
	code, me := do(t, s, http.MethodGet, "/auth/v1/user", "", access)
	if code != http.StatusOK || me["email"] != "a@example.com" {
		t.Fatalf("user: %d %v", code, me)
	}
	// API 側の検証器と同じ条件(HS256・aud=authenticated)で読めること
	u, err := s.bearerUser(httptest.NewRequest(http.MethodGet, "/", nil))
	if err == nil && u.Email == "a@example.com" {
		t.Fatal("token なしで通ってはいけない")
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	if u, err := s.bearerUser(req); err != nil || u.Email != "a@example.com" {
		t.Fatalf("bearerUser: %v %v", u, err)
	}
	code, sess2 := do(t, s, http.MethodPost, "/auth/v1/token?grant_type=refresh_token", `{"refresh_token":"`+refresh+`"}`, "")
	if code != http.StatusOK || sess2["access_token"] == "" {
		t.Fatalf("refresh: %d %v", code, sess2)
	}
	// 使った refresh token は二度使えない
	code, _ = do(t, s, http.MethodPost, "/auth/v1/token?grant_type=refresh_token", `{"refresh_token":"`+refresh+`"}`, "")
	if code == http.StatusOK {
		t.Fatal("refresh token の再利用が通ってはいけない")
	}
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	s := testServer()
	_, _ = do(t, s, http.MethodPost, "/auth/v1/signup", `{"email":"b@example.com","password":"password1"}`, "")
	code, _ := do(t, s, http.MethodPost, "/auth/v1/token?grant_type=password", `{"email":"b@example.com","password":"wrong-pass"}`, "")
	if code != http.StatusBadRequest {
		t.Fatalf("wrong password: %d", code)
	}
}

func TestRecoveryRedirect(t *testing.T) {
	for _, tc := range []struct {
		name     string
		redirect string
		want     string
	}{
		{name: "default", want: "http://localhost:3000/reset-password/update"},
		{name: "password update", redirect: "http://localhost:3000/reset-password/update", want: "http://localhost:3000/reset-password/update"},
		{name: "callback", redirect: "http://localhost:3000/auth/callback", want: "http://localhost:3000/auth/callback"},
		{name: "external", redirect: "https://evil.example/"},
		{name: "protocol relative", redirect: "//evil.example/"},
		{name: "javascript", redirect: "javascript:alert(1)"},
		{name: "userinfo", redirect: "http://localhost:3000@evil.example/auth/callback"},
		{name: "host suffix", redirect: "http://localhost.evil.example:3000/auth/callback"},
		{name: "different port", redirect: "http://localhost:4000/auth/callback"},
		{name: "other local path", redirect: "http://localhost:3000/other"},
		{name: "query redirect", redirect: "http://localhost:3000/auth/callback?next=https://evil.example"},
		{name: "fragment", redirect: "http://localhost:3000/auth/callback#https://evil.example"},
		{name: "encoded path", redirect: "http://localhost:3000/auth/%63allback"},
		{name: "backslash", redirect: `http://localhost:3000\@evil.example/auth/callback`},
		{name: "header injection", redirect: "http://localhost:3000/auth/callback\r\nLocation: https://evil.example"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := testServer()
			store := s.store.(*memStore)
			u, err := store.CreateUser(context.Background(), "a@example.com", "unused", uuid.New())
			if err != nil {
				t.Fatal(err)
			}
			const token = "one-time-recovery-token"
			if err := store.SaveRecovery(context.Background(), u.ID, tokenHash(token), time.Now().Add(time.Hour)); err != nil {
				t.Fatal(err)
			}
			query := url.Values{"type": {"recovery"}, "token": {token}, "redirect_to": {tc.redirect}}
			req := httptest.NewRequest(http.MethodGet, "/auth/v1/verify?"+query.Encode(), nil)
			rec := httptest.NewRecorder()
			s.routes().ServeHTTP(rec, req)
			if tc.want == "" {
				if rec.Code != http.StatusBadRequest || rec.Header().Get("Location") != "" {
					t.Fatalf("unsafe redirect: status=%d location=%q", rec.Code, rec.Header().Get("Location"))
				}
				if _, ok := store.recovery[tokenHash(token)]; !ok || len(store.refresh) != 0 {
					t.Fatal("rejected redirect consumed recovery token or created a session")
				}
				return
			}
			if rec.Code != http.StatusFound {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			target, err := url.Parse(rec.Header().Get("Location"))
			if err != nil {
				t.Fatal(err)
			}
			fragment, err := url.ParseQuery(target.Fragment)
			if err != nil || fragment.Get("access_token") == "" || fragment.Get("refresh_token") == "" || fragment.Get("type") != "recovery" {
				t.Fatal("missing recovery session in redirect")
			}
			target.Fragment = ""
			if target.String() != tc.want {
				t.Fatalf("target=%q want=%q", target.String(), tc.want)
			}
			if _, ok := store.recovery[tokenHash(token)]; ok {
				t.Fatal("successful recovery must consume token")
			}
			replay := httptest.NewRecorder()
			s.routes().ServeHTTP(replay, req)
			if replay.Code != http.StatusBadRequest {
				t.Fatal("recovery token was reusable")
			}
		})
	}
}
