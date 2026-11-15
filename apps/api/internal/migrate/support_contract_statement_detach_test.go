package migrate_test

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"

	gomigrate "github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/hinata/track-talk/apps/api/migrations"
	"github.com/jackc/pgx/v5"
)

// #748: 版174で support_contract_statements の contract_id 外部キーが ON DELETE SET NULL になり、
// 版173に戻すと CASCADE と元の NOT NULL・主キーに戻ること、もう一度上げられること、
// 契約から外れた行があれば戻さないことを確かめる（migration 000172 の往復テストと同型）。
func TestContractStatementDetachMigrationRoundTrip(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://tracktalk:tracktalk@127.0.0.1:55432/tracktalk?sslmode=disable"
	}
	if err := config.RequireLocalDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	defer admin.Close(ctx)
	name := "statement_detach_" + uuid.New().String()[:8]
	ident := pgx.Identifier{name}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+ident); err != nil {
		t.Fatalf("create isolated test database: %v", err)
	}
	defer func() {
		if _, err := admin.Exec(ctx, "DROP DATABASE "+ident+" WITH (FORCE)"); err != nil {
			t.Errorf("drop test database: %v", err)
		}
	}()
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Database = name
	parsedURL, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	parsedURL.Path = "/" + name
	migrationURL := parsedURL.String()
	migrateTo := func(version uint) error {
		src, err := iofs.New(migrations.FS, ".")
		if err != nil {
			t.Fatal(err)
		}
		m, err := gomigrate.NewWithSourceInstance("iofs", src, migrationURL)
		if err != nil {
			t.Fatal(err)
		}
		defer m.Close()
		return m.Migrate(version)
	}
	conn := func() *pgx.Conn {
		c, err := pgx.ConnectConfig(ctx, cfg)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	// 削除時の動き:confdeltype（c=CASCADE・n=SET NULL）、contract_id の NOT NULL、主キーの列、
	// UNIQUE (contract_id) の有無を1行にまとめる。
	shape := func() string {
		c := conn()
		defer c.Close(ctx)
		var s string
		if err := c.QueryRow(ctx, `SELECT format('%s:%s:%s:%s',
			(SELECT confdeltype FROM pg_constraint WHERE conrelid = 'support_contract_statements'::regclass AND conname = 'support_contract_statements_contract_id_fkey'),
			(SELECT attnotnull FROM pg_attribute WHERE attrelid = 'support_contract_statements'::regclass AND attname = 'contract_id'),
			(SELECT string_agg(a.attname, ',') FROM pg_constraint k JOIN pg_attribute a ON a.attrelid = k.conrelid AND a.attnum = ANY(k.conkey)
				WHERE k.conrelid = 'support_contract_statements'::regclass AND k.contype = 'p'),
			(SELECT count(*)::int FROM pg_constraint WHERE conrelid = 'support_contract_statements'::regclass AND contype = 'u'))`).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	const before = "c:t:contract_id:0"
	const after = "n:f:id:1"

	if err := migrateTo(173); err != nil {
		t.Fatal(err)
	}
	if got := shape(); got != before {
		t.Fatalf("173 shape\n got %s\nwant %s", got, before)
	}
	if err := migrateTo(174); err != nil {
		t.Fatal(err)
	}
	if got := shape(); got != after {
		t.Fatalf("174 shape\n got %s\nwant %s", got, after)
	}
	// 契約から外れた取引条件の明示が1件でもあれば、版173へは戻さない（証跡を消さない）。
	c := conn()
	if _, err := c.Exec(ctx, `INSERT INTO support_contract_statements(contract_id, coach_id, body) VALUES (NULL, NULL, '{}')`); err != nil {
		c.Close(ctx)
		t.Fatalf("insert detached statement: %v", err)
	}
	c.Close(ctx)
	if err := migrateTo(173); err == nil || !strings.Contains(err.Error(), "detached") {
		t.Fatalf("down with detached records: %v", err)
	}
	// 失敗した down は dirty で止まる。記録を片付けて版174に force し直してから、やり直す。
	c = conn()
	if _, err := c.Exec(ctx, `DELETE FROM support_contract_statements WHERE contract_id IS NULL; UPDATE schema_migrations SET version = 174, dirty = false`); err != nil {
		c.Close(ctx)
		t.Fatal(err)
	}
	c.Close(ctx)
	if got := shape(); got != after {
		t.Fatalf("shape after failed down\n got %s\nwant %s", got, after)
	}
	if err := migrateTo(173); err != nil {
		t.Fatal(err)
	}
	if got := shape(); got != before {
		t.Fatalf("down shape\n got %s\nwant %s", got, before)
	}
	if err := migrateTo(174); err != nil {
		t.Fatal(err)
	}
	if got := shape(); got != after {
		t.Fatalf("re-up shape\n got %s\nwant %s", got, after)
	}
}
