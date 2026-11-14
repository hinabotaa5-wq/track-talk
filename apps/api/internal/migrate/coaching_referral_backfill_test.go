package migrate_test

import (
	"context"
	"net/url"
	"os"
	"testing"

	gomigrate "github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
	appmigrate "github.com/hinata/track-talk/apps/api/internal/migrate"
	"github.com/hinata/track-talk/apps/api/migrations"
	"github.com/jackc/pgx/v5"
)

// §12-2: パック購入がなく、確定済み¥500レビュー支払いだけを持つ利用者にも、
// 移行で issued_by='backfill' の紹介コードが払い出される。
// 既存の TestCoachingUpgradePreservesLegacyData の fixture 利用者はパックと
// レビュー行（1000円/authorized）の両方を持つため、レビュー側 UNION 枝の
// 検証になっていない。ここではレビュー支払いだけの利用者でその枝を確かめる。
func TestCoachingReferralBackfillReviewOnlyPayer(t *testing.T) {
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
	name := "coaching_backfill_" + uuid.New().String()[:8]
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
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	parsedURL, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	parsedURL.Path = "/" + name
	migrationURL := parsedURL.String()
	m, err := gomigrate.NewWithSourceInstance("iofs", src, migrationURL)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Migrate(92); err != nil {
		m.Close()
		t.Fatal(err)
	}
	m.Close()
	// A: 確定済み¥500レビューだけ（パック購入なし）。B: 未確定（authorized）の¥500。
	// C: 確定済みだが金額が¥500ではない。B と C にはコードが出てはならない。
	_, err = conn.Exec(ctx, `
 INSERT INTO profiles(id,nickname) VALUES
 ('00000000-0000-0000-0000-000000000011','レビューのみ'),
 ('00000000-0000-0000-0000-000000000012','未確定レビュー'),
 ('00000000-0000-0000-0000-000000000013','金額違いレビュー');
 INSERT INTO review_payments(id,request_id,payer_id,method,amount_yen,checkout_session_id,checkout_url,session_expires_at,payment_intent_id,status,authorized_at,captured_at)
 VALUES ('00000000-0000-0000-0000-000000000021',NULL,'00000000-0000-0000-0000-000000000011','card',500,'cs_backfill_a','https://example.test',now(),'pi_backfill_a','captured',now(),now()),
 ('00000000-0000-0000-0000-000000000022',NULL,'00000000-0000-0000-0000-000000000012','card',500,'cs_backfill_b','https://example.test',now(),'pi_backfill_b','authorized',now(),NULL),
 ('00000000-0000-0000-0000-000000000023',NULL,'00000000-0000-0000-0000-000000000013','card',600,'cs_backfill_c','https://example.test',now(),'pi_backfill_c','captured',now(),now());
 `)
	if err != nil {
		t.Fatal(err)
	}
	if err := appmigrate.Up(migrationURL); err != nil {
		t.Fatal(err)
	}
	var issuedBy string
	if err := conn.QueryRow(ctx, `SELECT issued_by FROM coaching_referral_codes WHERE owner_id='00000000-0000-0000-0000-000000000011'`).Scan(&issuedBy); err != nil {
		t.Fatalf("review-only payer has no code: %v", err)
	}
	if issuedBy != "backfill" {
		t.Fatalf("issued_by = %q, want backfill", issuedBy)
	}
	for _, id := range []string{"00000000-0000-0000-0000-000000000012", "00000000-0000-0000-0000-000000000013"} {
		var n int
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM coaching_referral_codes WHERE owner_id=$1`, id).Scan(&n); err != nil || n != 0 {
			t.Fatalf("non-qualifying payer %s has %d codes: %v", id, n, err)
		}
	}
}
