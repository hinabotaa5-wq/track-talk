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

// #726: 版172で精算記録4表の contract_id 外部キーが ON DELETE SET NULL になり、版171に戻すと NO ACTION と
// 元の NOT NULL・主キーに戻ること、もう一度上げられること、契約から外れた行があれば戻さないことを確かめる。
func TestSupportSettlementDetachMigrationRoundTrip(t *testing.T) {
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
	name := "support_detach_" + uuid.New().String()[:8]
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
	// 4表の contract_id 外部キーの削除時の動き、contract_id の NOT NULL、主キーの列を1行にまとめる。
	shape := func() string {
		c := conn()
		defer c.Close(ctx)
		var s string
		if err := c.QueryRow(ctx, `SELECT string_agg(format('%s:%s:%s:%s', t,
				(SELECT confdeltype FROM pg_constraint WHERE conrelid = t::regclass AND conname = t || '_contract_id_fkey'),
				(SELECT attnotnull FROM pg_attribute WHERE attrelid = t::regclass AND attname = 'contract_id'),
				(SELECT string_agg(a.attname, ',') FROM pg_constraint k JOIN pg_attribute a ON a.attrelid = k.conrelid AND a.attnum = ANY(k.conkey)
					WHERE k.conrelid = t::regclass AND k.contype = 'p')), ' ' ORDER BY t)
			FROM unnest(ARRAY['support_coach_advances','support_coach_settlements','support_fault_determinations','coach_refund_adjustments']) t`).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	const before = "coach_refund_adjustments:a:f:id support_coach_advances:a:t:earning_id support_coach_settlements:a:t:contract_id support_fault_determinations:a:t:contract_id"
	// support_coach_advances は主キーが earning_id のままで、NOT NULL だけが外れる。
	const after = "coach_refund_adjustments:n:f:id support_coach_advances:n:f:earning_id support_coach_settlements:n:f:id support_fault_determinations:n:f:id"

	if err := migrateTo(171); err != nil {
		t.Fatal(err)
	}
	if got := shape(); got != before {
		t.Fatalf("171 shape\n got %s\nwant %s", got, before)
	}
	if err := migrateTo(172); err != nil {
		t.Fatal(err)
	}
	if got := shape(); got != after {
		t.Fatalf("172 shape\n got %s\nwant %s", got, after)
	}
	// 契約から外れた精算記録が1件でもあれば、版171へは戻さない（記録を消さない）。
	c := conn()
	if _, err := c.Exec(ctx, `INSERT INTO support_coach_settlements(contract_id, earned_yen, advances_yen, final_yen) VALUES (NULL, 100, 0, 100)`); err != nil {
		c.Close(ctx)
		t.Fatalf("insert detached settlement: %v", err)
	}
	c.Close(ctx)
	if err := migrateTo(171); err == nil || !strings.Contains(err.Error(), "detached") {
		t.Fatalf("down with detached records: %v", err)
	}
	// 失敗した down は dirty で止まる。記録を片付けて版172に force し直してから、やり直す。
	c = conn()
	if _, err := c.Exec(ctx, `DELETE FROM support_coach_settlements WHERE contract_id IS NULL; UPDATE schema_migrations SET version = 172, dirty = false`); err != nil {
		c.Close(ctx)
		t.Fatal(err)
	}
	c.Close(ctx)
	if got := shape(); got != after {
		t.Fatalf("shape after failed down\n got %s\nwant %s", got, after)
	}
	if err := migrateTo(171); err != nil {
		t.Fatal(err)
	}
	if got := shape(); got != before {
		t.Fatalf("down shape\n got %s\nwant %s", got, before)
	}
	if err := migrateTo(172); err != nil {
		t.Fatal(err)
	}
	if got := shape(); got != after {
		t.Fatalf("re-up shape\n got %s\nwant %s", got, after)
	}
}
