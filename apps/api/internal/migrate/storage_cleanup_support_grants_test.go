package migrate_test

import (
	"context"
	"errors"
	"net/url"
	"os"
	"testing"
	"time"

	gomigrate "github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// 本体 #1103：版200で、公開 API の実行ロール（track_talk_api）が、日次の storage cleanup の support 系削除（DELETE）と、
// 契約の行が消える時の契約終了日の書き写しトリガ（000190、呼んだロールの権限で4表を UPDATE）を実行できること。
// 版199では本番と同じく permission denied（42501）になること（障害の再現）。ロールの無い環境でも up・down が通ること。
func TestStorageCleanupSupportGrantsMigration(t *testing.T) {
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
	name := "cleanup_grants_" + uuid.New().String()[:8]
	ident := pgx.Identifier{name}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+ident); err != nil {
		t.Fatalf("create isolated test database: %v", err)
	}
	const role = "track_talk_api"
	var roleExisted bool
	if err := admin.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname=$1)`, role).Scan(&roleExisted); err != nil {
		t.Fatal(err)
	}
	createdRole := false
	defer func() {
		if _, err := admin.Exec(ctx, "DROP DATABASE "+ident+" WITH (FORCE)"); err != nil {
			t.Errorf("drop test database: %v", err)
		}
		if createdRole {
			if _, err := admin.Exec(ctx, "DROP ROLE "+role); err != nil {
				t.Errorf("drop role: %v", err)
			}
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
	migrateTo := func(version uint) {
		t.Helper()
		src, err := iofs.New(migrations.FS, ".")
		if err != nil {
			t.Fatal(err)
		}
		m, err := gomigrate.NewWithSourceInstance("iofs", src, migrationURL)
		if err != nil {
			t.Fatal(err)
		}
		defer m.Close()
		if err := m.Migrate(version); err != nil && !errors.Is(err, gomigrate.ErrNoChange) {
			t.Fatalf("migrate to %d: %v", version, err)
		}
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

	// T3（ロール不在）：ロールの無い環境でも up・down・up が通る。
	if !roleExisted {
		migrateTo(200)
		migrateTo(199)
	} else {
		migrateTo(199)
	}

	if !roleExisted {
		// 本番の実行ロールは RLS を有効にした（方針0件の）表も読み書きしているので、BYPASSRLS 付きで作る。
		if _, err := admin.Exec(ctx, "CREATE ROLE "+role+" BYPASSRLS"); err != nil {
			t.Fatal(err)
		}
		createdRole = true
	}
	// 本番の実行ロールが以前から持っている権限（読み書き・帰責判定の異議と見直し・契約の削除）を、試験用の DB で再現する。
	exec(`GRANT USAGE ON SCHEMA public TO ` + role)
	exec(`GRANT SELECT, INSERT ON support_contract_statements, support_coach_settlements, support_coach_advances, support_fault_determinations TO ` + role)
	exec(`GRANT UPDATE ON support_fault_determinations TO ` + role)
	exec(`GRANT SELECT, DELETE ON coaching_contracts TO ` + role)

	// 契約の消えた古い記録（cleanup の削除対象）と、まだ契約に付いている記録を作る。
	seed := func() uuid.UUID {
		t.Helper()
		contract := uuid.New()
		exec(`SET session_replication_role=replica`)
		exec(`INSERT INTO coaching_contracts(id,plan_id,coach_id,requester_id,plan_kind,plan_title,duration_days,price_yen,event,goal,current_record,message,minor,respond_by,status,ends_at)
			VALUES($1,gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),'period','plan',30,9000,'100m','goal','12','hello',false,now(),'cancelled','2010-01-31T00:00:00+09:00')`, contract)
		for _, ct := range []any{contract, nil} {
			exec(`INSERT INTO support_contract_statements(contract_id,body,issued_at,contract_end_on) VALUES($1,'{}','2010-01-01T00:00:00+09:00','2010-01-31')`, ct)
			exec(`INSERT INTO support_coach_settlements(contract_id,earned_yen,advances_yen,final_yen,created_at,contract_end_on) VALUES($1,0,0,0,'2010-01-01T00:00:00+09:00','2010-01-31')`, ct)
			earning := uuid.New()
			exec(`INSERT INTO coach_earnings(id,coach_id,source,gross_yen,share_percent,amount_yen,confirmed_at) VALUES($1,gen_random_uuid(),'review',100,75,75,now())`, earning)
			exec(`INSERT INTO support_coach_advances(earning_id,contract_id,through_date,elapsed_days,total_days,amount_yen,created_at,contract_end_on) VALUES($1,$2,'2010-01-31',1,30,100,'2010-01-01T00:00:00+09:00','2010-01-31')`, earning, ct)
			exec(`INSERT INTO support_fault_determinations(contract_id,decision,reason,notified_at,contract_end_on) VALUES($1,'unavoidable','理由','2010-01-01T00:00:00+09:00','2010-01-31')`, ct)
		}
		exec(`SET session_replication_role=origin`)
		return contract
	}

	// 実行ロールの接続（本番の API と同じ権限で SQL を流す）。
	asAPI := func() *pgx.Conn {
		t.Helper()
		rc := cfg.Copy()
		conn, err := pgx.ConnectConfig(ctx, rc)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, "SET ROLE "+role); err != nil {
			t.Fatal(err)
		}
		return conn
	}
	denied := func(err error) bool {
		var pgErr *pgconn.PgError
		return errors.As(err, &pgErr) && pgErr.Code == "42501"
	}
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	// cleanup の support 系4ステップを、storage_cleanup.go と同じ順に流す。
	cleanupSteps := func(q *db.Queries) []func(context.Context, time.Time) (int64, error) {
		return []func(context.Context, time.Time) (int64, error){
			q.DeleteExpiredSupportContractStatements,
			q.DeleteExpiredSupportCoachSettlements,
			q.DeleteExpiredSupportCoachAdvances,
			q.DeleteExpiredSupportFaultDeterminations,
		}
	}
	detached := func() int {
		t.Helper()
		var n int
		if err := c.QueryRow(ctx, `SELECT (SELECT count(*) FROM support_contract_statements WHERE contract_id IS NULL)
			+ (SELECT count(*) FROM support_coach_settlements WHERE contract_id IS NULL)
			+ (SELECT count(*) FROM support_coach_advances WHERE contract_id IS NULL)
			+ (SELECT count(*) FROM support_fault_determinations WHERE contract_id IS NULL)`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	// 版199（本番の障害と同じ状態）：最初の削除ステップと、契約の削除（トリガの UPDATE）が 42501 になる。
	contract := seed()
	api := asAPI()
	if _, err := db.New(api).DeleteExpiredSupportContractStatements(ctx, now); !denied(err) {
		t.Fatalf("199: cleanup delete was not denied: %v", err)
	}
	if _, err := api.Exec(ctx, `DELETE FROM coaching_contracts WHERE id=$1`, contract); !denied(err) {
		t.Fatalf("199: contract delete was not denied: %v", err)
	}
	api.Close(ctx)

	// T1：版200では、同じ SQL が実行ロールで通り、期限切れの記録だけが消える。
	migrateTo(200)
	api = asAPI()
	for i, step := range cleanupSteps(db.New(api)) {
		n, err := step(ctx, now)
		if err != nil {
			t.Fatalf("200: cleanup step %d: %v", i, err)
		}
		if n != 1 {
			t.Fatalf("200: cleanup step %d deleted %d, want 1", i, n)
		}
	}
	if detached() != 0 {
		t.Fatalf("200: detached records left %d", detached())
	}
	// 契約の削除（退会の CASCADE と同じ経路）も、トリガの書き写しを含めて通る。
	if _, err := api.Exec(ctx, `DELETE FROM coaching_contracts WHERE id=$1`, contract); err != nil {
		t.Fatalf("200: contract delete: %v", err)
	}
	if detached() != 4 {
		t.Fatalf("200: contract delete did not detach 4 records: %d", detached())
	}
	// 全権限の一括付与はしない（TRUNCATE などは付かない）。
	var truncate bool
	if err := api.QueryRow(ctx, `SELECT has_table_privilege('support_contract_statements','TRUNCATE')`).Scan(&truncate); err != nil || truncate {
		t.Fatalf("200: TRUNCATE granted %v %v", truncate, err)
	}
	api.Close(ctx)
	exec(`DELETE FROM support_contract_statements; DELETE FROM support_coach_settlements; DELETE FROM support_coach_advances; DELETE FROM support_fault_determinations`)

	// T3：down で足した権限だけが外れ（帰責判定の UPDATE は以前からの権限なので残す）、もう一度 up すると通る。
	migrateTo(199)
	privileges := func(table, privilege string) bool {
		t.Helper()
		var ok bool
		if err := c.QueryRow(ctx, `SELECT has_table_privilege($1, $2, $3)`, role, table, privilege).Scan(&ok); err != nil {
			t.Fatal(err)
		}
		return ok
	}
	for _, table := range []string{"support_contract_statements", "support_coach_settlements", "support_coach_advances", "support_fault_determinations"} {
		if privileges(table, "DELETE") {
			t.Fatalf("down kept DELETE on %s", table)
		}
		if want := table == "support_fault_determinations"; privileges(table, "UPDATE") != want {
			t.Fatalf("down UPDATE on %s = %v, want %v", table, !want, want)
		}
		if !privileges(table, "SELECT") || !privileges(table, "INSERT") {
			t.Fatalf("down removed the pre-existing privileges on %s", table)
		}
	}
	migrateTo(200)
	seed()
	api = asAPI()
	defer api.Close(ctx)
	for i, step := range cleanupSteps(db.New(api)) {
		if _, err := step(ctx, now); err != nil {
			t.Fatalf("re-up: cleanup step %d: %v", i, err)
		}
	}
	if detached() != 0 {
		t.Fatalf("re-up: detached records left %d", detached())
	}
}
