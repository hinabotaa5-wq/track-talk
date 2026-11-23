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

// 本体 #981（T8）：版190で対象4テーブルに契約終了日の控え（contract_end_on）が足され、残っている契約の終了日が
// backfill され、契約の行が消える時に書き写されること。版189へ戻すと列と仕組みだけが外れ、記録は残ること。
func TestSupportRecordContractEndOnMigrationRoundTrip(t *testing.T) {
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
	name := "record_end_on_" + uuid.New().String()[:8]
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
	c, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close(ctx)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := c.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%v\n%s", err, sql)
		}
	}
	columns := func() int {
		t.Helper()
		var n int
		if err := c.QueryRow(ctx, `SELECT count(*)::int FROM information_schema.columns WHERE column_name='contract_end_on'
			AND table_name IN ('support_contract_statements','support_coach_settlements','support_coach_advances','support_fault_determinations')`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	triggers := func() int {
		t.Helper()
		var n int
		if err := c.QueryRow(ctx, `SELECT count(*)::int FROM pg_trigger WHERE tgname='copy_support_record_contract_end_on' AND NOT tgisinternal`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	// 4テーブルの、契約 contract に付いた記録の控え（NULL は 'NULL'）。detached は contract_id が NULL の記録。
	endOns := func(where string, args ...any) []string {
		t.Helper()
		var got []string
		if err := c.QueryRow(ctx, `SELECT array_agg(COALESCE(v::text,'NULL') ORDER BY t) FROM (
			SELECT 1 t, contract_end_on v FROM support_contract_statements WHERE `+where+`
			UNION ALL SELECT 2, contract_end_on FROM support_coach_settlements WHERE `+where+`
			UNION ALL SELECT 3, contract_end_on FROM support_coach_advances WHERE `+where+`
			UNION ALL SELECT 4, contract_end_on FROM support_fault_determinations WHERE `+where+`) x`, args...).Scan(&got); err != nil {
			t.Fatal(err)
		}
		return got
	}
	same := func(label string, got []string, want string) {
		t.Helper()
		if len(got) != 4 {
			t.Fatalf("%s: %v", label, got)
		}
		for _, v := range got {
			if v != want {
				t.Fatalf("%s: %v, want all %s", label, got, want)
			}
		}
	}

	if err := migrateTo(189); err != nil {
		t.Fatal(err)
	}
	if columns() != 0 || triggers() != 0 {
		t.Fatal("189 already has the end-date copy")
	}
	// 版189の記録を、外部キーを素通りして作る（契約の中身は終了日の列だけを見る）。
	ended, extended, open := uuid.New(), uuid.New(), uuid.New()
	exec(`SET session_replication_role=replica`)
	for _, ct := range []struct {
		id         uuid.UUID
		endsAt     any
		supportEnd any
	}{
		// ends_at は 2028-05-01 00:00 JST（UTC では 4/30）。日本時間の日付で控える。
		{ended, "2028-04-30T15:00:00Z", nil},
		// 途中終了の日割りの終わりの日が ends_at より遅いときは、そちらを控える。
		{extended, "2028-03-01T00:00:00+09:00", "2028-03-05"},
		// 終了日が決まっていない契約は NULL のまま。
		{open, nil, nil},
	} {
		exec(`INSERT INTO coaching_contracts(id,plan_id,coach_id,requester_id,plan_kind,plan_title,duration_days,price_yen,event,goal,current_record,message,minor,respond_by,status,ends_at,support_end_cause,support_end_on)
			VALUES($1,gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),'period','plan',30,9000,'100m','goal','12','hello',false,now(),'cancelled',$2::timestamptz,CASE WHEN $3::date IS NULL THEN NULL ELSE 'player' END,$3::date)`, ct.id, ct.endsAt, ct.supportEnd)
	}
	for _, contract := range []any{ended, extended, open, nil} {
		exec(`INSERT INTO support_contract_statements(contract_id,body) VALUES($1,'{}')`, contract)
		exec(`INSERT INTO support_coach_settlements(contract_id,earned_yen,advances_yen,final_yen) VALUES($1,0,0,0)`, contract)
		earning := uuid.New()
		exec(`INSERT INTO coach_earnings(id,coach_id,source,gross_yen,share_percent,amount_yen,confirmed_at) VALUES($1,gen_random_uuid(),'review',100,75,75,now())`, earning)
		exec(`INSERT INTO support_coach_advances(earning_id,contract_id,through_date,elapsed_days,total_days,amount_yen) VALUES($1,$2,'2028-03-31',1,30,100)`, earning, contract)
		exec(`INSERT INTO support_fault_determinations(contract_id,decision,reason) VALUES($1,'unavoidable','理由')`, contract)
	}
	exec(`SET session_replication_role=origin`)

	if err := migrateTo(190); err != nil {
		t.Fatal(err)
	}
	if columns() != 4 || triggers() != 1 {
		t.Fatalf("190 columns %d triggers %d", columns(), triggers())
	}
	same("backfill ended", endOns(`contract_id=$1`, ended), "2028-05-01")
	same("backfill extended", endOns(`contract_id=$1`, extended), "2028-03-05")
	same("backfill open", endOns(`contract_id=$1`, open), "NULL")
	same("backfill detached", endOns(`contract_id IS NULL`), "NULL")

	// 契約の行が消える時に、その時点の終了日を書き写す（backfill の後で終了日が後ろ倒しになっていれば、それに追随する）。
	exec(`UPDATE coaching_contracts SET ends_at = ends_at + interval '7 days' WHERE id=$1`, ended)
	exec(`DELETE FROM coaching_contracts WHERE id=$1`, ended)
	same("copied on delete", endOns(`contract_id IS NULL AND contract_end_on IS NOT NULL`), "2028-05-08")
	// 終了日が決まらないまま消えたら、消えた日（日本時間）を書き写す。
	var today string
	if err := c.QueryRow(ctx, `SELECT (now() AT TIME ZONE 'Asia/Tokyo')::date::text`).Scan(&today); err != nil {
		t.Fatal(err)
	}
	exec(`DELETE FROM coaching_contracts WHERE id=$1`, open)
	same("copied undetermined", endOns(`contract_id IS NULL AND contract_end_on=$1::date`, today), today)

	var rows int
	countRows := func() int {
		t.Helper()
		if err := c.QueryRow(ctx, `SELECT (SELECT count(*) FROM support_contract_statements) + (SELECT count(*) FROM support_coach_settlements)
			+ (SELECT count(*) FROM support_coach_advances) + (SELECT count(*) FROM support_fault_determinations)`).Scan(&rows); err != nil {
			t.Fatal(err)
		}
		return rows
	}
	before := countRows()
	if before != 16 {
		t.Fatalf("rows %d", before)
	}
	// 戻すと列と書き写しの仕組みだけが外れ、記録は残る（契約から外れた記録があっても戻せる）。
	if err := migrateTo(189); err != nil {
		t.Fatal(err)
	}
	if columns() != 0 || triggers() != 0 || countRows() != before {
		t.Fatalf("down: columns %d triggers %d rows %d", columns(), triggers(), countRows())
	}
	exec(`DELETE FROM coaching_contracts WHERE id=$1`, extended)
	if err := migrateTo(190); err != nil {
		t.Fatal(err)
	}
	if columns() != 4 || triggers() != 1 || countRows() != before {
		t.Fatalf("re-up: columns %d triggers %d rows %d", columns(), triggers(), countRows())
	}
}
