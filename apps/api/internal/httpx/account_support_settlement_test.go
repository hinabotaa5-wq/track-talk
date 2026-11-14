package httpx_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// #726: 精算まで終えた1ヶ月プランがあっても退会でき、精算記録（仮払い・確定額・帰責判定・返還の台帳）は
// 行ごと残る（契約への参照だけが外れる）。運営者決定（2026-11-15）で CASCADE ではなく SET NULL。

// settlementKeys は、契約に付いた精算記録の各行のキー（退会で contract_id が外れたあとも行を追えるように）。
type settlementKeys struct {
	advances, settlements, determinations, adjustments []string
}

func supportSettlementKeys(t *testing.T, contract string) settlementKeys {
	t.Helper()
	var k settlementKeys
	queryRowArgs(t, `SELECT
		COALESCE((SELECT array_agg(earning_id::text ORDER BY earning_id) FROM support_coach_advances WHERE contract_id=$1), '{}'),
		COALESCE((SELECT array_agg(id::text ORDER BY id) FROM support_coach_settlements WHERE contract_id=$1), '{}'),
		COALESCE((SELECT array_agg(id::text ORDER BY id) FROM support_fault_determinations WHERE contract_id=$1), '{}'),
		COALESCE((SELECT array_agg(id::text ORDER BY id) FROM coach_refund_adjustments WHERE contract_id=$1), '{}')`,
		[]any{contract}, &k.advances, &k.settlements, &k.determinations, &k.adjustments)
	return k
}

// settlementSnapshot は、キーで選んだ行の中身（契約とコーチへの参照を除く）と、件数・契約に付いたままの件数を返す。
// コーチへの参照（coach_refund_adjustments.coach_id）は、もとからコーチの退会で SET NULL になるので比べない。
type settlementSnapshot struct {
	body                        string
	rows, attached, adjustCount int
}

func supportSettlementSnapshot(t *testing.T, k settlementKeys) settlementSnapshot {
	t.Helper()
	var s settlementSnapshot
	queryRowArgs(t, `SELECT
		jsonb_build_array(
			COALESCE((SELECT jsonb_agg(to_jsonb(a) - 'contract_id' ORDER BY a.earning_id) FROM support_coach_advances a WHERE a.earning_id::text = ANY($1)), '[]'),
			COALESCE((SELECT jsonb_agg(to_jsonb(s) - 'contract_id' ORDER BY s.id) FROM support_coach_settlements s WHERE s.id::text = ANY($2)), '[]'),
			COALESCE((SELECT jsonb_agg(to_jsonb(d) - 'contract_id' ORDER BY d.id) FROM support_fault_determinations d WHERE d.id::text = ANY($3)), '[]'),
			COALESCE((SELECT jsonb_agg(to_jsonb(j) - 'contract_id' - 'coach_id' ORDER BY j.id) FROM coach_refund_adjustments j WHERE j.id::text = ANY($4)), '[]')
		)::text,
		(SELECT count(*) FROM support_coach_advances WHERE earning_id::text = ANY($1))::int
		+ (SELECT count(*) FROM support_coach_settlements WHERE id::text = ANY($2))::int
		+ (SELECT count(*) FROM support_fault_determinations WHERE id::text = ANY($3))::int
		+ (SELECT count(*) FROM coach_refund_adjustments WHERE id::text = ANY($4))::int,
		(SELECT count(*) FROM support_coach_advances WHERE earning_id::text = ANY($1) AND contract_id IS NOT NULL)::int
		+ (SELECT count(*) FROM support_coach_settlements WHERE id::text = ANY($2) AND contract_id IS NOT NULL)::int
		+ (SELECT count(*) FROM support_fault_determinations WHERE id::text = ANY($3) AND contract_id IS NOT NULL)::int
		+ (SELECT count(*) FROM coach_refund_adjustments WHERE id::text = ANY($4) AND contract_id IS NOT NULL)::int,
		(SELECT count(*) FROM coach_refund_adjustments WHERE id::text = ANY($4))::int`,
		[]any{k.advances, k.settlements, k.determinations, k.adjustments}, &s.body, &s.rows, &s.attached, &s.adjustCount)
	return s
}

// 退会のあと、精算記録が行ごと残り、中身は変わらず、契約への参照だけが外れていることを確かめる。
func assertSettlementKeptDetached(t *testing.T, k settlementKeys, before settlementSnapshot, label string) {
	t.Helper()
	after := supportSettlementSnapshot(t, k)
	if after.rows != before.rows {
		t.Fatalf("%s: settlement rows %d -> %d", label, before.rows, after.rows)
	}
	if after.attached != 0 {
		t.Fatalf("%s: %d settlement rows still reference the deleted contract", label, after.attached)
	}
	if after.body != before.body {
		t.Fatalf("%s: settlement contents changed\nbefore %s\nafter  %s", label, before.body, after.body)
	}
}

func withdrawAccount(t *testing.T, h http.Handler, token string) {
	t.Helper()
	if rec := doJSON(t, h, http.MethodDelete, "/v1/me", token, map[string]string{"confirm": "退会する"}); rec.Code != http.StatusOK {
		t.Fatalf("withdraw %d %s", rec.Code, rec.Body.String())
	}
}

// コーチのまだ送金していない報酬を、送金済みにする（退会の 409 coach_earnings_unpaid を外す）。
func markCoachEarningsPaid(t *testing.T, coach string) {
	t.Helper()
	var unpaid int
	queryRowArgs(t, `SELECT COALESCE(sum(amount_yen),0)::int FROM coach_earnings WHERE coach_id=$1 AND payout_id IS NULL`, []any{coach}, &unpaid)
	if unpaid == 0 {
		return
	}
	payout := uuid.New()
	execSQLArgs(t, `INSERT INTO coach_payouts(id,coach_id,month,amount_yen,status,stripe_transfer_id,paid_at) VALUES($1,$2,date_trunc('month',now())::date + interval '1 month',$3,'paid','tr_test_withdraw',now())`, payout, coach, unpaid)
	execSQLArgs(t, `UPDATE coach_earnings SET payout_id=$1 WHERE coach_id=$2 AND payout_id IS NULL`, payout, coach)
}

func contractCoach(t *testing.T, contract string) string {
	t.Helper()
	var coach string
	queryRowArgs(t, `SELECT coach_id::text FROM coaching_contracts WHERE id=$1`, []any{contract}, &coach)
	return coach
}

func contractExists(t *testing.T, contract string) bool {
	t.Helper()
	var n int
	queryRowArgs(t, `SELECT count(*)::int FROM coaching_contracts WHERE id=$1`, []any{contract}, &n)
	return n == 1
}

// Issue の再現手順（選手の都合で終了→tick で精算）のあと、選手が退会でき、確定の記録は残る。
func TestWithdrawAfterSupportPlayerSettlementKeepsRecords(t *testing.T) {
	h, _ := setupStripe(t)
	admin := adminToken(t, h)
	ta, tc, contract := paidConsultContract(t, h, 2, 10)
	coach := contractCoach(t, contract)
	supportEnd(t, h, admin, contract, map[string]any{"cause": "player"})
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	var status, state string
	queryRowArgs(t, `SELECT status, fulfillment_state FROM coaching_contracts WHERE id=$1`, []any{contract}, &status, &state)
	if status != "refunded" || state != "settled" {
		t.Fatalf("contract %s/%s", status, state)
	}
	k := supportSettlementKeys(t, contract)
	if len(k.settlements) != 1 {
		t.Fatalf("settlement keys %+v", k)
	}
	before := supportSettlementSnapshot(t, k)

	// 選手の退会（以前は 422 validation_error「関連データが不正です」）。
	withdrawAccount(t, h, ta)
	if contractExists(t, contract) {
		t.Fatal("contract remains after athlete withdrawal")
	}
	assertSettlementKeptDetached(t, k, before, "after athlete")
	var earned, final int
	queryRowArgs(t, `SELECT earned_yen, final_yen FROM support_coach_settlements WHERE id::text=$1`, []any{k.settlements[0]}, &earned, &final)
	if earned != 2025 || final != 2025 {
		t.Fatalf("settlement %d %d", earned, final)
	}

	// コーチは、確定した報酬が未送金のあいだは従来どおり 409 で止まる。
	if rec := doJSON(t, h, http.MethodDelete, "/v1/me", tc, map[string]string{"confirm": "退会する"}); rec.Code != http.StatusConflict {
		t.Fatalf("coach withdraw with unpaid earnings %d %s", rec.Code, rec.Body.String())
	}
	assertSettlementKeptDetached(t, k, before, "after blocked coach")
	var unpaid int
	queryRowArgs(t, `SELECT count(*)::int FROM coach_earnings WHERE coach_id=$1 AND payout_id IS NULL`, []any{coach}, &unpaid)
	if unpaid != 1 {
		t.Fatalf("coach unpaid earnings %d", unpaid)
	}
}

// コーチ都合の終了（仮払い・帰責判定・返還の台帳・確定の4表すべてに行がある契約）でも選手が退会でき、
// 4表の行はどれも中身を変えずに残り、契約への参照だけが外れる。
func TestWithdrawAfterSupportCoachFaultSettlementKeepsRecords(t *testing.T) {
	h, _ := setupStripe(t)
	admin := adminToken(t, h)
	ta, tc, contract := paidConsultContract(t, h, 0, 30)
	execSQLArgs(t, `UPDATE coaching_contracts SET
		starts_at=(date_trunc('month', now() AT TIME ZONE 'Asia/Tokyo')::date - 10 + time '09:00') AT TIME ZONE 'Asia/Tokyo',
		competition_date=date_trunc('month', now() AT TIME ZONE 'Asia/Tokyo')::date - 10 + 29 WHERE id=$1`, contract)
	execSQLArgs(t, `INSERT INTO coaching_messages(id,contract_id,sender_role,body) VALUES(gen_random_uuid(),$1,'coach','よろしくお願いします')`, contract)
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	markCoachEarningsPaid(t, contractCoach(t, contract)) // 仮払いは送金済み
	supportEnd(t, h, admin, contract, map[string]any{"cause": "coach_fault", "reason": "連絡が10日以上途絶え、指導が提供されなかったため"})
	if r := doJSON(t, h, "POST", "/v1/coach/support-contracts/"+contract+"/objection", tc, map[string]any{"objection": "入院していたためです"}); r.Code != 200 {
		t.Fatalf("objection %d %s", r.Code, r.Body.String())
	}
	if r := doJSON(t, h, "POST", "/v1/admin/coaching/contracts/"+contract+"/support-fault/review", admin, map[string]any{"decision": "coach_fault", "reason": "入院の事実は確認できなかったため"}); r.Code != 200 {
		t.Fatalf("review %d %s", r.Code, r.Body.String())
	}
	if r := doJSON(t, h, "POST", "/v1/admin/coaching/contracts/"+contract+"/support-fault/finalize", admin, nil); r.Code != 200 {
		t.Fatalf("finalize %d %s", r.Code, r.Body.String())
	}
	k := supportSettlementKeys(t, contract)
	if len(k.advances) != 1 || len(k.settlements) != 1 || len(k.determinations) != 1 || len(k.adjustments) != 1 {
		t.Fatalf("settlement keys %+v", k)
	}
	before := supportSettlementSnapshot(t, k)

	withdrawAccount(t, h, ta)
	if contractExists(t, contract) {
		t.Fatal("contract remains after athlete withdrawal")
	}
	assertSettlementKeptDetached(t, k, before, "after athlete")
	var advance, earned, clawback, claw int
	var decision, review string
	queryRowArgs(t, `SELECT
		(SELECT amount_yen FROM support_coach_advances WHERE earning_id::text=$1),
		(SELECT earned_yen FROM support_coach_settlements WHERE id::text=$2),
		(SELECT clawback_yen FROM support_coach_settlements WHERE id::text=$2),
		(SELECT decision FROM support_fault_determinations WHERE id::text=$3),
		(SELECT review_decision FROM support_fault_determinations WHERE id::text=$3),
		(SELECT amount_yen FROM coach_refund_adjustments WHERE id::text=$4 AND kind='support_clawback')`,
		[]any{k.advances[0], k.settlements[0], k.determinations[0], k.adjustments[0]}, &advance, &earned, &clawback, &decision, &review, &claw)
	if advance != 2250 || clawback != 2250 || claw != 2250 || decision != "coach_fault" || review != "coach_fault" {
		t.Fatalf("kept records advance=%d earned=%d clawback=%d claw=%d decision=%s review=%s", advance, earned, clawback, claw, decision, review)
	}

}

// 進行中の契約があるあいだは、従来どおり 409 で止まり、契約も精算記録も消えない。
func TestWithdrawBlockedWhileSupportContractOpenKeepsRecords(t *testing.T) {
	h, _ := setupStripe(t)
	ta, tc, contract := paidConsultContract(t, h, 0, 30)
	execSQLArgs(t, `UPDATE coaching_contracts SET
		starts_at=(date_trunc('month', now() AT TIME ZONE 'Asia/Tokyo')::date - 10 + time '09:00') AT TIME ZONE 'Asia/Tokyo',
		competition_date=date_trunc('month', now() AT TIME ZONE 'Asia/Tokyo')::date - 10 + 29 WHERE id=$1`, contract)
	execSQLArgs(t, `INSERT INTO coaching_messages(id,contract_id,sender_role,body) VALUES(gen_random_uuid(),$1,'coach','よろしくお願いします')`, contract)
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	markCoachEarningsPaid(t, contractCoach(t, contract))
	k := supportSettlementKeys(t, contract)
	if len(k.advances) != 1 {
		t.Fatalf("advance keys %+v", k)
	}
	before := supportSettlementSnapshot(t, k)
	for _, tok := range []string{ta, tc} {
		rec := doJSON(t, h, http.MethodDelete, "/v1/me", tok, map[string]string{"confirm": "退会する"})
		if rec.Code != http.StatusConflict {
			t.Fatalf("withdraw with open contract %d %s", rec.Code, rec.Body.String())
		}
	}
	if !contractExists(t, contract) {
		t.Fatal("contract deleted by blocked withdrawal")
	}
	after := supportSettlementSnapshot(t, k)
	if after.rows != before.rows || after.attached != before.attached || after.body != before.body {
		t.Fatalf("settlement changed by blocked withdrawal %+v -> %+v", before, after)
	}
}
