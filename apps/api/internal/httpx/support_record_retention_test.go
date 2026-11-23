package httpx_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// 本体 #981: 取引条件の明示と精算記録（仮払い・確定・帰責判定）の保存期限。
// 消すのは契約がすでに消えた（contract_id が NULL の）記録だけで、作成した年の8年後の3月16日（日本時間）と
// 契約終了日の7年後の翌日の遅い方を過ぎてから。契約が残っている記録と、契約終了日の控えが無い記録は消さない。

var tokyo = time.FixedZone("JST", 9*60*60)

func atJST(y int, m time.Month, d, hh, mm, ss int) time.Time {
	return time.Date(y, m, d, hh, mm, ss, 0, tokyo)
}

// supportRecordTables は対象4テーブルと、行を追うキーの列。
var supportRecordTables = []struct{ table, key, json string }{
	{"support_contract_statements", "id", "support_contract_statements_deleted"},
	{"support_coach_settlements", "id", "support_coach_settlements_deleted"},
	{"support_coach_advances", "earning_id", "support_coach_advances_deleted"},
	{"support_fault_determinations", "id", "support_fault_determinations_deleted"},
}

// seedDetachedSupportRecord は、契約がすでに消えた形（contract_id が NULL）の記録を1行作り、キーを返す。
// created は作成日時の列（写しは issued_at、帰責判定は notified_at、他は created_at）。endOn は控えの契約終了日（nil で NULL）。
func seedDetachedSupportRecord(t *testing.T, table string, coach uuid.UUID, created time.Time, endOn *string) string {
	t.Helper()
	var key string
	switch table {
	case "support_contract_statements":
		queryRowArgs(t, `INSERT INTO support_contract_statements(contract_id,coach_id,body,issued_at,contract_end_on) VALUES(NULL,NULL,'{}',$1,$2::date) RETURNING id::text`, []any{created, endOn}, &key)
	case "support_coach_settlements":
		queryRowArgs(t, `INSERT INTO support_coach_settlements(contract_id,earned_yen,advances_yen,final_yen,created_at,contract_end_on) VALUES(NULL,100,0,100,$1,$2::date) RETURNING id::text`, []any{created, endOn}, &key)
	case "support_coach_advances":
		earning := uuid.New()
		execSQLArgs(t, `INSERT INTO coach_earnings(id,coach_id,source,gross_yen,share_percent,amount_yen,confirmed_at) VALUES($1,$2,'review',100,75,75,$3)`, earning, coach, created)
		queryRowArgs(t, `INSERT INTO support_coach_advances(earning_id,contract_id,through_date,elapsed_days,total_days,amount_yen,created_at,contract_end_on) VALUES($1,NULL,'2026-01-31',1,30,75,$2,$3::date) RETURNING earning_id::text`, []any{earning, created, endOn}, &key)
	case "support_fault_determinations":
		queryRowArgs(t, `INSERT INTO support_fault_determinations(contract_id,decision,reason,notified_at,contract_end_on) VALUES(NULL,'unavoidable','やむを得ない事情',$1,$2::date) RETURNING id::text`, []any{created, endOn}, &key)
	default:
		t.Fatalf("unknown table %s", table)
	}
	return key
}

func supportRecordExists(t *testing.T, table, keyColumn, key string) bool {
	t.Helper()
	var n int
	queryRowArgs(t, `SELECT count(*)::int FROM `+table+` WHERE `+keyColumn+`::text=$1`, []any{key}, &n)
	return n == 1
}

// retentionCleanupHarness は、時計を動かせる storage cleanup（Cloudinary は空の一覧を返す偽物）。
func retentionCleanupHarness(t *testing.T, stripe bool) (http.Handler, *time.Time) {
	t.Helper()
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"resources": []any{}})
	}))
	t.Cleanup(cloud.Close)
	now := time.Now()
	mutate := func(c *config.Config) {
		c.StorageCleanupSecret = "retention-cleanup"
		c.Clock = func() time.Time { return now }
		c.CloudName = "demo"
		c.CloudAPIKey = "key"
		c.CloudAPISecret = "secret"
		c.CloudAPIBase = cloud.URL
	}
	if stripe {
		h, _ := setupStripeConfig(t, mutate)
		return h, &now
	}
	return setupConfig(t, mutate), &now
}

// runRetentionCleanup は、指定の時刻でジョブを1回動かし、対象ごとの削除件数を返す。
func runRetentionCleanup(t *testing.T, h http.Handler, clock *time.Time, at time.Time) map[string]int64 {
	t.Helper()
	*clock = at
	rec := cleanupRequest(h, "retention-cleanup")
	if rec.Code != http.StatusOK {
		t.Fatalf("cleanup at %s: %d %s", at, rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	out := map[string]int64{}
	for _, target := range supportRecordTables {
		v, ok := body[target.json].(float64)
		if !ok {
			t.Fatalf("result has no %s: %s", target.json, rec.Body.String())
		}
		out[target.table] = int64(v)
	}
	return out
}

func strp(s string) *string { return &s }

// 細目1・2・5、T1・T2・T6、細目4（対象外の記録）、T3 の「期限前は contract_id が NULL でも消えない」。
func TestSupportRecordRetentionDetachedBoundaries(t *testing.T) {
	h, clock := retentionCleanupHarness(t, false)
	coach := uuid.New()
	execSQLArgs(t, `INSERT INTO profiles(id,nickname) VALUES($1,'retention')`, coach)

	type row struct {
		name    string
		created time.Time
		endOn   *string
	}
	rows := []row{
		// 細目1：2026年中の作成（契約は早く終わっている）→ 2034-03-16 00:00 JST から消える。
		{"boundary", atJST(2026, 6, 1, 12, 0, 0), strp("2026-07-01")},
		// 細目2：暦年は日本時間。12/31 の作成と、1/1 早朝（UTC ではまだ前年）の作成で1年ずれる。
		{"dec31", atJST(2026, 12, 31, 23, 30, 0), strp("2027-01-31")},
		{"jan1", atJST(2027, 1, 1, 0, 30, 0), strp("2027-01-31")},
		// T1：2026-11-22 発行・2028-05-01 終了 → 2034-03-16 では消えず、2035-05-02 から消える。
		{"long", atJST(2026, 11, 22, 10, 0, 0), strp("2028-05-01")},
		// T2：2026年1月作成・2月終了 → 契約終了基準（2033-02-16）を過ぎても、税務期限（2034-03-16）までは消えない。
		{"short", atJST(2026, 1, 15, 10, 0, 0), strp("2026-02-15")},
		// T6：契約終了日がどこからも分からない記録は、作成が古くても消さない。
		{"unknown", atJST(2010, 1, 1, 10, 0, 0), nil},
	}
	keys := map[string]map[string]string{}
	for _, r := range rows {
		keys[r.name] = map[string]string{}
		for _, target := range supportRecordTables {
			keys[r.name][target.table] = seedDetachedSupportRecord(t, target.table, coach, r.created, r.endOn)
		}
	}
	if utc := atJST(2027, 1, 1, 0, 30, 0).UTC(); utc.Year() != 2026 {
		t.Fatalf("fixture must be in the previous UTC year: %s", utc)
	}
	// 細目4：対象外の記録（返還の台帳・報酬・退会していない人の文書同意）は、同じ作成年でも消さない。
	adjustment, earning := uuid.New(), uuid.New()
	execSQLArgs(t, `INSERT INTO coach_refund_adjustments(id,coach_id,kind,amount_yen,contract_id,created_at) VALUES($1,$2,'support_clawback',100,NULL,$3)`, adjustment, coach, atJST(2026, 6, 1, 12, 0, 0))
	execSQLArgs(t, `INSERT INTO coach_earnings(id,coach_id,source,gross_yen,share_percent,amount_yen,confirmed_at,created_at) VALUES($1,$2,'review',100,75,75,$3,$3)`, earning, coach, atJST(2026, 6, 1, 12, 0, 0))
	execSQLArgs(t, `DELETE FROM document_consents`)
	execSQLArgs(t, `INSERT INTO document_consents(user_id,document,version,consented_at) VALUES($1,'terms','test',$2)`, coach, atJST(2026, 6, 1, 12, 0, 0))

	alive := func(name string) []string {
		var tables []string
		for _, target := range supportRecordTables {
			if supportRecordExists(t, target.table, target.key, keys[name][target.table]) {
				tables = append(tables, target.table)
			}
		}
		return tables
	}
	expectAlive := func(label string, want map[string]bool) {
		t.Helper()
		for name, keep := range want {
			got := len(alive(name))
			if keep && got != len(supportRecordTables) {
				t.Fatalf("%s: %s deleted too early (alive %v)", label, name, alive(name))
			}
			if !keep && got != 0 {
				t.Fatalf("%s: %s not deleted (alive %v)", label, name, alive(name))
			}
		}
	}
	expectCounts := func(label string, got map[string]int64, want int64) {
		t.Helper()
		for _, target := range supportRecordTables {
			if got[target.table] != want {
				t.Fatalf("%s: %s deleted %d, want %d (%v)", label, target.table, got[target.table], want, got)
			}
		}
	}

	// T2 の契約終了基準は過ぎているが、税務期限の前。
	expectCounts("2033-03-01", runRetentionCleanup(t, h, clock, atJST(2033, 3, 1, 0, 0, 0)), 0)
	expectAlive("2033-03-01", map[string]bool{"boundary": true, "dec31": true, "jan1": true, "long": true, "short": true, "unknown": true})
	// 細目1：3月15日の終わりまでは残す。
	expectCounts("2034-03-15 23:59:59", runRetentionCleanup(t, h, clock, atJST(2034, 3, 15, 23, 59, 59)), 0)
	expectAlive("2034-03-15 23:59:59", map[string]bool{"boundary": true, "dec31": true, "jan1": true, "long": true, "short": true, "unknown": true})
	// 3月16日 00:00（日本時間）から消す。2026年作成の boundary・dec31・short が消え、jan1（2027年作成）と long（T1）は残る。
	expectCounts("2034-03-16 00:00", runRetentionCleanup(t, h, clock, atJST(2034, 3, 16, 0, 0, 0)), 3)
	expectAlive("2034-03-16 00:00", map[string]bool{"boundary": false, "dec31": false, "short": false, "jan1": true, "long": true, "unknown": true})
	// 細目2：2027年1月1日の作成は1年後ろの 2035-03-16 から。
	expectCounts("2035-03-15 23:59:59", runRetentionCleanup(t, h, clock, atJST(2035, 3, 15, 23, 59, 59)), 0)
	expectCounts("2035-03-16 00:00", runRetentionCleanup(t, h, clock, atJST(2035, 3, 16, 0, 0, 0)), 1)
	expectAlive("2035-03-16 00:00", map[string]bool{"jan1": false, "long": true, "unknown": true})
	// T1：契約終了基準（2028-05-01 の7年後の翌日）が遅いので、その前日の終わりまでは残す。
	expectCounts("2035-05-01 23:59:59", runRetentionCleanup(t, h, clock, atJST(2035, 5, 1, 23, 59, 59)), 0)
	expectAlive("2035-05-01 23:59:59", map[string]bool{"long": true})
	expectCounts("2035-05-02 00:00", runRetentionCleanup(t, h, clock, atJST(2035, 5, 2, 0, 0, 0)), 1)
	expectAlive("2035-05-02 00:00", map[string]bool{"long": false, "unknown": true})
	// T6：ずっと先でも、終了日の分からない記録は消さない。
	expectCounts("2100", runRetentionCleanup(t, h, clock, atJST(2100, 1, 1, 0, 0, 0)), 0)
	expectAlive("2100", map[string]bool{"unknown": true})

	var n int
	queryRowArgs(t, `SELECT (SELECT count(*) FROM coach_refund_adjustments WHERE id=$1) + (SELECT count(*) FROM coach_earnings WHERE id=$2) + (SELECT count(*) FROM document_consents WHERE user_id=$3)`, []any{adjustment, earning, coach}, &n)
	if n != 3 {
		t.Fatalf("out-of-scope records deleted: %d of 3 remain", n)
	}
}

// 細目6：Cloudinary が未設定でも、対象の削除は動く（保護者同意・文書同意の削除と同じ段）。
func TestSupportRecordRetentionRunsWithoutCloudinary(t *testing.T) {
	now := atJST(2040, 1, 1, 0, 0, 0)
	h := setupConfig(t, func(c *config.Config) {
		c.StorageCleanupSecret = "retention-no-cloud"
		c.Clock = func() time.Time { return now }
	})
	coach := uuid.New()
	execSQLArgs(t, `INSERT INTO profiles(id,nickname) VALUES($1,'retention')`, coach)
	keys := map[string]string{}
	for _, target := range supportRecordTables {
		keys[target.table] = seedDetachedSupportRecord(t, target.table, coach, atJST(2026, 6, 1, 0, 0, 0), strp("2026-07-01"))
	}
	if rec := cleanupRequest(h, "retention-no-cloud"); rec.Code != http.StatusInternalServerError {
		t.Fatalf("cleanup without cloudinary %d %s", rec.Code, rec.Body.String())
	}
	for _, target := range supportRecordTables {
		if supportRecordExists(t, target.table, target.key, keys[target.table]) {
			t.Fatalf("%s not deleted without cloudinary", target.table)
		}
	}
}

func supportRecordKeysOf(t *testing.T, contract string) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	for _, target := range supportRecordTables {
		var keys []string
		queryRowArgs(t, `SELECT COALESCE(array_agg(`+target.key+`::text ORDER BY `+target.key+`), '{}') FROM `+target.table+` WHERE contract_id=$1`, []any{contract}, &keys)
		out[target.table] = keys
	}
	return out
}

// backdateSupportRecords は、契約に付いた記録の作成日時を古い日に動かす。
func backdateSupportRecords(t *testing.T, contract string, at time.Time) {
	t.Helper()
	execSQLArgs(t, `UPDATE support_contract_statements SET issued_at=$2 WHERE contract_id=$1`, contract, at)
	execSQLArgs(t, `UPDATE support_coach_settlements SET created_at=$2 WHERE contract_id=$1`, contract, at)
	execSQLArgs(t, `UPDATE support_coach_advances SET created_at=$2 WHERE contract_id=$1`, contract, at)
	execSQLArgs(t, `UPDATE support_fault_determinations SET notified_at=$2 WHERE contract_id=$1`, contract, at)
}

func countSupportRecords(t *testing.T, keys map[string][]string) int {
	t.Helper()
	total := 0
	for _, target := range supportRecordTables {
		var n int
		queryRowArgs(t, `SELECT count(*)::int FROM `+target.table+` WHERE `+target.key+`::text = ANY($1)`, []any{keys[target.table]}, &n)
		total += n
	}
	return total
}

func contractEndOnValues(t *testing.T, keys map[string][]string) []string {
	t.Helper()
	var values []string
	for _, target := range supportRecordTables {
		var got []string
		queryRowArgs(t, `SELECT COALESCE(array_agg(COALESCE(contract_end_on::text,'NULL')), '{}') FROM `+target.table+` WHERE `+target.key+`::text = ANY($1)`, []any{keys[target.table]}, &got)
		values = append(values, got...)
	}
	return values
}

func finalizationCandidates(t *testing.T) []string {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	rows, err := db.New(pool).ListSupportFinalizations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, r := range rows {
		ids = append(ids, r.ID.String())
	}
	slices.Sort(ids)
	return ids
}

// T3・T9：精算済みの契約が残っている間は、両期限を過ぎても記録を消さず、精算候補も再精算も起きない。
// T4・T11：退会で契約が消えると、契約終了日が控えに書き写され、削除可能日を過ぎたら消える。
func TestSupportRecordRetentionKeepsLiveContractAndDeletesAfterWithdrawal(t *testing.T) {
	h, clock := retentionCleanupHarness(t, true)
	admin := adminToken(t, h)
	ta, _, contract := paidConsultContract(t, h, 2, 10)
	coach := contractCoach(t, contract)
	supportEnd(t, h, admin, contract, map[string]any{"cause": "player"})
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	keys := supportRecordKeysOf(t, contract)
	if len(keys["support_contract_statements"]) != 1 || len(keys["support_coach_settlements"]) != 1 {
		t.Fatalf("records %+v", keys)
	}
	total := countSupportRecords(t, keys)
	// 作成も契約終了も十分に古くする（2011年作成・2012-03-01 終了）。
	backdateSupportRecords(t, contract, atJST(2011, 5, 1, 12, 0, 0))
	execSQLArgs(t, `UPDATE coaching_contracts SET support_end_on='2012-02-20', ends_at=$2 WHERE id=$1`, contract, atJST(2012, 3, 1, 0, 0, 0))
	var earnings int
	queryRowArgs(t, `SELECT count(*)::int FROM coach_earnings WHERE contract_id=$1`, []any{contract}, &earnings)
	candidates := finalizationCandidates(t)

	// T3・T9：契約が残っている間は、両期限を過ぎていても消さない。
	got := runRetentionCleanup(t, h, clock, time.Now())
	for table, n := range got {
		if n != 0 {
			t.Fatalf("live contract records deleted: %s %d", table, n)
		}
	}
	if countSupportRecords(t, keys) != total {
		t.Fatal("live contract records deleted")
	}
	if after := finalizationCandidates(t); !slices.Equal(after, candidates) || slices.Contains(after, contract) {
		t.Fatalf("finalization candidates changed: %v -> %v", candidates, after)
	}
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	var after int
	queryRowArgs(t, `SELECT count(*)::int FROM coach_earnings WHERE contract_id=$1`, []any{contract}, &after)
	if after != earnings {
		t.Fatalf("final earning re-registered: %d -> %d", earnings, after)
	}

	// T4：選手の退会で契約が消えると、終了日（support_end_on と ends_at の日本時間の日付の遅い方）が控えに残る。
	markCoachEarningsPaid(t, coach)
	withdrawAccount(t, h, ta)
	if contractExists(t, contract) {
		t.Fatal("contract remains after withdrawal")
	}
	for _, v := range contractEndOnValues(t, keys) {
		if v != "2012-03-01" {
			t.Fatalf("contract_end_on %v", contractEndOnValues(t, keys))
		}
	}
	// T11：削除可能日（2011年作成 → 2019-03-16、2012-03-01 終了 → 2019-03-02 の遅い方）の前は残し、過ぎたら消す。
	runRetentionCleanup(t, h, clock, atJST(2019, 3, 15, 23, 59, 59))
	if countSupportRecords(t, keys) != total {
		t.Fatal("deleted before the deadline")
	}
	got = runRetentionCleanup(t, h, clock, atJST(2019, 3, 16, 0, 0, 0))
	var deleted int64
	for _, n := range got {
		deleted += n
	}
	if countSupportRecords(t, keys) != 0 || deleted != int64(total) {
		t.Fatalf("after deadline: remain %d, deleted %v (total %d)", countSupportRecords(t, keys), got, total)
	}
}

// T10：帰責判定が保留中の契約（現存）の記録は、両期限を過ぎても消さず、保留解除の精算が従来どおり終わる。
func TestSupportRecordRetentionKeepsHeldFaultSettlement(t *testing.T) {
	h, clock := retentionCleanupHarness(t, true)
	admin := adminToken(t, h)
	_, tc, contract := paidConsultContract(t, h, 0, 30)
	execSQLArgs(t, `UPDATE coaching_contracts SET
		starts_at=(date_trunc('month', now() AT TIME ZONE 'Asia/Tokyo')::date - 10 + time '09:00') AT TIME ZONE 'Asia/Tokyo',
		competition_date=date_trunc('month', now() AT TIME ZONE 'Asia/Tokyo')::date - 10 + 29 WHERE id=$1`, contract)
	execSQLArgs(t, `INSERT INTO coaching_messages(id,contract_id,sender_role,body) VALUES(gen_random_uuid(),$1,'coach','よろしくお願いします')`, contract)
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	payout := uuid.New()
	coach := contractCoach(t, contract)
	execSQLArgs(t, `INSERT INTO coach_payouts(id,coach_id,month,amount_yen,status,stripe_transfer_id,paid_at) VALUES($1,$2,date_trunc('month',now())::date,2250,'paid','tr_test_retention',now())`, payout, coach)
	execSQLArgs(t, `UPDATE coach_earnings SET payout_id=$1 WHERE id IN (SELECT earning_id FROM support_coach_advances WHERE contract_id=$2)`, payout, contract)
	supportEnd(t, h, admin, contract, map[string]any{"cause": "coach_fault", "reason": "連絡が10日以上途絶え、指導が提供されなかったため"})
	if r := doJSON(t, h, "POST", "/v1/coach/support-contracts/"+contract+"/objection", tc, map[string]any{"objection": "入院していたためです"}); r.Code != 200 {
		t.Fatalf("objection %d %s", r.Code, r.Body.String())
	}
	keys := supportRecordKeysOf(t, contract)
	if len(keys["support_fault_determinations"]) != 1 || len(keys["support_coach_advances"]) != 1 || len(keys["support_contract_statements"]) != 1 {
		t.Fatalf("records %+v", keys)
	}
	total := countSupportRecords(t, keys)
	backdateSupportRecords(t, contract, atJST(2010, 5, 1, 12, 0, 0))
	execSQLArgs(t, `UPDATE coaching_contracts SET support_end_on='2010-06-01', ends_at=$2 WHERE id=$1`, contract, atJST(2010, 6, 1, 0, 0, 0))
	// 控えに古い終了日が入っていても、契約が残っている間は消さない。
	for _, target := range supportRecordTables {
		execSQLArgs(t, `UPDATE `+target.table+` SET contract_end_on='2010-06-01' WHERE contract_id=$1`, contract)
	}
	got := runRetentionCleanup(t, h, clock, time.Now())
	for table, n := range got {
		if n != 0 {
			t.Fatalf("held contract records deleted: %s %d", table, n)
		}
	}
	if countSupportRecords(t, keys) != total {
		t.Fatal("held contract records deleted")
	}
	if r := doJSON(t, h, "POST", "/v1/admin/coaching/contracts/"+contract+"/support-fault/review", admin, map[string]any{"decision": "coach_fault", "reason": "入院の事実は確認できなかったため"}); r.Code != 200 {
		t.Fatalf("review %d %s", r.Code, r.Body.String())
	}
	r := doJSON(t, h, "POST", "/v1/admin/coaching/contracts/"+contract+"/support-fault/finalize", admin, nil)
	if r.Code != 200 || !strings.Contains(r.Body.String(), `"clawback_yen":2250`) {
		t.Fatalf("finalize %d %s", r.Code, r.Body.String())
	}
}

// T5：終了日が決まらないまま契約が消えたら、消えた日（日本時間）が控えに入り、その日から7年の翌日までは消さない。
// T7：契約が消える前に ends_at が合意延長で後ろ倒しになっていれば、控えも削除可能日も後ろ倒しになる。
func TestSupportRecordRetentionCopiesEndOnContractDeletion(t *testing.T) {
	h, clock := retentionCleanupHarness(t, true)
	admin := adminToken(t, h)

	// T5：承諾した直後（支払い前で ends_at が無い）の契約。取引条件の明示は出ている。
	coachID, tc := consultCoach(t, h)
	_, ta := demoUser(t, h, bornYearsAgo(20))
	id := sendConsult(t, h, ta, coachID, consultAnswersBody())
	if r := doJSON(t, h, "POST", "/v1/support-consults/"+id+"/propose", tc, proposalBody()); r.Code != 200 {
		t.Fatalf("propose %d %s", r.Code, r.Body.String())
	}
	r := doJSON(t, h, "POST", "/v1/support-consults/"+id+"/accept", ta, consentBody)
	if r.Code != 201 {
		t.Fatalf("accept %d %s", r.Code, r.Body.String())
	}
	pending := decodeContract(t, r.Body.Bytes()).ID
	var endsAt *time.Time
	var supportEndOn *time.Time
	queryRowArgs(t, `SELECT ends_at, support_end_on FROM coaching_contracts WHERE id=$1`, []any{pending}, &endsAt, &supportEndOn)
	if endsAt != nil || supportEndOn != nil {
		t.Fatalf("pending contract already has an end: %v %v", endsAt, supportEndOn)
	}
	pendingKeys := supportRecordKeysOf(t, pending)
	if len(pendingKeys["support_contract_statements"]) != 1 {
		t.Fatalf("statement %+v", pendingKeys)
	}
	backdateSupportRecords(t, pending, atJST(2010, 1, 1, 0, 0, 0))
	var today string
	queryRowArgs(t, `SELECT (now() AT TIME ZONE 'Asia/Tokyo')::date::text`, nil, &today)
	execSQLArgs(t, `DELETE FROM coaching_contracts WHERE id=$1`, pending)
	if v := contractEndOnValues(t, pendingKeys); len(v) != 1 || v[0] != today {
		t.Fatalf("pending contract_end_on %v, want %s", v, today)
	}
	deletedOn, err := time.ParseInLocation(time.DateOnly, today, tokyo)
	if err != nil {
		t.Fatal(err)
	}
	deadline := deletedOn.AddDate(7, 0, 1)
	runRetentionCleanup(t, h, clock, deadline.Add(-time.Second))
	if countSupportRecords(t, pendingKeys) != 1 {
		t.Fatal("pending contract statement deleted before the deadline")
	}
	runRetentionCleanup(t, h, clock, deadline)
	if countSupportRecords(t, pendingKeys) != 0 {
		t.Fatal("pending contract statement not deleted after the deadline")
	}

	// T7：2012年作成・2013-03-10 終了の契約が、合意延長で7日後ろ倒しになってから消える。
	_, _, contract := paidConsultContract(t, h, 2, 10)
	supportEnd(t, h, admin, contract, map[string]any{"cause": "player"})
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	keys := supportRecordKeysOf(t, contract)
	total := countSupportRecords(t, keys)
	backdateSupportRecords(t, contract, atJST(2012, 1, 10, 12, 0, 0))
	execSQLArgs(t, `UPDATE coaching_contracts SET support_end_on='2013-03-01', ends_at=$2 WHERE id=$1`, contract, atJST(2013, 3, 10, 0, 0, 0))
	execSQLArgs(t, `UPDATE coaching_contracts SET ends_at = ends_at + make_interval(days => 7) WHERE id=$1`, contract)
	execSQLArgs(t, `DELETE FROM coaching_contracts WHERE id=$1`, contract)
	for _, v := range contractEndOnValues(t, keys) {
		if v != "2013-03-17" {
			t.Fatalf("extended contract_end_on %v", contractEndOnValues(t, keys))
		}
	}
	// 延長前の終了日（2013-03-10 → 2020-03-11）と税務期限（2020-03-16）はどちらも過ぎているが、延長後（2020-03-18）の前なので残す。
	runRetentionCleanup(t, h, clock, atJST(2020, 3, 17, 12, 0, 0))
	if countSupportRecords(t, keys) != total {
		t.Fatal("deleted by the pre-extension end date")
	}
	runRetentionCleanup(t, h, clock, atJST(2020, 3, 18, 0, 0, 0))
	if countSupportRecords(t, keys) != 0 {
		t.Fatal("not deleted after the extended deadline")
	}
}
