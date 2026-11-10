package httpx_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hinata/track-talk/apps/api/internal/httpx"
)

// 二段階認証の設定済み判定の関数（migration 000153）は、実行を許したロールだけが呼べ、
// 従来の直接の読み取りと同じ真偽を返し、呼べるロールでも auth.mfa_factors は直接読めない。
func TestMFAVerifiedFunctionPrivileges(t *testing.T) {
	setup(t)
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, testDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	if _, err := admin.Exec(ctx, `
		CREATE SCHEMA IF NOT EXISTS auth;
		CREATE TABLE IF NOT EXISTS auth.mfa_factors (id uuid PRIMARY KEY, user_id uuid NOT NULL, status text NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	verified, pending, plain := uuid.New(), uuid.New(), uuid.New()
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), `DELETE FROM auth.mfa_factors WHERE user_id = ANY($1)`, []uuid.UUID{verified, pending})
	})
	if _, err := admin.Exec(ctx, `INSERT INTO auth.mfa_factors (id, user_id, status) VALUES ($1, $2, 'verified'), ($3, $4, 'unverified')`,
		uuid.New(), verified, uuid.New(), pending); err != nil {
		t.Fatal(err)
	}

	var definer bool
	var config []string
	if err := admin.QueryRow(ctx, `SELECT prosecdef, COALESCE(proconfig, '{}') FROM pg_proc WHERE oid = 'public.mfa_verified(uuid)'::regprocedure`).Scan(&definer, &config); err != nil {
		t.Fatal(err)
	}
	if !definer || strings.Join(config, ",") != "search_path=pg_catalog" {
		t.Fatalf("definer %v config %v", definer, config)
	}

	newRole := func(prefix string) string {
		role := prefix + strings.ReplaceAll(uuid.NewString(), "-", "")
		if _, err := admin.Exec(ctx, "CREATE ROLE "+role); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_, _ = admin.Exec(context.Background(), "DROP OWNED BY "+role)
			_, _ = admin.Exec(context.Background(), "DROP ROLE "+role)
		})
		return role
	}
	allowed, other := newRole("mfa_allowed_"), newRole("mfa_other_")
	// 本番の track_talk_api への付与（migration の中の GRANT）と同じ付与を、試験用のロールに行う。
	if _, err := admin.Exec(ctx, "GRANT EXECUTE ON FUNCTION public.mfa_verified(uuid) TO "+allowed); err != nil {
		t.Fatal(err)
	}
	asRole := func(role string) *pgxpool.Pool {
		cfg, err := pgxpool.ParseConfig(testDSN(t))
		if err != nil {
			t.Fatal(err)
		}
		cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
			_, err := conn.Exec(ctx, "SET ROLE "+role)
			return err
		}
		p, err := pgxpool.NewWithConfig(ctx, cfg)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(p.Close)
		return p
	}
	denied := func(err error) bool {
		var pgErr *pgconn.PgError
		return errors.As(err, &pgErr) && pgErr.Code == "42501"
	}

	allowedDB := asRole(allowed)
	for id, want := range map[uuid.UUID]bool{verified: true, pending: false, plain: false} {
		var got bool
		if err := allowedDB.QueryRow(ctx, `SELECT public.mfa_verified($1)`, id).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("mfa_verified %v want %v", got, want)
		}
	}
	if _, err := allowedDB.Exec(ctx, `SELECT 1 FROM auth.mfa_factors LIMIT 0`); !denied(err) {
		t.Fatalf("allowed role read auth.mfa_factors directly: %v", err)
	}
	if _, err := asRole(other).Exec(ctx, `SELECT public.mfa_verified($1)`, verified); !denied(err) {
		t.Fatalf("other role executed mfa_verified: %v", err)
	}

	// 実行を許したロールの接続でも、二段階認証の強制が外れない（従来どおり判定できる）。
	m := httpx.NewMFAEnrollment(allowedDB)
	if ok, err := m.Enrolled(ctx, verified); !ok || err != nil {
		t.Fatalf("enrolled via function %v %v", ok, err)
	}
	if ok, err := m.Enrolled(ctx, pending); ok || err != nil {
		t.Fatalf("pending via function %v %v", ok, err)
	}
	// 実行を許していないロールでは、従来の「読めない」と同じく確認をやめる（42501）。
	if ok, err := httpx.NewMFAEnrollment(asRole(other)).Enrolled(ctx, verified); ok || err != nil {
		t.Fatalf("other role %v %v", ok, err)
	}
}
