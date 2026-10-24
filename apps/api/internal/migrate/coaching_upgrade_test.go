package migrate_test

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"reflect"
	"testing"

	gomigrate "github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
	appmigrate "github.com/hinata/track-talk/apps/api/internal/migrate"
	"github.com/hinata/track-talk/apps/api/migrations"
	"github.com/jackc/pgx/v5"
)

// 旧データを入れた版92から移行し、旧契約や決済を新しい条件で上書きしないことを確認する。
// 一時DBを作るため、接続先はローカルに限定し、通常のAPIテストのDBと分離する。
func TestCoachingUpgradePreservesLegacyData(t *testing.T) {
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
	name := "coaching_upgrade_" + uuid.New().String()[:8]
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
	// database/sqlドライバーへ渡す接続文字列は、元URLのパスだけを変更する。
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
	_, err = conn.Exec(ctx, `
 INSERT INTO profiles(id,nickname) VALUES
 ('00000000-0000-0000-0000-000000000001','旧コーチ'),
 ('00000000-0000-0000-0000-000000000002','旧利用者');
 INSERT INTO coach_profiles(profile_id,tier,events,best_record,achievements,years,pledged_at,pledge_version)
 VALUES ('00000000-0000-0000-0000-000000000001','senpai',ARRAY['100m'],'10秒','実績',10,now(),1);
 INSERT INTO coaching_plans(id,coach_id,kind,title,description,duration_days,price_yen,capacity,video_reviews,reply_formats)
 VALUES ('00000000-0000-0000-0000-000000000003','00000000-0000-0000-0000-000000000001','period','旧プラン','旧説明',30,8000,5,3,ARRAY['text','video']);
 INSERT INTO coaching_contracts(id,plan_id,coach_id,requester_id,plan_kind,plan_title,duration_days,price_yen,event,goal,current_record,message,minor,status,respond_by,starts_at,ends_at,video_reviews,reply_formats,payment_required)
 VALUES ('00000000-0000-0000-0000-000000000004','00000000-0000-0000-0000-000000000003','00000000-0000-0000-0000-000000000001','00000000-0000-0000-0000-000000000002','period','購入時のプラン',30,7000,'100m','目標','11秒','旧申込',false,'active',now(),now(),now()+interval '30 days',2,ARRAY['text'],true);
 INSERT INTO coaching_messages(id,contract_id,sender_id,sender_role,body)
 VALUES ('00000000-0000-0000-0000-000000000005','00000000-0000-0000-0000-000000000004','00000000-0000-0000-0000-000000000001','coach','移行前の指導');
 INSERT INTO coaching_payments(id,contract_id,payer,amount_yen,checkout_session_id,checkout_url,session_expires_at,payment_intent_id,status,paid_at)
 VALUES ('00000000-0000-0000-0000-000000000006','00000000-0000-0000-0000-000000000004','requester',7000,'cs_legacy','https://example.test',now(),'pi_legacy','paid',now());
 INSERT INTO review_requests(id,requester_id,tier,event,focus,video_public_id,due_at)
 VALUES ('00000000-0000-0000-0000-000000000007','00000000-0000-0000-0000-000000000002','senpai','100m',ARRAY['start'],'legacy-video',now()+interval '2 days');
 INSERT INTO review_payments(id,request_id,payer_id,method,amount_yen,checkout_session_id,checkout_url,session_expires_at,payment_intent_id,status)
 VALUES ('00000000-0000-0000-0000-000000000008','00000000-0000-0000-0000-000000000007','00000000-0000-0000-0000-000000000002','card',1000,'cs_legacy_review','https://example.test',now(),'pi_legacy_review','authorized');
 `)
	if err != nil {
		t.Fatal(err)
	}
	tables := []string{"coach_profiles", "coaching_plans", "coaching_contracts", "coaching_messages", "coaching_payments", "review_requests", "review_payments"}
	before := map[string]map[string]any{}
	for _, table := range tables {
		var raw []byte
		if err := conn.QueryRow(ctx, "SELECT to_jsonb(t) FROM "+table+" t").Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var row map[string]any
		if err := json.Unmarshal(raw, &row); err != nil {
			t.Fatal(err)
		}
		before[table] = row
	}
	if err := appmigrate.Up(migrationURL); err != nil {
		t.Fatal(err)
	}
	if err := appmigrate.Up(migrationURL); err != nil {
		t.Fatalf("repeat upgrade: %v", err)
	}
	for _, table := range tables {
		var raw []byte
		if err := conn.QueryRow(ctx, "SELECT to_jsonb(t) FROM "+table+" t").Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var after map[string]any
		if err := json.Unmarshal(raw, &after); err != nil {
			t.Fatal(err)
		}
		for key, want := range before[table] {
			if key == "tier" {
				want = "coach"
			}
			if !reflect.DeepEqual(after[key], want) {
				t.Errorf("%s.%s changed: %v -> %v", table, key, want, after[key])
			}
		}
	}
	var legacy bool
	if err := conn.QueryRow(ctx, `SELECT service_type IS NULL AND plan_description IS NULL AND additional_consultation_price_yen IS NULL AND fulfillment_version=0 FROM coaching_contracts`).Scan(&legacy); err != nil || !legacy {
		t.Fatalf("legacy contract classification: %v %v", legacy, err)
	}
	var reservations int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM coaching_ticket_reservations`).Scan(&reservations); err != nil || reservations != 0 {
		t.Fatalf("legacy payments converted into tickets: %d %v", reservations, err)
	}
}
