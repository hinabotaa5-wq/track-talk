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
	"github.com/hinata/track-talk/apps/api/migrations"
	"github.com/jackc/pgx/v5"
)

// 本体 #686 受け入れ 5：000170 の時点で進行中の周は旧ルールのまま（起点を問わず）引き継ぎ、
// 付与済みの周は終了扱いになる。2周目のIDがまだなければ 000169 へ戻せる。
func TestCoachingReferralCyclesMigration(t *testing.T) {
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
	name := "coaching_cycles_" + uuid.New().String()[:8]
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
	m, err := gomigrate.NewWithSourceInstance("iofs", src, parsedURL.String())
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if err := m.Migrate(169); err != nil {
		t.Fatal(err)
	}
	// A：友達2人で付与済み。B：友達1人で進行中。C〜E：友達（パック購入）。
	_, err = conn.Exec(ctx, `
 INSERT INTO profiles(id,nickname) VALUES
 ('00000000-0000-0000-0000-00000000000a','A'),('00000000-0000-0000-0000-00000000000b','B'),
 ('00000000-0000-0000-0000-00000000000c','C'),('00000000-0000-0000-0000-00000000000d','D'),
 ('00000000-0000-0000-0000-00000000000e','E');
 SELECT coaching_issue_referral('00000000-0000-0000-0000-00000000000a', 'purchase');
 SELECT coaching_issue_referral('00000000-0000-0000-0000-00000000000b', 'purchase');
 INSERT INTO coaching_ticket_purchases(id,owner_id,request_key,product_id,quantity,amount_yen,status,session_expires_at,return_path,payment_intent_id,paid_at)
 SELECT ('00000000-0000-0000-0000-0000000001'||n)::uuid, ('00000000-0000-0000-0000-00000000000'||b)::uuid, gen_random_uuid(),'six',6,2500,'paid',now(),'/coaching/new','pi_cycle_'||n,now()
 FROM (VALUES ('01','c'),('02','d'),('03','e')) v(n,b);
 SELECT coaching_count_referral('00000000-0000-0000-0000-00000000000c', (SELECT code FROM coaching_referral_codes WHERE owner_id='00000000-0000-0000-0000-00000000000a'), '00000000-0000-0000-0000-000000000101', NULL);
 SELECT coaching_count_referral('00000000-0000-0000-0000-00000000000d', (SELECT code FROM coaching_referral_codes WHERE owner_id='00000000-0000-0000-0000-00000000000a'), '00000000-0000-0000-0000-000000000102', NULL);
 SELECT coaching_count_referral('00000000-0000-0000-0000-00000000000e', (SELECT code FROM coaching_referral_codes WHERE owner_id='00000000-0000-0000-0000-00000000000b'), '00000000-0000-0000-0000-000000000103', NULL);
 `)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Migrate(170); err != nil {
		t.Fatal(err)
	}
	var aClosed, aAtGrant, bOpen, bLegacy bool
	var bCount int
	if err := conn.QueryRow(ctx, `SELECT c.closed_at IS NOT NULL, c.closed_at = r.granted_at FROM coaching_referral_codes c
		JOIN coaching_referral_rewards r ON r.referral_code = c.code WHERE c.owner_id='00000000-0000-0000-0000-00000000000a'`).Scan(&aClosed, &aAtGrant); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(ctx, `SELECT c.closed_at IS NULL, num_nonnulls(c.origin_ticket_purchase_id, c.origin_review_payment_id) = 0,
		(SELECT count(*) FROM coaching_referral_counts a WHERE a.referral_code = c.code AND a.canceled_at IS NULL)
		FROM coaching_referral_codes c WHERE c.owner_id='00000000-0000-0000-0000-00000000000b'`).Scan(&bOpen, &bLegacy, &bCount); err != nil {
		t.Fatal(err)
	}
	if !aClosed || !aAtGrant || !bOpen || !bLegacy || bCount != 1 {
		t.Fatalf("A closed=%v at grant=%v, B open=%v legacy=%v count=%d", aClosed, aAtGrant, bOpen, bLegacy, bCount)
	}
	// 移行対象の周は起点を問わずに達成できる（C は A の友達だったが、B へは新しい購入者）。
	if _, err := conn.Exec(ctx, `
 INSERT INTO coaching_ticket_purchases(id,owner_id,request_key,product_id,quantity,amount_yen,status,session_expires_at,return_path,payment_intent_id,paid_at)
 VALUES ('00000000-0000-0000-0000-000000000104','00000000-0000-0000-0000-00000000000c',gen_random_uuid(),'six',6,2500,'paid',now(),'/coaching/new','pi_cycle_04',now())`); err != nil {
		t.Fatal(err)
	}
	var recipient *uuid.UUID
	if err := conn.QueryRow(ctx, `SELECT coaching_record_referral('00000000-0000-0000-0000-00000000000c',
		(SELECT code FROM coaching_referral_codes WHERE owner_id='00000000-0000-0000-0000-00000000000b'),
		'00000000-0000-0000-0000-000000000104', NULL)`).Scan(&recipient); err != nil {
		t.Fatal(err)
	}
	if recipient == nil || recipient.String() != "00000000-0000-0000-0000-00000000000b" {
		t.Fatalf("legacy cycle was not granted: %v", recipient)
	}
	if err := m.Migrate(169); err != nil {
		t.Fatalf("down to 169: %v", err)
	}
}
