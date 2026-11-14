package httpx_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/config"
)

// #730: 契約が残っているコーチも退会できる（削除順の修正・migrationなし）。
//
// 退会（purgeAccount）は DeleteProfile の直前に DeleteCoachProfileForUser で
// coach_profiles を明示削除する。そこからの coaching_plans の CASCADE 削除が
// coaching_contracts.plan_id の ON DELETE RESTRICT に当たり、契約が残るコーチは
// 422 validation_error「関連データが不正です」で退会できなかった。
// 直し方（運営者決定 (a改2)）：purgeAccount の中で、契約がまだある状態で
// コーチの status を終了側へ1回だけ動かし（status と on_duty=false を同じ UPDATE で）、
//  identity_retirement_status に保持スナップショットを記録させてから、
// 契約→coach_profiles→profiles の順に消す。中間の status と epoch 付け替えは
// 同一トランザクション内だけで、行削除で消える。

// coachRetirement は、identity_boundary.retirement の行の有無と last_transaction_at を返す。
// 終了済み契約がない場合など、last_transaction_at は NULL（nil）になりうる。
func coachRetirement(t *testing.T, coach string) (bool, *time.Time) {
	t.Helper()
	var n int
	queryRowArgs(t, `SELECT count(*)::int FROM identity_boundary.retirement WHERE coach_id=$1`, []any{coach}, &n)
	if n == 0 {
		return false, nil
	}
	var at *time.Time
	queryRowArgs(t, `SELECT last_transaction_at FROM identity_boundary.retirement WHERE coach_id=$1`, []any{coach}, &at)
	return true, at
}

// coachEpoch は、identity_boundary.coach_epochs の行の有無と epoch を返す。
func coachEpoch(t *testing.T, coach string) (bool, string) {
	t.Helper()
	var n int
	queryRowArgs(t, `SELECT count(*)::int FROM identity_boundary.coach_epochs WHERE id=$1`, []any{coach}, &n)
	if n == 0 {
		return false, ""
	}
	var epoch string
	queryRowArgs(t, `SELECT epoch::text FROM identity_boundary.coach_epochs WHERE id=$1`, []any{coach}, &epoch)
	return true, epoch
}

func coachProfileStatus(t *testing.T, coach string) (bool, string, bool) {
	t.Helper()
	var n int
	queryRowArgs(t, `SELECT count(*)::int FROM coach_profiles WHERE profile_id=$1`, []any{coach}, &n)
	if n == 0 {
		return false, "", false
	}
	var status string
	var onDuty bool
	queryRowArgs(t, `SELECT status, on_duty FROM coach_profiles WHERE profile_id=$1`, []any{coach}, &status, &onDuty)
	return true, status, onDuty
}

func planCountForCoach(t *testing.T, coach string) int {
	t.Helper()
	var n int
	queryRowArgs(t, `SELECT count(*)::int FROM coaching_plans WHERE coach_id=$1`, []any{coach}, &n)
	return n
}

func profileExists(t *testing.T, id string) bool {
	t.Helper()
	var n int
	queryRowArgs(t, `SELECT count(*)::int FROM profiles WHERE id=$1`, []any{id}, &n)
	return n == 1
}

// touchContract は、契約行の updated_at を今に進める。本番では tick の遅延精算や
// 再審査などでコーチをやめた後に契約行が触れることがあり、その形を模す。
// last_transaction の起算が契約の更新を取りこぼさないことを確かめるために、
// 他の取引源（報酬・送金）より後に置く。
func touchContract(t *testing.T, contract string) time.Time {
	t.Helper()
	execSQLArgs(t, `UPDATE coaching_contracts SET updated_at=now() WHERE id=$1`, contract)
	var at time.Time
	queryRowArgs(t, `SELECT updated_at FROM coaching_contracts WHERE id=$1`, []any{contract}, &at)
	return at
}

func withdrawExpect(t *testing.T, h http.Handler, tok string, code int) {
	t.Helper()
	if rec := doJSON(t, h, http.MethodDelete, "/v1/me", tok, map[string]string{"confirm": "退会する"}); rec.Code != code {
		t.Fatalf("withdraw %d (want %d) %s", rec.Code, code, rec.Body.String())
	}
}

// settledCoachFixture は、1ヶ月プランを選手都合で終えて tick で精算し、
// コーチの未送金報酬を送金済みにした形を作る（Issue #730 の再現手順）。
// 契約は refunded・settled、精算記録4表に行がある。
func settledCoachFixture(t *testing.T, h http.Handler) (athlete, coachTok, coach, contract string) {
	t.Helper()
	admin := adminToken(t, h)
	ta, tc, c := paidConsultContract(t, h, 2, 10)
	co := contractCoach(t, c)
	supportEnd(t, h, admin, c, map[string]any{"cause": "player"})
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	var status, state string
	queryRowArgs(t, `SELECT status, fulfillment_state FROM coaching_contracts WHERE id=$1`, []any{c}, &status, &state)
	if status != "refunded" || state != "settled" {
		t.Fatalf("contract %s/%s", status, state)
	}
	markCoachEarningsPaid(t, co)
	return ta, tc, co, c
}

// appliedOldContract は、旧フロー（1回プラン等）で返事待ちの申し込みを作る。
// 精算記録・報酬・送金は付かない形。
func appliedOldContract(t *testing.T, h http.Handler) (athlete, coachTok, coach, contract string) {
	t.Helper()
	coachID, tc, planID := planCoach(t, h, 1)
	_, ta := demoUser(t, h, bornYearsAgo(20))
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts", ta, contractBody(planID))
	if rec.Code != http.StatusCreated {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
	c := decodeContract(t, rec.Body.Bytes()).ID
	return ta, tc, coachID.String(), c
}

// 細目1・3・4・13: 精算まで終えた契約が残るコーチが退会でき、精算記録は行ごと残り、
// 契約・プラン・プロフィールは消え、epoch 行も残らない。
func TestWithdrawCoachWithSettledContractKeepsSettlement(t *testing.T) {
	h, _ := setupStripe(t)
	_, tc, coach, contract := settledCoachFixture(t, h)
	k := supportSettlementKeys(t, contract)
	if len(k.settlements) != 1 {
		t.Fatalf("settlement keys %+v", k)
	}
	before := supportSettlementSnapshot(t, k)
	if ok, _ := coachEpoch(t, coach); !ok {
		t.Fatal("coach has no epoch row before withdraw")
	}
	touched := touchContract(t, contract)

	withdrawExpect(t, h, tc, http.StatusOK)

	if contractExists(t, contract) {
		t.Fatal("contract remains after coach withdrawal")
	}
	if n := planCountForCoach(t, coach); n != 0 {
		t.Fatalf("plans remain after coach withdrawal: %d", n)
	}
	if ok, _, _ := coachProfileStatus(t, coach); ok {
		t.Fatal("coach profile remains after coach withdrawal")
	}
	if profileExists(t, coach) {
		t.Fatal("profile remains after coach withdrawal")
	}
	assertSettlementKeptDetached(t, k, before, "after coach")
	if ok, _ := coachEpoch(t, coach); ok {
		t.Fatal("coach epoch remains after coach withdrawal")
	}
	if ok, at := coachRetirement(t, coach); !ok || at == nil || !at.Equal(touched) {
		t.Fatalf("retirement last_transaction %v (want %v)", at, touched)
	}
}

// 細目2: 精算記録のない別形（断った申し込み）の契約が残るコーチも退会できる。
// 断った申し込みは last_transaction の算出対象（ended・refunded）ではないため、
// 保持スナップショットの行は作られるが last_transaction_at は NULL のまま。
// 要点は退会自体が通ることと、契約・プラン・プロフィールが消えること。
func TestWithdrawCoachWithDeclinedContract(t *testing.T) {
	h := setup(t)
	_, tc, coach, contract := appliedOldContract(t, h)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/contracts/"+contract+"/decline", tc, nil); rec.Code != http.StatusOK {
		t.Fatalf("decline %d %s", rec.Code, rec.Body.String())
	}
	k := supportSettlementKeys(t, contract)
	if len(k.advances)+len(k.settlements)+len(k.determinations)+len(k.adjustments) != 0 {
		t.Fatalf("declined contract has settlement rows %+v", k)
	}

	withdrawExpect(t, h, tc, http.StatusOK)

	if contractExists(t, contract) {
		t.Fatal("contract remains after coach withdrawal")
	}
	if n := planCountForCoach(t, coach); n != 0 {
		t.Fatalf("plans remain after coach withdrawal: %d", n)
	}
	if ok, _, _ := coachProfileStatus(t, coach); ok {
		t.Fatal("coach profile remains after coach withdrawal")
	}
	if ok, at := coachRetirement(t, coach); !ok || at != nil {
		t.Fatalf("retirement %v %v (want row with NULL last_transaction)", ok, at)
	}
}

// 細目2: 取り下げられた申し込み（cancelled）の形でも詰まらない。
func TestWithdrawCoachWithCancelledContract(t *testing.T) {
	h := setup(t)
	ta, tc, coach, contract := appliedOldContract(t, h)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts/"+contract+"/cancel", ta, nil); rec.Code != http.StatusOK {
		t.Fatalf("cancel %d %s", rec.Code, rec.Body.String())
	}

	withdrawExpect(t, h, tc, http.StatusOK)

	if contractExists(t, contract) {
		t.Fatal("contract remains after coach withdrawal")
	}
	if n := planCountForCoach(t, coach); n != 0 {
		t.Fatalf("plans remain after coach withdrawal: %d", n)
	}
	if ok, _, _ := coachProfileStatus(t, coach); ok {
		t.Fatal("coach profile remains after coach withdrawal")
	}
	if ok, at := coachRetirement(t, coach); !ok || at != nil {
		t.Fatalf("retirement %v %v (want row with NULL last_transaction)", ok, at)
	}
}

// oldTickSetup は、旧フロー（1回プラン等）の契約を期間終了させる tick が使えるハンドラを返す。
func oldTickSetup(t *testing.T) http.Handler {
	t.Helper()
	return setupConfig(t, func(cfg *config.Config) { cfg.CoachingTickSecret = "tick-secret" })
}

// oldEndedContract は、旧フローで引き受けられて期間が終わった（ended）契約を作る。
// 精算記録・報酬は付かないことがあるため、呼び側で markCoachEarningsPaid してから使う。
func oldEndedContract(t *testing.T, h http.Handler) (athlete, coachTok, coach, contract string) {
	t.Helper()
	coachID, tc, planID := planCoach(t, h, 3)
	_, tu := demoUser(t, h, bornYearsAgo(20))
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts", tu, contractBody(planID))
	if rec.Code != http.StatusCreated {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
	c := decodeContract(t, rec.Body.Bytes()).ID
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/contracts/"+c+"/accept", tc, map[string]any{"agree_report": true, "agree_detection": true}); rec.Code != http.StatusOK {
		t.Fatalf("accept %d %s", rec.Code, rec.Body.String())
	}
	execSQLArgs(t, `UPDATE coaching_contracts SET ends_at=now() - interval '1 hour' WHERE id=$1`, c)
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	var status string
	queryRowArgs(t, `SELECT status FROM coaching_contracts WHERE id=$1`, []any{c}, &status)
	if status != "ended" {
		t.Fatalf("contract %s", status)
	}
	return tu, tc, coachID.String(), c
}

// 細目8: 現役コーチ（approved）で、最新の取引が終了済み契約の updated_at である形では、
// 退会後の保持スナップショットはその契約の updated_at と一致する（起算が前倒しされない）。
func TestWithdrawCoachSnapshotsEndedContractUpdate(t *testing.T) {
	h := oldTickSetup(t)
	_, tc, coach, contract := oldEndedContract(t, h)
	if ok, status, _ := coachProfileStatus(t, coach); !ok || status != "approved" {
		t.Fatalf("coach %v %q (want approved)", ok, status)
	}
	markCoachEarningsPaid(t, coach)
	touched := touchContract(t, contract)

	withdrawExpect(t, h, tc, http.StatusOK)

	if contractExists(t, contract) {
		t.Fatal("contract remains after coach withdrawal")
	}
	if n := planCountForCoach(t, coach); n != 0 {
		t.Fatalf("plans remain after coach withdrawal: %d", n)
	}
	if ok, at := coachRetirement(t, coach); !ok || at == nil || !at.Equal(touched) {
		t.Fatalf("retirement last_transaction %v (want contract %v)", at, touched)
	}
}

// 細目9: すでにコーチをやめていた（withdrawn）コーチで、やめた後に契約の更新が
// 進んだ形でも、退会後の保持スナップショットはその進んだ値を指す（取りこぼさない）。
func TestWithdrawEndedCoachSnapshotsAdvancedContract(t *testing.T) {
	h := oldTickSetup(t)
	_, tc, coach, contract := oldEndedContract(t, h)
	markCoachEarningsPaid(t, coach)
	if rec := doJSON(t, h, http.MethodPost, coachWithdrawPath, tc, withdrawCoachBody()); rec.Code != http.StatusOK {
		t.Fatalf("coach quit %d %s", rec.Code, rec.Body.String())
	}
	if ok, status, _ := coachProfileStatus(t, coach); !ok || status != "withdrawn" {
		t.Fatalf("coach status %v %q", ok, status)
	}
	// やめた後の契約行の更新（遅延精算・再審査など）を模す。
	touched := touchContract(t, contract)

	withdrawExpect(t, h, tc, http.StatusOK)

	if contractExists(t, contract) {
		t.Fatal("contract remains after coach withdrawal")
	}
	if ok, at := coachRetirement(t, coach); !ok || at == nil || !at.Equal(touched) {
		t.Fatalf("retirement last_transaction %v (want contract %v)", at, touched)
	}
}

// 細目12: 受付中（on_duty=true）のコーチも、終了済み契約がある形で退会できる。
func TestWithdrawOnDutyCoachWithSettledContract(t *testing.T) {
	h, _ := setupStripe(t)
	_, tc, coach, contract := settledCoachFixture(t, h)
	execSQLArgs(t, `UPDATE coach_profiles SET on_duty=true WHERE profile_id=$1`, coach)
	if ok, status, onDuty := coachProfileStatus(t, coach); !ok || status != "approved" || !onDuty {
		t.Fatalf("coach %v %q on_duty=%v", ok, status, onDuty)
	}
	touched := touchContract(t, contract)

	withdrawExpect(t, h, tc, http.StatusOK)

	if contractExists(t, contract) {
		t.Fatal("contract remains after coach withdrawal")
	}
	if ok, at := coachRetirement(t, coach); !ok || at == nil || !at.Equal(touched) {
		t.Fatalf("retirement last_transaction %v (want contract %v)", at, touched)
	}
}

// 細目10: 選手側の退会では、その契約相手のコーチに retirement 行は作られない・変わらない。
func TestPlayerWithdrawLeavesCoachRetirementUntouched(t *testing.T) {
	h, _ := setupStripe(t)
	ta, _, coach, contract := settledCoachFixture(t, h)
	okBefore, epochBefore := coachEpoch(t, coach)
	if !okBefore {
		t.Fatal("coach has no epoch row before player withdraw")
	}
	if ok, _ := coachRetirement(t, coach); ok {
		t.Fatal("coach already has retirement row")
	}

	withdrawExpect(t, h, ta, http.StatusOK)

	if contractExists(t, contract) {
		t.Fatal("contract remains after athlete withdrawal")
	}
	if ok, _ := coachRetirement(t, coach); ok {
		t.Fatal("coach retirement row created by athlete withdrawal")
	}
	if ok, epoch := coachEpoch(t, coach); !ok || epoch != epochBefore {
		t.Fatalf("coach epoch changed by athlete withdrawal: %v %q (want %q)", ok, epoch, epochBefore)
	}
}

// 細目6・11・14: 退会ブロック（409）の経路では、契約・プラン・プロフィール・
// 精算記録・コーチの status／on_duty／epoch・retirement 行のいずれも変わらない。
func TestWithdrawBlockedLeavesCoachDataUntouched(t *testing.T) {
	h, _ := setupStripe(t)
	admin := adminToken(t, h)
	ta, tc, contract := paidConsultContract(t, h, 0, 30)
	coach := contractCoach(t, contract)
	execSQLArgs(t, `UPDATE coaching_contracts SET
		starts_at=(date_trunc('month', now() AT TIME ZONE 'Asia/Tokyo')::date - 10 + time '09:00') AT TIME ZONE 'Asia/Tokyo',
		competition_date=date_trunc('month', now() AT TIME ZONE 'Asia/Tokyo')::date - 10 + 29 WHERE id=$1`, contract)
	execSQLArgs(t, `INSERT INTO coaching_messages(id,contract_id,sender_role,body) VALUES(gen_random_uuid(),$1,'coach','よろしくお願いします')`, contract)
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	markCoachEarningsPaid(t, coach)
	k := supportSettlementKeys(t, contract)
	if len(k.advances) != 1 {
		t.Fatalf("advance keys %+v", k)
	}
	before := supportSettlementSnapshot(t, k)
	_, statusBefore, onDutyBefore := coachProfileStatus(t, coach)
	_, epochBefore := coachEpoch(t, coach)

	// 進行中の契約があるあいだは選手もコーチも 409 で止まる。
	for _, tok := range []string{ta, tc} {
		withdrawExpect(t, h, tok, http.StatusConflict)
	}

	if !contractExists(t, contract) {
		t.Fatal("contract deleted by blocked withdrawal")
	}
	if n := planCountForCoach(t, coach); n == 0 {
		t.Fatal("plans deleted by blocked withdrawal")
	}
	if ok, _, _ := coachProfileStatus(t, coach); !ok {
		t.Fatal("coach profile deleted by blocked withdrawal")
	}
	after := supportSettlementSnapshot(t, k)
	if after != before {
		t.Fatalf("settlement changed by blocked withdrawal %+v -> %+v", before, after)
	}
	if ok, status, onDuty := coachProfileStatus(t, coach); !ok || status != statusBefore || onDuty != onDutyBefore {
		t.Fatalf("coach status changed by blocked withdrawal: %q on_duty=%v (want %q %v)", status, onDuty, statusBefore, onDutyBefore)
	}
	if ok, epoch := coachEpoch(t, coach); !ok || epoch != epochBefore {
		t.Fatalf("coach epoch changed by blocked withdrawal: %v %q (want %q)", ok, epoch, epochBefore)
	}
	if ok, _ := coachRetirement(t, coach); ok {
		t.Fatal("coach retirement row created by blocked withdrawal")
	}

	// 精算まで終えても未送金があるあいだはコーチだけ 409 で止まり、記録は消えない。
	supportEnd(t, h, admin, contract, map[string]any{"cause": "player"})
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	k2 := supportSettlementKeys(t, contract)
	before2 := supportSettlementSnapshot(t, k2)
	withdrawExpect(t, h, tc, http.StatusConflict)
	if !contractExists(t, contract) {
		t.Fatal("contract deleted by unpaid-blocked withdrawal")
	}
	if after2 := supportSettlementSnapshot(t, k2); after2 != before2 {
		t.Fatalf("settlement changed by unpaid-blocked withdrawal %+v -> %+v", before2, after2)
	}
	if ok, status, onDuty := coachProfileStatus(t, coach); !ok || status != statusBefore || onDuty != onDutyBefore {
		t.Fatalf("coach status changed by unpaid-blocked withdrawal: %q on_duty=%v", status, onDuty)
	}
	if ok, _ := coachRetirement(t, coach); ok {
		t.Fatal("coach retirement row created by unpaid-blocked withdrawal")
	}
}
