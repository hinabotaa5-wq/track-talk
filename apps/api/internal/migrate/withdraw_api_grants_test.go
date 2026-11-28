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

// 退会（DELETE /v1/me）が、本番で permission denied for table coaching_contracts
// (42501) になり 500 を返していた問題の修正（migration 000202）のテスト。
// 退会のトランザクションが実行する文の一式を、公開 API の実行ロール
// （track_talk_api）で流し、版201では 42501 になること（障害の再現）、
// 版202では最後まで通ること、down で足した権限だけが外れること、
// ロールの無い環境でも up・down が通ることを確かめる。
//
// ベースラインの考え方：本番ロールの権限は migration の外で付けられてきたため、
// ここでは「migration が保証する権限」だけを正とする。読み取り（SELECT）は
// アプリの通常機能が本番で動いている実績に合わせてテスト側で与え、
// 書き込みは 000200 が与える分以外は 000202 が保証すべきものとして監査する。
// 精算の金額計算そのものは httpx の責務なので、ここでは finalize が使うのと
// 同じクエリを同じ対象に実行し、権限だけを検証する。
func TestWithdrawAPIGrantsMigration(t *testing.T) {
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
	const role = "track_talk_api"
	var roleExisted bool
	if err := admin.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname=$1)`, role).Scan(&roleExisted); err != nil {
		t.Fatal(err)
	}
	createdRole := false
	t.Cleanup(func() {
		if createdRole {
			if c, err := pgx.Connect(ctx, dsn); err == nil {
				c.Exec(ctx, "DROP ROLE "+role)
				c.Close(ctx)
			}
		}
	})
	newDB := func() (string, *pgx.Conn, func(uint)) {
		name := "withdraw_grants_" + uuid.New().String()[:8]
		ident := pgx.Identifier{name}.Sanitize()
		if _, err := admin.Exec(ctx, "CREATE DATABASE "+ident); err != nil {
			t.Fatalf("create isolated test database: %v", err)
		}
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
		conn, err := pgx.ConnectConfig(ctx, cfg)
		if err != nil {
			t.Fatal(err)
		}
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
		t.Cleanup(func() {
			conn.Close(ctx)
			drop, err := pgx.Connect(ctx, dsn)
			if err != nil {
				t.Errorf("drop test database connect: %v", err)
				return
			}
			defer drop.Close(ctx)
			if _, err := drop.Exec(ctx, "DROP DATABASE IF EXISTS "+ident+" WITH (FORCE)"); err != nil {
				t.Errorf("drop test database: %v", err)
			}
		})
		return name, conn, migrateTo
	}
	denied := func(err error) bool {
		var pgErr *pgconn.PgError
		return errors.As(err, &pgErr) && pgErr.Code == "42501"
	}
	_ = roleExisted
	_ = createdRole
	db1Name, c, migrateTo := newDB()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := c.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%v\n%s", err, sql)
		}
	}
	// ロールは migration より先に作る（000153/000154 の EXECUTE 付与などが
	// 本番と同じくロールへ届くように）。BYPASSRLS は本番の実行ロールと同じ。
	if !roleExisted {
		if _, err := admin.Exec(ctx, "CREATE ROLE "+role+" BYPASSRLS"); err != nil {
			t.Fatal(err)
		}
		createdRole = true
	}
	migrateTo(201)
	// ベースライン：読み取りはアプリの通常機能が本番で動いている分を与える。
	exec(`GRANT USAGE ON SCHEMA public TO ` + role)
	exec(`GRANT SELECT ON ALL TABLES IN SCHEMA public TO ` + role)
	exec(`GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO ` + role)

	// フィクスチャの ID（固定）。
	player := uuid.MustParse("10000000-0000-0000-0000-000000000001")
	coach := uuid.MustParse("10000000-0000-0000-0000-000000000002")
	requester := uuid.MustParse("10000000-0000-0000-0000-000000000003")
	supportCoach := uuid.MustParse("10000000-0000-0000-0000-000000000004")
	postID := uuid.MustParse("20000000-0000-0000-0000-000000000001")
	planID := uuid.MustParse("30000000-0000-0000-0000-000000000001")
	contractID := uuid.MustParse("40000000-0000-0000-0000-000000000001")
	reviewID := uuid.MustParse("50000000-0000-0000-0000-000000000001")
	supportPlanID := uuid.MustParse("30000000-0000-0000-0000-000000000002")
	supportContractID := uuid.MustParse("40000000-0000-0000-0000-000000000002")
	seed := func() {
		t.Helper()
		exec(`SET session_replication_role=replica`)
		defer exec(`SET session_replication_role=origin`)
		exec(`INSERT INTO profiles(id,nickname) VALUES($1,'選手'),($2,'コーチ'),($3,'依頼者'),($4,'大会コーチ')`, player, coach, requester, supportCoach)
		exec(`INSERT INTO posts(id,author_id,kind,title,body) VALUES($1,$2,'text','投稿','本文')`, postID, player)
		exec(`INSERT INTO comments(id,post_id,author_id,body) VALUES(gen_random_uuid(),$1,$2,'コメント')`, postID, player)
		exec(`INSERT INTO post_likes(post_id,user_id) VALUES($1,$2)`, postID, player)
		exec(`INSERT INTO coach_profiles(profile_id,tier,events,best_record,achievements,years,pledged_at,pledge_version,status,reviewed_at) VALUES($1,'coach',ARRAY['100m'],'10.00','実績',5,now(),1,'approved',now())`, coach)
		exec(`INSERT INTO coach_profiles(profile_id,tier,events,best_record,achievements,years,pledged_at,pledge_version,status,reviewed_at) VALUES($1,'coach',ARRAY['100m'],'10.10','実績',8,now(),1,'approved',now())`, supportCoach)
		exec(`INSERT INTO coaching_plans(id,coach_id,kind,title,description,duration_days,price_yen,capacity,video_reviews,reply_formats) VALUES($1,$2,'period','プラン','説明',30,8000,5,3,ARRAY['text'])`, planID, coach)
		exec(`INSERT INTO coaching_plans(id,coach_id,kind,title,description,duration_days,price_yen,capacity,video_reviews,reply_formats) VALUES($1,$2,'period','大会プラン','説明',30,22000,5,0,ARRAY['text'])`, supportPlanID, supportCoach)
		exec(`INSERT INTO coaching_contracts(id,plan_id,coach_id,requester_id,plan_kind,plan_title,duration_days,price_yen,event,goal,current_record,message,minor,respond_by,status,starts_at,ends_at) VALUES($1,$2,$3,$4,'period','プラン',30,8000,'100m','目標','12','申込',false,now(),'ended','2026-01-01T00:00:00+09:00','2026-01-31T00:00:00+09:00')`, contractID, planID, coach, requester)
		exec(`INSERT INTO coaching_payments(id,contract_id,payer,amount_yen,checkout_session_id,checkout_url,session_expires_at,payment_intent_id,status,paid_at) VALUES(gen_random_uuid(),$1,'requester',8000,'cs_c','https://example.test',now(),'pi_c','paid',now())`, contractID)
		exec(`INSERT INTO review_requests(id,requester_id,tier,event,focus,video_public_id,due_at,coach_id,status,returned_at) VALUES($1,$2,'coach','100m',ARRAY['start'],'video1',now(),$3,'returned',now())`, reviewID, requester, coach)
		exec(`INSERT INTO review_payments(id,request_id,payer_id,method,amount_yen,checkout_session_id,checkout_url,session_expires_at,payment_intent_id,status,captured_at) VALUES('60000000-0000-0000-0000-000000000001',$1,$2,'card',1000,'cs_r','https://example.test',now(),'pi_r','captured',now())`, reviewID, requester)
		exec(`INSERT INTO captured_payment_refunds(id,source,payment_id,payment_intent_id,amount_yen,reason,origin,status) VALUES('70000000-0000-0000-0000-000000000001','review','60000000-0000-0000-0000-000000000001','pi_r',400,'test refund','stripe','succeeded')`)
		exec(`INSERT INTO coaching_contracts(id,plan_id,coach_id,requester_id,plan_kind,plan_title,duration_days,price_yen,event,goal,current_record,message,minor,respond_by,status,starts_at,ends_at,fulfillment_version,fulfillment_state,service_type,video_reviews,support_coach_hold) VALUES($1,$2,$3,$4,'period','大会プラン',30,22000,'100m','目標','12','申込',false,now(),'ended','2026-01-01T00:00:00+09:00','2026-01-31T00:00:00+09:00',1,'settled','competition_support',0,false)`, supportContractID, supportPlanID, supportCoach, requester)
		exec(`INSERT INTO coaching_payments(id,contract_id,payer,amount_yen,checkout_session_id,checkout_url,session_expires_at,payment_intent_id,status,paid_at) VALUES(gen_random_uuid(),$1,'requester',22000,'cs_s','https://example.test',now(),'pi_s','paid',now())`, supportContractID)
		exec(`INSERT INTO coach_earnings(id,coach_id,source,gross_yen,share_percent,amount_yen,confirmed_at) VALUES('80000000-0000-0000-0000-000000000001',$1,'support_advance',5000,100,5000,now())`, supportCoach)
		exec(`INSERT INTO support_coach_advances(earning_id,contract_id,through_date,elapsed_days,total_days,amount_yen) VALUES('80000000-0000-0000-0000-000000000001',$1,'2026-01-15',10,30,5000)`, supportContractID)
		exec(`INSERT INTO coach_earnings(id,coach_id,source,gross_yen,share_percent,amount_yen,confirmed_at) VALUES('80000000-0000-0000-0000-000000000002',$1,'personal',1000,75,750,now())`, supportCoach)
		exec(`INSERT INTO coach_refund_adjustments(id,refund_id,earning_id,coach_id,kind,amount_yen,applied_yen) VALUES('90000000-0000-0000-0000-000000000001','70000000-0000-0000-0000-000000000001','80000000-0000-0000-0000-000000000002',$1,'refund_offset',1000,1000)`, supportCoach)
		exec(`INSERT INTO coach_refund_applications(id,adjustment_id,earning_id,earning_reference_id,amount_yen) VALUES('a0000000-0000-0000-0000-000000000001','90000000-0000-0000-0000-000000000001','80000000-0000-0000-0000-000000000001','80000000-0000-0000-0000-000000000001',1000)`)
	}
	seed()
	// 実行ロールの接続（本番の API と同じ権限で SQL を流す）。
	asAPI := func() *pgx.Conn {
		t.Helper()
		cfg, err := pgx.ParseConfig(dsn)
		if err != nil {
			t.Fatal(err)
		}
		// newDB で作った DB 名に合わせる。
		var dbName string
		if err := c.QueryRow(ctx, `SELECT current_database()`).Scan(&dbName); err != nil {
			t.Fatal(err)
		}
		cfg.Database = dbName
		conn, err := pgx.ConnectConfig(ctx, cfg)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, "SET ROLE "+role); err != nil {
			t.Fatal(err)
		}
		return conn
	}
	type step struct {
		name string
		run  func(q *db.Queries) error
	}
	// withdrawSteps は deleteAccount／purgeAccount が uid に対して実行する文を、
	// ハンドラと同じ順で並べたもの。withEarnings は報酬記録（全体処理）を含む版。
	withdrawSteps := func(uid uuid.UUID, withEarnings bool) []step {
		steps := []step{
			{"LockCoachPayouts", func(q *db.Queries) error { return q.LockCoachPayouts(ctx) }},
			{"LockReviewRequester", func(q *db.Queries) error { return q.LockReviewRequester(ctx, uid) }},
			{"HasOpenConsultationAccount", func(q *db.Queries) error { _, err := q.HasOpenConsultationAccount(ctx, &uid); return err }},
			{"HasOpenTicketAccount", func(q *db.Queries) error { _, err := q.HasOpenTicketAccount(ctx, &uid); return err }},
			{"WithdrawBlockers", func(q *db.Queries) error { _, err := q.WithdrawBlockers(ctx, uid); return err }},
		}
		if withEarnings {
			steps = append(steps,
				step{"InsertReviewEarnings", func(q *db.Queries) error { _, err := q.InsertReviewEarnings(ctx, 75); return err }},
				step{"InsertPersonalEarnings", func(q *db.Queries) error { _, err := q.InsertPersonalEarnings(ctx, 25); return err }},
				step{"ListSupportAdvanceCandidates", func(q *db.Queries) error { _, err := q.ListSupportAdvanceCandidates(ctx, time.Now()); return err }},
				step{"InsertSupportAdvance", func(q *db.Queries) error {
					e, err := q.InsertSupportAdvanceEarning(ctx, db.InsertSupportAdvanceEarningParams{ID: uuid.New(), CoachID: &supportCoach, AmountYen: 100, ConfirmedAt: time.Now()})
					if err != nil {
						return err
					}
					return q.InsertSupportAdvance(ctx, db.InsertSupportAdvanceParams{EarningID: e.ID, ContractID: &supportContractID, ThroughDate: time.Now(), ElapsedDays: 1, TotalDays: 30, AmountYen: 100})
				}},
				step{"ListSupportFinalizations", func(q *db.Queries) error { _, err := q.ListSupportFinalizations(ctx); return err }},
				step{"GetSupportPaid", func(q *db.Queries) error { _, err := q.GetSupportPaid(ctx, &supportContractID); return err }},
				step{"SupportAdvanceTotals", func(q *db.Queries) error { _, err := q.SupportAdvanceTotals(ctx, &supportContractID); return err }},
				step{"DeleteUnpaidSupportAdvances", func(q *db.Queries) error { return q.DeleteUnpaidSupportAdvances(ctx, &supportContractID) }},
				step{"InsertSupportFinalEarning", func(q *db.Queries) error {
					return q.InsertSupportFinalEarning(ctx, db.InsertSupportFinalEarningParams{ID: uuid.New(), CoachID: &supportCoach, ContractID: &supportContractID, GrossYen: 22000, SharePercent: 75, AmountYen: 100, ConfirmedAt: time.Now()})
				}},
				step{"InsertSupportClawback", func(q *db.Queries) error {
					return q.InsertSupportClawback(ctx, db.InsertSupportClawbackParams{ID: uuid.New(), ContractID: &supportContractID, CoachID: &supportCoach, AmountYen: 100})
				}},
				step{"InsertSupportSettlement", func(q *db.Queries) error {
					_, err := q.InsertSupportSettlement(ctx, db.InsertSupportSettlementParams{ContractID: &supportContractID, EarnedYen: 22000, AdvancesYen: 0, FinalYen: 100, ClawbackYen: 0})
					return err
				}},
			)
		}
		steps = append(steps,
			step{"GetCoachEarningsSummary", func(q *db.Queries) error { _, err := q.GetCoachEarningsSummary(ctx, &uid); return err }},
			step{"ListAccountMedia", func(q *db.Queries) error { _, err := q.ListAccountMedia(ctx, uid); return err }},
			step{"ReleaseCommentThreadsForUser", func(q *db.Queries) error { return q.ReleaseCommentThreadsForUser(ctx, uid) }},
			step{"DecrementCommentCountsForUser", func(q *db.Queries) error { return q.DecrementCommentCountsForUser(ctx, uid) }},
			step{"DecrementLikeCountsForUser", func(q *db.Queries) error { return q.DecrementLikeCountsForUser(ctx, uid) }},
			step{"DeletePracticeRatingsForUser", func(q *db.Queries) error { return q.DeletePracticeRatingsForUser(ctx, uid) }},
			step{"DeletePracticeApplicationsForUser", func(q *db.Queries) error { return q.DeletePracticeApplicationsForUser(ctx, uid) }},
			step{"DeleteReportsForUser", func(q *db.Queries) error { return q.DeleteReportsForUser(ctx, uid) }},
			step{"DeleteCommentsForUser", func(q *db.Queries) error { return q.DeleteCommentsForUser(ctx, uid) }},
			step{"DeletePostsForUser", func(q *db.Queries) error { return q.DeletePostsForUser(ctx, uid) }},
			step{"KeepGuardianRecordsOnWithdraw", func(q *db.Queries) error { return q.KeepGuardianRecordsOnWithdraw(ctx, uid) }},
			step{"KeepDocumentConsentsOnWithdraw", func(q *db.Queries) error { return q.KeepDocumentConsentsOnWithdraw(ctx, uid) }},
			step{"KeepMinorPurchaseDeclarationsOnWithdraw", func(q *db.Queries) error { return q.KeepMinorPurchaseDeclarationsOnWithdraw(ctx, uid) }},
			step{"RememberDeletedAccount", func(q *db.Queries) error { return q.RememberDeletedAccount(ctx, uid) }},
			step{"RetireCoachProfileForWithdraw", func(q *db.Queries) error { return q.RetireCoachProfileForWithdraw(ctx, uid) }},
			step{"DeleteCoachingContractsForCoach", func(q *db.Queries) error { return q.DeleteCoachingContractsForCoach(ctx, uid) }},
			step{"DeleteCoachProfileForUser", func(q *db.Queries) error { return q.DeleteCoachProfileForUser(ctx, uid) }},
			step{"DeleteProfile", func(q *db.Queries) error { return q.DeleteProfile(ctx, uid) }},
		)
		return steps
	}
	// audit は各ステップを個別トランザクション（常にロールバック）で実行し、
	// 失敗したステップ名とエラーを集める。
	audit := func(api *pgx.Conn, uid uuid.UUID, withEarnings bool) map[string]error {
		t.Helper()
		out := map[string]error{}
		for _, s := range withdrawSteps(uid, withEarnings) {
			tx, err := api.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.run(db.New(tx)); err != nil {
				out[s.name] = err
			}
			if err := tx.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
		}
		return out
	}
	// withdraw は退会の一式を1トランザクションで実行してコミットする。
	withdraw := func(api *pgx.Conn, uid uuid.UUID, withEarnings bool) {
		t.Helper()
		tx, err := api.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		q := db.New(tx)
		for _, s := range withdrawSteps(uid, withEarnings) {
			if err := s.run(q); err != nil {
				tx.Rollback(ctx)
				t.Fatalf("withdraw step %s: %v", s.name, err)
			}
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatalf("withdraw commit: %v", err)
		}
	}
	// T1（再現）：版201では退会の書き込みが 42501 で落ちる。
	// 本番障害の DeleteCoachingContractsForCoach が含まれていることを必ず確かめる。
	api := asAPI()
	failures := audit(api, coach, true)
	if len(failures) == 0 {
		t.Fatal("201: withdraw steps unexpectedly all passed")
	}
	for name, err := range failures {
		if name == "DeleteProfile" {
			continue // 監査では削除順を踏まないため RESTRICT (23503) になりうる。権限の話ではない。
		}
		if !denied(err) {
			t.Fatalf("201: step %s failed with non-42501 error: %v", name, err)
		}
	}
	if err, ok := failures["DeleteCoachingContractsForCoach"]; !ok || !denied(err) {
		t.Fatalf("201: DeleteCoachingContractsForCoach failure = %v (present=%v)", err, ok)
	}
	t.Logf("201 audit failures: %v", func() []string {
		ks := []string{}
		for k := range failures {
			ks = append(ks, k)
		}
		return ks
	}())
	api.Close(ctx)

	// T2（解消）：版202では、選手・通常コーチ・大会サポートコーチの退会が
	// 1トランザクションで最後まで通り、プロフィールが消える。
	migrateTo(202)
	api = asAPI()
	withdraw(api, coach, true)
	withdraw(api, supportCoach, false)
	withdraw(api, player, false)
	api.Close(ctx)
	for _, uid := range []uuid.UUID{player, coach, supportCoach} {
		var n int
		if err := c.QueryRow(ctx, `SELECT count(*) FROM profiles WHERE id=$1`, uid).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Fatalf("202: profile %s still exists", uid)
		}
	}
	// 精算記録は SET NULL で残る（#729 の設計どおり）。
	var settlements int
	if err := c.QueryRow(ctx, `SELECT count(*) FROM support_coach_settlements`).Scan(&settlements); err != nil {
		t.Fatal(err)
	}
	if settlements == 0 {
		t.Fatal("202: support settlement records were not kept")
	}
	// 全権限の一括付与はしない（TRUNCATE などは付かない）。
	var truncate bool
	if err := c.QueryRow(ctx, `SELECT has_table_privilege($1, 'coaching_contracts', 'TRUNCATE')`, role).Scan(&truncate); err != nil || truncate {
		t.Fatalf("202: TRUNCATE granted %v %v", truncate, err)
	}

	// T3（往復）：down で足した権限だけが外れて再び 42501 になり、up で戻る。
	migrateTo(201)
	api = asAPI()
	failures = audit(api, requester, false)
	if err, ok := failures["DeleteCoachingContractsForCoach"]; !ok || !denied(err) {
		t.Fatalf("down: DeleteCoachingContractsForCoach failure = %v (present=%v)", err, ok)
	}
	api.Close(ctx)
	migrateTo(202)
	api = asAPI()
	withdraw(api, requester, false)
	api.Close(ctx)

	// T4（ロール不在）：別の隔離 DB で、ロールが無いまま up・down・up が通る。
	if createdRole {
		if _, err := admin.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{db1Name}.Sanitize()+" WITH (FORCE)"); err != nil {
			t.Fatal(err)
		}
		if _, err := admin.Exec(ctx, "DROP ROLE "+role); err != nil {
			t.Fatal(err)
		}
		createdRole = false
	}
	_, _, migrateTo2 := newDB()
	migrateTo2(202)
	migrateTo2(201)
	migrateTo2(202)
}
