package httpx_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// paidConsultContract は、無料相談の提案（9,000円）から成立し、支払いが済んだ大会までプランを作る。
// 期間は startOffset 日前に始まり、days 日間（両端を含む）になるように日付を合わせる。
func paidConsultContract(t *testing.T, h http.Handler, startOffset, days int) (athlete, coach, contract string) {
	t.Helper()
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
	contract = decodeContract(t, r.Body.Bytes()).ID
	checkout(t, h, ta, contract)
	paySupport(t, h, contract)
	execSQLArgs(t, `UPDATE coaching_contracts SET status='active',
		starts_at=((now() AT TIME ZONE 'Asia/Tokyo')::date - $2::int + time '09:00') AT TIME ZONE 'Asia/Tokyo',
		competition_date=(now() AT TIME ZONE 'Asia/Tokyo')::date - $2::int + $3::int - 1,
		ends_at=((now() AT TIME ZONE 'Asia/Tokyo')::date - $2::int + $3::int) AT TIME ZONE 'Asia/Tokyo' WHERE id=$1`, contract, startOffset, days)
	return ta, tc, contract
}

func supportEnd(t *testing.T, h http.Handler, admin, contract string, body map[string]any) string {
	t.Helper()
	r := doJSON(t, h, "POST", "/v1/admin/coaching/contracts/"+contract+"/support-end", admin, body)
	if r.Code != 200 {
		t.Fatalf("support end %d %s", r.Code, r.Body.String())
	}
	return r.Body.String()
}

func TestSupportEndPlayerRefundsUnelapsedAndFinalizesByDays(t *testing.T) {
	h, f := setupStripe(t)
	admin := adminToken(t, h)
	_, tc, contract := paidConsultContract(t, h, 2, 10) // 10日間の3日目
	// 取引条件の明示（フリーランス法第3条）は契約の成立時にコーチへ出ている。
	// やむを得ない事情の項目は規約第9条4の2と揃える（異議申立てに期限なし・再審査で判断が変わっても全額返金は維持し経過分を支払う）。
	r := doJSON(t, h, "GET", "/v1/coach/support-contracts/"+contract, tc, nil)
	if r.Code != 200 || !strings.Contains(r.Body.String(), `"statement":{`) || !strings.Contains(r.Body.String(), "経過日数の日割り") || !strings.Contains(r.Body.String(), "翌月15日") || !strings.Contains(r.Body.String(), "期限なく") || !strings.Contains(r.Body.String(), "全額返金はそのまま") {
		t.Fatalf("statement %d %s", r.Code, r.Body.String())
	}
	// 理由が要らない終わり方でも、判定の記録は作らない。
	out := supportEnd(t, h, admin, contract, map[string]any{"cause": "player"})
	if strings.Contains(out, `"determination"`) {
		t.Fatalf("player end has determination: %s", out)
	}
	if len(f.refundList()) != 1 || f.refundList()[0].Get("amount") != "6300" {
		t.Fatalf("refund %+v", f.refundList())
	}
	var reason string
	queryRowArgs(t, `SELECT refund_reason FROM coaching_payments WHERE contract_id=$1`, []any{contract}, &reason)
	if reason != "support_player" {
		t.Fatalf("reason %s", reason)
	}
	// 二重に終えられない。
	if r := doJSON(t, h, "POST", "/v1/admin/coaching/contracts/"+contract+"/support-end", admin, map[string]any{"cause": "player"}); r.Code != 422 {
		t.Fatalf("double end %d", r.Code)
	}
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	var amount int
	queryRowArgs(t, `SELECT amount_yen FROM coach_earnings WHERE contract_id=$1`, []any{contract}, &amount)
	if amount != 2025 { // 経過3日分2,700円の75%
		t.Fatalf("final earning %d", amount)
	}
	var earned, final int
	queryRowArgs(t, `SELECT earned_yen, final_yen FROM support_coach_settlements WHERE contract_id=$1`, []any{contract}, &earned, &final)
	if earned != 2025 || final != 2025 {
		t.Fatalf("settlement %d %d", earned, final)
	}
}

// 大会の中止・延期での終了は、1ヶ月プランでは前提が消滅したため受け付けない（無効化。定数・列は残す）。
func TestSupportEndCancelledIsDisabled(t *testing.T) {
	h, f := setupStripe(t)
	admin := adminToken(t, h)
	_, _, contract := paidConsultContract(t, h, 2, 10)
	if r := doJSON(t, h, "POST", "/v1/admin/coaching/contracts/"+contract+"/support-end", admin, map[string]any{"cause": "competition_cancelled", "announced_on": day(-1)}); r.Code != 400 {
		t.Fatalf("cancelled end %d", r.Code)
	}
	if len(f.refundList()) != 0 {
		t.Fatalf("refund on disabled cause %+v", f.refundList())
	}
	// 同じ契約は通常の終わり方では終えられる。
	supportEnd(t, h, admin, contract, map[string]any{"cause": "player"})
	if len(f.refundList()) != 1 {
		t.Fatalf("refund %+v", f.refundList())
	}
}

func TestSupportCoachFaultHoldsAdvancesAndClawsBackWithObjection(t *testing.T) {
	h, f := setupStripe(t)
	admin := adminToken(t, h)
	_, tc, contract := paidConsultContract(t, h, 0, 30)
	// 前の月の末日までに10日経過した形にする（30日間・9,000円 → 経過3,000円の75%＝2,250円が仮払い）。
	execSQLArgs(t, `UPDATE coaching_contracts SET
		starts_at=(date_trunc('month', now() AT TIME ZONE 'Asia/Tokyo')::date - 10 + time '09:00') AT TIME ZONE 'Asia/Tokyo',
		competition_date=date_trunc('month', now() AT TIME ZONE 'Asia/Tokyo')::date - 10 + 29 WHERE id=$1`, contract)
	// コーチは返答している（無返答の自動返金に当たらないように）。
	execSQLArgs(t, `INSERT INTO coaching_messages(id,contract_id,sender_role,body) VALUES(gen_random_uuid(),$1,'coach','よろしくお願いします')`, contract)
	for i := 0; i < 2; i++ { // 同じ月に何度記録しても重ならない
		if r := tick(t, h, "tick-secret"); r.Code != 200 {
			t.Fatal(r.Body.String())
		}
	}
	var advance, n int
	queryRowArgs(t, `SELECT COALESCE(sum(amount_yen),0)::int, count(*)::int FROM support_coach_advances WHERE contract_id=$1`, []any{contract}, &advance, &n)
	if advance != 2250 || n != 1 {
		t.Fatalf("advance %d (%d rows)", advance, n)
	}
	// 仮払いは送金済みにする。
	payout := uuid.New()
	var coach string
	queryRowArgs(t, `SELECT coach_id::text FROM coaching_contracts WHERE id=$1`, []any{contract}, &coach)
	execSQLArgs(t, `INSERT INTO coach_payouts(id,coach_id,month,amount_yen,status,stripe_transfer_id,paid_at) VALUES($1,$2,date_trunc('month',now())::date,2250,'paid','tr_test_advance',now())`, payout, coach)
	execSQLArgs(t, `UPDATE coach_earnings SET payout_id=$1 WHERE id IN (SELECT earning_id FROM support_coach_advances WHERE contract_id=$2)`, payout, contract)

	// 理由なしのコーチ都合は受け付けない。
	if r := doJSON(t, h, "POST", "/v1/admin/coaching/contracts/"+contract+"/support-end", admin, map[string]any{"cause": "coach_fault"}); r.Code != 400 {
		t.Fatalf("no reason %d", r.Code)
	}
	out := supportEnd(t, h, admin, contract, map[string]any{"cause": "coach_fault", "reason": "連絡が10日以上途絶え、指導が提供されなかったため"})
	if !strings.Contains(out, `"decision":"coach_fault"`) || !strings.Contains(out, `"coach_hold":true`) {
		t.Fatalf("end %s", out)
	}
	// 選手へは経過分も含めて全額返金。
	if len(f.refundList()) != 1 || f.refundList()[0].Get("amount") != "9000" {
		t.Fatalf("refund %+v", f.refundList())
	}
	// 止めている間は確定しない。
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	queryRowArgs(t, `SELECT count(*)::int FROM support_coach_settlements WHERE contract_id=$1`, []any{contract}, &n)
	if n != 0 {
		t.Fatal("settled while held")
	}
	// コーチは判定の理由を見られ、異議申立ては1回だけ。他人は申し立てられない。
	r := doJSON(t, h, "GET", "/v1/coach/support-contracts/"+contract, tc, nil)
	if r.Code != 200 || !strings.Contains(r.Body.String(), "連絡が10日以上途絶え") {
		t.Fatalf("coach view %d %s", r.Code, r.Body.String())
	}
	_, other := demoUser(t, h, bornYearsAgo(30))
	if r := doJSON(t, h, "POST", "/v1/coach/support-contracts/"+contract+"/objection", other, map[string]any{"objection": "x"}); r.Code != 404 {
		t.Fatalf("other objection %d", r.Code)
	}
	if r := doJSON(t, h, "POST", "/v1/coach/support-contracts/"+contract+"/objection", tc, map[string]any{"objection": "入院していたためです"}); r.Code != 200 {
		t.Fatalf("objection %d %s", r.Code, r.Body.String())
	}
	if r := doJSON(t, h, "POST", "/v1/coach/support-contracts/"+contract+"/objection", tc, map[string]any{"objection": "もう一度"}); r.Code != 409 {
		t.Fatalf("second objection %d", r.Code)
	}
	// 再審査の前は精算できない。
	if r := doJSON(t, h, "POST", "/v1/admin/coaching/contracts/"+contract+"/support-fault/finalize", admin, nil); r.Code != 422 {
		t.Fatalf("finalize before review %d", r.Code)
	}
	if r := doJSON(t, h, "POST", "/v1/admin/coaching/contracts/"+contract+"/support-fault/review", admin, map[string]any{"decision": "coach_fault", "reason": "入院の事実は確認できなかったため"}); r.Code != 200 {
		t.Fatalf("review %d %s", r.Code, r.Body.String())
	}
	r = doJSON(t, h, "POST", "/v1/admin/coaching/contracts/"+contract+"/support-fault/finalize", admin, nil)
	if r.Code != 200 || !strings.Contains(r.Body.String(), `"clawback_yen":2250`) {
		t.Fatalf("finalize %d %s", r.Code, r.Body.String())
	}
	// 返還は返金相殺の台帳に載り、翌月以降の報酬から差し引かれる（没収ではなく返還・精算）。
	var claw int
	queryRowArgs(t, `SELECT amount_yen FROM coach_refund_adjustments WHERE contract_id=$1 AND kind='support_clawback'`, []any{contract}, &claw)
	if claw != 2250 {
		t.Fatalf("clawback %d", claw)
	}
	queryRowArgs(t, `SELECT count(*)::int FROM coach_earnings WHERE contract_id=$1`, []any{contract}, &n)
	if n != 0 {
		t.Fatal("coach-fault contract got a final earning")
	}
}

func TestSupportUnavoidableIsNormalSettlementWithReason(t *testing.T) {
	h, f := setupStripe(t)
	admin := adminToken(t, h)
	_, tc, contract := paidConsultContract(t, h, 4, 10) // 5日目
	if r := doJSON(t, h, "POST", "/v1/admin/coaching/contracts/"+contract+"/support-end", admin, map[string]any{"cause": "unavoidable"}); r.Code != 400 {
		t.Fatalf("no reason %d", r.Code)
	}
	supportEnd(t, h, admin, contract, map[string]any{"cause": "unavoidable", "reason": "台風による長期の業務不能"})
	if len(f.refundList()) != 1 || f.refundList()[0].Get("amount") != "4500" {
		t.Fatalf("refund %+v", f.refundList())
	}
	// やむを得ない事情の判定には異議申立ての対象がない（コーチ都合の判定だけが対象）。
	if r := doJSON(t, h, "POST", "/v1/coach/support-contracts/"+contract+"/objection", tc, map[string]any{"objection": "x"}); r.Code != 409 {
		t.Fatalf("objection on unavoidable %d", r.Code)
	}
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	var amount int
	queryRowArgs(t, `SELECT amount_yen FROM coach_earnings WHERE contract_id=$1`, []any{contract}, &amount)
	if amount != 3375 { // 経過5日分4,500円の75%
		t.Fatalf("earning %d", amount)
	}
}
