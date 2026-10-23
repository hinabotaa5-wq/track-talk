package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

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
