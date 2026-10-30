package httpx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/hinata/track-talk/apps/api/internal/mail"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAllowedDBApplicationName(t *testing.T) {
	allowed := []string{
		"track-talk-api", "track-talk-import", "track-talk-migrate",
		"supavisor", "postgrest", "Supabase Auth", "Supabase Realtime",
	}
	for _, name := range allowed {
		if !allowedDBApplicationName(name) {
			t.Fatalf("%q should be allowed", name)
		}
	}
	// 名札なし・自前名札の偽装（前方一致でのすり抜け）は許可しない。
	unknown := []string{"", "psql", "track-talk-api-2", "track-talk-importx", "attacker"}
	for _, name := range unknown {
		if allowedDBApplicationName(name) {
			t.Fatalf("%q should be unknown", name)
		}
	}
}

// 名札のない接続を1回だけ数えて知らせ、同じ接続では二度知らせない。
func TestCheckDBConnectionsAlertsOncePerConnection(t *testing.T) {
	ctx := context.Background()
	dsn := "postgres://tracktalk:tracktalk@127.0.0.1:55432/tracktalk?sslmode=disable"
	if err := config.RequireLocalDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("test database unavailable: %v", err)
	}
	// 名札のない接続を1本張って保持する。
	stranger, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer stranger.Release()

	s := &Server{pool: pool, cfg: config.Config{OpsAlertEmail: "ops@example.com"}, mail: mail.Client{DevLog: true}}
	unknown, alerted, err := s.checkDBConnections(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if unknown < 1 || alerted < 1 {
		t.Fatalf("first check: unknown=%d alerted=%d, want >=1/1", unknown, alerted)
	}
	_, alerted, err = s.checkDBConnections(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if alerted != 0 {
		t.Fatalf("second check: alerted=%d, want 0 (same connection)", alerted)
	}
}

func TestSecurityCheckRejectsBadSecret(t *testing.T) {
	s := &Server{cfg: config.Config{SecurityCheckSecret: "test-secret"}}
	for _, header := range []string{"", "wrong"} {
		req := httptest.NewRequest(http.MethodPost, "/internal/security/check", nil)
		req.Header.Set("X-Security-Check-Secret", header)
		rec := httptest.NewRecorder()
		s.securityCheck(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("header %q: status = %d, want 404", header, rec.Code)
		}
	}
	// 秘密が未設定なら、ヘッダーがあっても使えない。
	s.cfg.SecurityCheckSecret = ""
	req := httptest.NewRequest(http.MethodPost, "/internal/security/check", nil)
	req.Header.Set("X-Security-Check-Secret", "test-secret")
	rec := httptest.NewRecorder()
	s.securityCheck(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unset secret: status = %d, want 404", rec.Code)
	}
}

// OPS_ALERT_EMAIL 未設定なら、見慣れない接続があってもメールは送らず、件数だけ返す。
func TestCheckDBConnectionsWithoutOpsEmail(t *testing.T) {
	ctx := context.Background()
	dsn := "postgres://tracktalk:tracktalk@127.0.0.1:55432/tracktalk?sslmode=disable"
	if err := config.RequireLocalDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("test database unavailable: %v", err)
	}
	stranger, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer stranger.Release()
	s := &Server{pool: pool, mail: mail.Client{DevLog: true}} // OpsAlertEmail は未設定
	unknown, alerted, err := s.checkDBConnections(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if unknown < 1 {
		t.Fatalf("unknown=%d, want >=1", unknown)
	}
	if alerted != 0 {
		t.Fatalf("alerted=%d, want 0 without OPS_ALERT_EMAIL", alerted)
	}
}
