package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// declarationsFor は、購入ごとの宣言の行数と文案の版。
func declarationsFor(t *testing.T, minor uuid.UUID, refType, refID string) (int, string) {
	t.Helper()
	var n int
	var version string
	queryRowArgs(t, `SELECT count(*), COALESCE(max(text_version), '') FROM minor_purchase_declarations WHERE minor_id = $1 AND ref_type = $2 AND ref_id = $3`,
		[]any{minor, refType, refID}, &n, &version)
	return n, version
}

// 18歳未満は、保護者の同意の記録がなくても、購入のたびに宣言にチェックすれば動画添削・追加相談を買える。
// チェックがなければ支払い画面を作らない。宣言は取引ごとに残り、月の上限では止めず、保護者への購入の通知も送らない。
func TestMinorPurchaseNeedsDeclarationNotGuardianConsent(t *testing.T) {
	allowPayPayForTest(t)
	fakeCoachingCloudinary(t)
	h, f := setupCoachChoice(t)
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)
	minor, tm := demoUser(t, h, time.Date(time.Now().Year()-14, 1, 1, 0, 0, 0, 0, time.UTC))

	// チェックなしでは進めない（依頼も支払い画面も作らない）。
	body := reviewBodyFor(t, h, tm)
	body["method"] = "paypay"
	body["guardian_permission"] = false
	body["requested_coach_id"] = coach.String()
	before := sessionCount(f)
	rec := doJSON(t, h, "POST", "/v1/coaching/requests", tm, body)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "guardian_permission_required") {
		t.Fatalf("without declaration %d %s", rec.Code, rec.Body.String())
	}
	var requests int
	queryRowArgs(t, `SELECT count(*) FROM review_requests WHERE requester_id = $1`, []any{minor}, &requests)
	if sessionCount(f) != before || requests != 0 {
		t.Fatalf("created without declaration sessions=%d requests=%d", sessionCount(f)-before, requests)
	}
	body["guardian_permission"] = false
	if rec := doJSON(t, h, "POST", "/v1/coaching/requests", tm, body); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unchecked declaration %d %s", rec.Code, rec.Body.String())
	}

	// 同意の記録がなくても、チェックすれば支払い画面に進める。宣言と、支払いの未成年の印が残る。
	first := chosenReview(t, h, tm, coach.String(), "paypay", false)
	if n, version := declarationsFor(t, minor, "review", first); n != 1 || version != "purchase-notice-2026-11-13" {
		t.Fatalf("declaration %d %q", n, version)
	}
	var consents, minorPayments int
	queryRowArgs(t, `SELECT count(*) FROM guardian_consents WHERE minor_id = $1`, []any{minor}, &consents)
	queryRowArgs(t, `SELECT count(*) FROM review_payments WHERE request_id = $1 AND minor`, []any{first}, &minorPayments)
	if consents != 0 || minorPayments != 0 {
		t.Fatalf("consents=%d minor payments=%d", consents, minorPayments)
	}

	// 支払い画面を開き直すときも、毎回チェックが要る。開き直した分も宣言が残る。
	checkout := "/v1/coaching/requests/" + first + "/checkout"
	if rec := doJSON(t, h, "POST", checkout, tm, map[string]any{"method": "card"}); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "guardian_permission_required") {
		t.Fatalf("reopen without declaration %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, "POST", checkout, tm, map[string]any{"method": "paypay", "guardian_permission": true}); rec.Code != http.StatusOK {
		t.Fatalf("reopen %d %s", rec.Code, rec.Body.String())
	}
	if n, _ := declarationsFor(t, minor, "review", first); n != 2 {
		t.Fatalf("declarations after reopen %d", n)
	}
	payChosenReview(t, h, first, "paypay")

	// 前の月の上限（初期値1,500円）を超えても止めない。
	second := chosenReview(t, h, tm, coach.String(), "card", false)
	payChosenReview(t, h, second, "card")
	if n, _ := declarationsFor(t, minor, "review", second); n != 1 {
		t.Fatalf("second declaration %d", n)
	}

	// 1本目を返却してもらい、追加相談（600円）を頼む。送るときも、支払い画面を開き直すときも毎回チェックが要る。
	for _, o := range coachRequests(t, h, tc).Offers {
		if r := doJSON(t, h, "POST", "/v1/coach/offers/"+o.ID+"/accept", tc, nil); r.Code != 200 {
			t.Fatal(r.Body.String())
		}
	}
	if r := doJSON(t, h, "POST", "/v1/coaching/requests/"+first+"/response", tc, responseBody()); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	c := consultationDraft(t, h, tm, "/v1/coaching/requests/"+first+"/consultations", uuid.NewString(), "次の大会ではどこを意識しますか")
	if c.Funding != "cash" || c.Price != 600 {
		t.Fatalf("draft %+v", c)
	}
	submit := "/v1/coaching/consultations/" + c.ID + "/submit"
	if rec := doJSON(t, h, "POST", submit, tm, nil); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "guardian_permission_required") {
		t.Fatalf("submit without declaration %d %s", rec.Code, rec.Body.String())
	}
	var status string
	queryRowArgs(t, `SELECT status FROM coaching_consultations WHERE id = $1`, []any{c.ID}, &status)
	if status != "draft" {
		t.Fatalf("status after refused submit %s", status)
	}
	rec = doJSON(t, h, "POST", submit, tm, map[string]any{"guardian_permission": true})
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "checkout_url") {
		t.Fatalf("submit %d %s", rec.Code, rec.Body.String())
	}
	if n, _ := declarationsFor(t, minor, "consultation", c.ID); n != 1 {
		t.Fatalf("consultation declaration %d", n)
	}
	pay := "/v1/coaching/consultations/" + c.ID + "/checkout"
	if rec := doJSON(t, h, "POST", pay, tm, nil); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "guardian_permission_required") {
		t.Fatalf("consultation checkout without declaration %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, "POST", pay, tm, map[string]any{"guardian_permission": true}); rec.Code != 200 {
		t.Fatalf("consultation checkout %d %s", rec.Code, rec.Body.String())
	}
	if n, _ := declarationsFor(t, minor, "consultation", c.ID); n != 2 {
		t.Fatalf("consultation declarations after reopen %d", n)
	}
	if code := payConsultation(t, h, c.ID, 600); code != 200 {
		t.Fatalf("pay consultation %d", code)
	}
	queryRowArgs(t, `SELECT status FROM coaching_consultations WHERE id = $1`, []any{c.ID}, &status)
	if status != "offered" {
		t.Fatalf("paid consultation %s", status)
	}

	// 保護者への購入の通知・上限変更のお願いは作らない。
	var notices int
	queryRowArgs(t, `SELECT count(*) FROM guardian_notices WHERE minor_id = $1`, []any{minor}, &notices)
	if notices != 0 {
		t.Fatalf("guardian notices %d", notices)
	}
}

// 18歳以上は宣言を求めず、記録も残さない。支払いに未成年の印も付かない。
func TestAdultPurchaseRecordsNotice(t *testing.T) {
	allowPayPayForTest(t)
	fakeCoachingCloudinary(t)
	h, _ := setupCoachChoice(t)
	admin := adminToken(t, h)
	coach, _ := onDutyCoach(t, h, admin)
	adult, tu := demoRequester(t, h, admin)
	body := reviewBodyFor(t, h, tu)
	body["method"] = "paypay"
	body["requested_coach_id"] = coach.String()
	rec := doJSON(t, h, "POST", "/v1/coaching/requests", tu, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("adult create %d %s", rec.Code, rec.Body.String())
	}
	var created struct{ ID string }
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	payChosenReview(t, h, created.ID, "paypay")
	var declarations, notices, minorPayments int
	queryRowArgs(t, `SELECT count(*) FROM minor_purchase_declarations WHERE minor_id = $1`, []any{adult}, &declarations)
	queryRowArgs(t, `SELECT count(*) FROM guardian_notices WHERE minor_id = $1`, []any{adult}, &notices)
	queryRowArgs(t, `SELECT count(*) FROM review_payments WHERE request_id = $1 AND minor`, []any{created.ID}, &minorPayments)
	if declarations != 1 || notices != 0 || minorPayments != 0 {
		t.Fatalf("adult declarations=%d notices=%d minor payments=%d", declarations, notices, minorPayments)
	}
	var me struct {
		IsMinor bool `json:"is_minor"`
	}
	_ = json.Unmarshal(doJSON(t, h, "GET", "/v1/me/coaching", tu, nil).Body.Bytes(), &me)
	if me.IsMinor {
		t.Fatal("adult reported as minor")
	}
}

// 追加相談の支払いが届いた時に宣言の記録がなければ（宣言の前の支払い画面など）、相談を始めずに返金する。
func TestReviewConsultationPaymentDoesNotRecheckLegacyDeclaration(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupCoachChoice(t)
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)
	_, tm := demoUser(t, h, time.Date(time.Now().Year()-15, 1, 1, 0, 0, 0, 0, time.UTC))
	review := chosenReview(t, h, tm, coach.String(), "paypay", false)
	payChosenReview(t, h, review, "paypay")
	for _, o := range coachRequests(t, h, tc).Offers {
		if r := doJSON(t, h, "POST", "/v1/coach/offers/"+o.ID+"/accept", tc, nil); r.Code != 200 {
			t.Fatal(r.Body.String())
		}
	}
	if r := doJSON(t, h, "POST", "/v1/coaching/requests/"+review+"/response", tc, responseBody()); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	c := consultationDraft(t, h, tm, "/v1/coaching/requests/"+review+"/consultations", uuid.NewString(), "練習の量はどれくらいがよいですか")
	if rec := doJSON(t, h, "POST", "/v1/coaching/consultations/"+c.ID+"/submit", tm, map[string]any{"guardian_permission": true}); rec.Code != 200 {
		t.Fatalf("submit %d %s", rec.Code, rec.Body.String())
	}
	execSQLArgs(t, `DELETE FROM minor_purchase_declarations WHERE ref_type = 'consultation' AND ref_id = $1`, c.ID)
	if code := payConsultation(t, h, c.ID, 600); code != 200 {
		t.Fatalf("pay %d", code)
	}
	var status string
	queryRowArgs(t, `SELECT status FROM coaching_consultations WHERE id = $1`, []any{c.ID}, &status)
	if status != "offered" {
		t.Fatalf("consultation without declaration %s", status)
	}
	// 返金は支払いと同じ処理で予約する（Stripe への依頼は tick が行う）。
	var payment string
	queryRowArgs(t, `SELECT payment_state FROM coaching_consultations WHERE id = $1`, []any{c.ID}, &payment)
	if payment != "paid" {
		t.Fatalf("payment state %s", payment)
	}
}

// 未成年の購入の返金は、管理画面の返金 API で返金した分を含めて、件数と金額を後から集計できる。
func TestMinorRefundSummary(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupCoachChoice(t)
	admin := adminToken(t, h)
	coach, _ := onDutyCoach(t, h, admin)
	_, tm := demoUser(t, h, time.Date(time.Now().Year()-14, 1, 1, 0, 0, 0, 0, time.UTC))
	_, tu := demoRequester(t, h, admin)
	type summary struct {
		ReviewCount    int `json:"review_count"`
		ReviewAmount   int `json:"review_amount_yen"`
		ConsultCount   int `json:"consultation_count"`
		ConsultAmount  int `json:"consultation_amount_yen"`
		AdminCount     int `json:"admin_count"`
		AdminAmountYen int `json:"admin_amount_yen"`
	}
	read := func() (month, total summary) {
		t.Helper()
		rec := doJSON(t, h, "GET", "/v1/admin/coaching/payments", admin, nil)
		if rec.Code != 200 {
			t.Fatalf("payments %d %s", rec.Code, rec.Body.String())
		}
		var out struct {
			MinorRefunds struct {
				Month summary `json:"month"`
				Total summary `json:"total"`
			} `json:"minor_refunds"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return out.MinorRefunds.Month, out.MinorRefunds.Total
	}
	base, baseTotal := read()

	minorReview := chosenReview(t, h, tm, coach.String(), "card", false)
	payChosenReview(t, h, minorReview, "card")
	execSQLArgs(t, `UPDATE review_payments SET minor=true WHERE request_id=$1`, minorReview) // 旧方式の取引のみの集計を確認
	adultReview := chosenReview(t, h, tu, coach.String(), "card", false)
	payChosenReview(t, h, adultReview, "card")
	// 未成年の分を管理画面から、大人の分を自動で返金した状態にする。大人の分は数えない。
	var minorPayment string
	queryRowArgs(t, `SELECT id::text FROM review_payments WHERE request_id = $1`, []any{minorReview}, &minorPayment)
	execSQLArgs(t, `UPDATE review_payments SET status = 'refunded', captured_at = now(), refunded_at = now() WHERE request_id IN ($1, $2)`, minorReview, adultReview)
	execSQLArgs(t, `INSERT INTO captured_payment_refunds (id, source, payment_id, payment_intent_id, amount_yen, reason, origin, status)
		VALUES (gen_random_uuid(), 'review', $1, 'pi_minor_refund', 500, '保護者から許可のない購入との申立て', 'admin', 'succeeded')`, minorPayment)

	month, total := read()
	if month.ReviewCount-base.ReviewCount != 1 || month.ReviewAmount-base.ReviewAmount != 500 ||
		month.AdminCount-base.AdminCount != 1 || month.AdminAmountYen-base.AdminAmountYen != 500 ||
		total.ReviewCount-baseTotal.ReviewCount != 1 {
		t.Fatalf("summary month=%+v total=%+v base=%+v", month, total, base)
	}
}
