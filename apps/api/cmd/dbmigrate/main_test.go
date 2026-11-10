package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"os"
	"strings"
	"testing"

	gomigrate "github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/hinata/track-talk/apps/api/migrations"
)

func adminDSN(t *testing.T) string {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://tracktalk:tracktalk@127.0.0.1:55432/tracktalk?sslmode=disable"
	}
	if err := config.RequireLocalDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	return dsn
}

// isolatedDB は、試験専用の DB を作り、指定の版まで migration を当てた接続文字列を返す。
func isolatedDB(t *testing.T, version uint) (string, *pgx.Conn) {
	t.Helper()
	ctx := context.Background()
	dsn := adminDSN(t)
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	t.Cleanup(func() { admin.Close(context.Background()) })
	name := "dbmigrate_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	isolated := u.String()
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	m, err := gomigrate.NewWithSourceInstance("iofs", src, isolated)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Migrate(version); err != nil {
		t.Fatal(err)
	}
	m.Close()
	conn, err := pgx.Connect(ctx, isolated)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		conn.Close(context.Background())
		_, _ = admin.Exec(context.Background(), "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
	})
	return isolated, conn
}

func newRole(t *testing.T, conn *pgx.Conn, prefix, attrs string) string {
	t.Helper()
	ctx := context.Background()
	role := prefix + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	if _, err := conn.Exec(ctx, "CREATE ROLE "+role+" "+attrs); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = conn.Exec(context.Background(), "DROP OWNED BY "+role)
		_, _ = conn.Exec(context.Background(), "DROP ROLE "+role)
	})
	return role
}

func decode(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("output is not one JSON line: %v", err)
	}
	return out
}

// 版153の本番相当（本体読み取りロールに mark_retired の実行権がある）から、check→apply→再 apply を通す。
func TestApplyIsIdempotentAndGrantsOnlyBoundaryReader(t *testing.T) {
	ctx := context.Background()
	dsn, conn := isolatedDB(t, 153)
	if _, err := conn.Exec(ctx, `
		CREATE SCHEMA IF NOT EXISTS auth;
		CREATE TABLE IF NOT EXISTS auth.users (id uuid PRIMARY KEY, email text, deleted_at timestamptz)`); err != nil {
		t.Fatal(err)
	}
	reader := newRole(t, conn, "boundary_reader_", "LOGIN")
	other := newRole(t, conn, "other_login_", "LOGIN")
	if _, err := conn.Exec(ctx, "GRANT USAGE ON SCHEMA identity_boundary TO "+reader+
		"; GRANT EXECUTE ON FUNCTION identity_boundary.mark_retired(uuid) TO "+reader); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if code := run("check", dsn, &buf); code != 0 {
		t.Fatalf("check exit %d: %s", code, buf.String())
	}
	out := decode(t, &buf)
	if out["before_version"] != float64(153) || out["pending"] != float64(1) || out["result"] != "ok" {
		t.Fatalf("check output %v", out)
	}
	var version int
	if err := conn.QueryRow(ctx, "SELECT version FROM schema_migrations").Scan(&version); err != nil || version != 153 {
		t.Fatalf("check must not apply: %d %v", version, err)
	}

	buf.Reset()
	if code := run("apply", dsn, &buf); code != 0 {
		t.Fatalf("apply exit %d: %s", code, buf.String())
	}
	out = decode(t, &buf)
	if out["after_version"] != float64(154) || out["after_dirty"] != false || out["applied"] != float64(1) || out["result"] != "ok" {
		t.Fatalf("apply output %v", out)
	}
	checks := out["checks"].(map[string]any)
	for key, want := range map[string]any{
		"coach_matches_email_definer": true, "coach_matches_email_search_path_fixed": true,
		"coach_matches_email_owner_matches_boundary": true, "api_roles_can_execute": float64(0),
		"synthetic_coach_id_definer": true, "synthetic_coach_id_search_path_fixed": true,
		"public_can_execute": false, "boundary_reader_roles": float64(1), "boundary_readers_can_execute": float64(1),
	} {
		if checks[key] != want {
			t.Errorf("check %s = %v, want %v", key, checks[key], want)
		}
	}
	// 出力に接続文字列の要素（ユーザー名・パスワード・ホスト・DB 名）が出ない。
	u, _ := url.Parse(dsn)
	pw, _ := u.User.Password()
	for _, secret := range []string{pw, u.User.Username(), strings.TrimPrefix(u.Path, "/"), "postgres://"} {
		if secret != "" && strings.Contains(buf.String(), secret) {
			t.Fatalf("output leaks connection detail")
		}
	}

	buf.Reset()
	if code := run("apply", dsn, &buf); code != 0 {
		t.Fatalf("repeat apply exit %d: %s", code, buf.String())
	}
	out = decode(t, &buf)
	if out["applied"] != float64(0) || out["after_version"] != float64(154) {
		t.Fatalf("repeat apply output %v", out)
	}

	var readerCan, otherCan bool
	if err := conn.QueryRow(ctx, `SELECT has_function_privilege($1, 'identity_boundary.coach_matches_email(uuid, text)', 'EXECUTE'),
		has_function_privilege($2, 'identity_boundary.coach_matches_email(uuid, text)', 'EXECUTE')`, reader, other).Scan(&readerCan, &otherCan); err != nil {
		t.Fatal(err)
	}
	if !readerCan || otherCan {
		t.Fatalf("reader %v other %v", readerCan, otherCan)
	}
}

// メール一致関数は、承認済みかつメール一致（大文字小文字・前後の空白は区別しない）のときだけ true を返す。
func TestCoachMatchesEmail(t *testing.T) {
	ctx := context.Background()
	dsn, conn := isolatedDB(t, 153)
	if _, err := conn.Exec(ctx, `
		CREATE SCHEMA IF NOT EXISTS auth;
		CREATE TABLE IF NOT EXISTS auth.users (id uuid PRIMARY KEY, email text, deleted_at timestamptz)`); err != nil {
		t.Fatal(err)
	}
	reader := newRole(t, conn, "boundary_reader_", "LOGIN")
	if _, err := conn.Exec(ctx, "GRANT USAGE ON SCHEMA identity_boundary TO "+reader+
		"; GRANT EXECUTE ON FUNCTION identity_boundary.mark_retired(uuid) TO "+reader); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if code := run("apply", dsn, &buf); code != 0 {
		t.Fatalf("apply exit %d: %s", code, buf.String())
	}
	approved, pending, deleted, plain := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	for i, c := range []struct {
		id     uuid.UUID
		status string
		email  string
		gone   bool
	}{{approved, "approved", "ops+synthetic-a@example.test", false}, {pending, "pending", "ops+synthetic-b@example.test", false}, {deleted, "approved", "ops+synthetic-c@example.test", true}, {plain, "approved", "plain.coach@example.test", false}} {
		if _, err := conn.Exec(ctx, `INSERT INTO profiles (id, nickname) VALUES ($1, $2)`, c.id, "合成"+string(rune('A'+i))); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, `INSERT INTO coach_profiles (profile_id, tier, events, best_record, achievements, years, status, reviewed_at)
			VALUES ($1, 'coach', ARRAY['100m'], '-', '-', 1, $2, CASE WHEN $2 = 'pending' THEN NULL ELSE now() END)`, c.id, c.status); err != nil {
			t.Fatal(err)
		}
		var gone any
		if c.gone {
			gone = "2026-11-10T00:00:00Z"
		}
		if _, err := conn.Exec(ctx, `INSERT INTO auth.users (id, email, deleted_at) VALUES ($1, $2, $3)`, c.id, c.email, gone); err != nil {
			t.Fatal(err)
		}
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SET LOCAL ROLE "+reader); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		coach any
		email any
		want  bool
	}{
		{approved, "ops+synthetic-a@example.test", true},
		{approved, "  OPS+Synthetic-A@Example.test ", true},
		{approved, "ops+synthetic-b@example.test", false},
		{approved, "", false},
		{approved, nil, false},
		{nil, "ops+synthetic-a@example.test", false},
		{pending, "ops+synthetic-b@example.test", false},
		{deleted, "ops+synthetic-c@example.test", false},
		{uuid.New(), "ops+synthetic-a@example.test", false},
	} {
		var got bool
		if err := tx.QueryRow(ctx, `SELECT identity_boundary.coach_matches_email($1::uuid, $2::text)`, c.coach, c.email).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != c.want {
			t.Errorf("coach_matches_email(%v, %v) = %v, want %v", c.coach, c.email, got, c.want)
		}
	}
	// 合成コーチの ID は、合成の標識があり承認済みでメールが一致するときだけ返る。
	for _, c := range []struct {
		email any
		want  *uuid.UUID
	}{
		{"ops+synthetic-a@example.test", &approved},
		{" OPS+Synthetic-A@example.test", &approved},
		{"ops+synthetic-b@example.test", nil},
		{"ops+synthetic-c@example.test", nil},
		{"ops@example.test", nil},
		{"plain.coach@example.test", nil}, // 承認済み・一致でも、合成の標識が無ければ返さない
		{nil, nil},
	} {
		var got *uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT identity_boundary.synthetic_coach_id($1::text)`, c.email).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if (got == nil) != (c.want == nil) || (got != nil && *got != *c.want) {
			t.Errorf("synthetic_coach_id(%v) = %v, want %v", c.email, got, c.want)
		}
	}
	// 読み取りロールのままでは auth.users を直接読めない（関数経由の真偽だけ）。
	if _, err := tx.Exec(ctx, "SAVEPOINT direct"); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, "SELECT email FROM auth.users"); err == nil {
		t.Fatal("reader can read auth.users directly")
	}
}

// 実行前が dirty なら何も当てず、版を変えずに止まる。
func TestApplyRefusesDirty(t *testing.T) {
	ctx := context.Background()
	dsn, conn := isolatedDB(t, 153)
	if _, err := conn.Exec(ctx, "UPDATE schema_migrations SET dirty = true"); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if code := run("apply", dsn, &buf); code != 3 {
		t.Fatalf("exit %d: %s", code, buf.String())
	}
	out := decode(t, &buf)
	if out["error"] != "dirty_before_apply" || out["after_version"] != float64(153) {
		t.Fatalf("output %v", out)
	}
	var exists bool
	if err := conn.QueryRow(ctx, "SELECT to_regprocedure('identity_boundary.coach_matches_email(uuid, text)') IS NOT NULL").Scan(&exists); err != nil || exists {
		t.Fatalf("applied despite dirty: %v %v", exists, err)
	}
}

// 本体読み取りロールが無い（付与先が0件）なら、適用後の確認で失敗として止まる。
func TestApplyFailsWithoutBoundaryReader(t *testing.T) {
	ctx := context.Background()
	dsn, conn := isolatedDB(t, 153)
	if _, err := conn.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS auth; CREATE TABLE IF NOT EXISTS auth.users (id uuid PRIMARY KEY, email text, deleted_at timestamptz)`); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if code := run("apply", dsn, &buf); code != 1 {
		t.Fatalf("exit %d: %s", code, buf.String())
	}
	if out := decode(t, &buf); out["error"] != "post_check_mismatch" {
		t.Fatalf("output %v", out)
	}
}

func TestCheckURL(t *testing.T) {
	for raw, want := range map[string]string{
		"":                            "database_url_missing",
		"mysql://u:p@db.example/x":    "invalid_database_configuration",
		"postgres://u:p@db.example/x": "encrypted_database_connection_required",
		"postgres://u:p@db.example/x?sslmode=disable": "encrypted_database_connection_required",
		"postgres://u:p@db.example/x?sslmode=require": "",
		"postgres://u:p@127.0.0.1:5432/x":             "",
	} {
		err := checkURL(raw)
		got := ""
		if err != nil {
			got = err.Error()
		}
		if got != want {
			t.Errorf("checkURL(%q) = %q, want %q", raw, got, want)
		}
	}
	var buf bytes.Buffer
	if code := run("drop", "postgres://u:p@127.0.0.1/x", &buf); code != 2 || strings.Contains(buf.String(), "u:p") {
		t.Fatalf("unknown mode: %d %s", code, buf.String())
	}
}
