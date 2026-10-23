package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

type consultationView struct {
	ID, Status, Funding, Question, Answer string
	RequestKey                            string `json:"request_key"`
	Price                                 int    `json:"price_yen"`
	Payment                               string `json:"payment_state"`
}

func consultationAction(t *testing.T, h http.Handler, tok, id, action string, body any) consultationView {
	t.Helper()
	r := doJSON(t, h, "POST", "/v1/coaching/consultations/"+id+"/"+action, tok, body)
	if r.Code != 200 {
		t.Fatalf("consultation %s %d %s", action, r.Code, r.Body.String())
	}
	var out struct{ Consultation consultationView }
	if err := json.Unmarshal(r.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out.Consultation
}
func consultationDraft(t *testing.T, h http.Handler, tok, path, key, body string) consultationView {
	t.Helper()
	r := doJSON(t, h, "POST", path, tok, map[string]any{"request_key": key, "body": body})
	if r.Code != 200 {
		t.Fatalf("draft %d %s", r.Code, r.Body.String())
	}
	var out struct{ Consultation consultationView }
	if err := json.Unmarshal(r.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out.Consultation
}
func answeredTicketRoom(t *testing.T, h http.Handler) (owner, coach, root string) {
	t.Helper()
	admin := adminToken(t, h)
	_, coach = onDutyCoach(t, h, admin)
	_, owner = demoRequester(t, h, admin)
	buyFiveTickets(t, h, owner)
	root = ticketReview(t, h, owner)
	offers := coachRequests(t, h, coach).Offers
	if len(offers) != 1 {
		t.Fatalf("offers %+v", offers)
	}
	if r := doJSON(t, h, "POST", "/v1/coach/offers/"+offers[0].ID+"/accept", coach, nil); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if r := doJSON(t, h, "POST", "/v1/coaching/requests/"+root+"/response", coach, responseBody()); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	return
}
func endedCashRoom(t *testing.T, h http.Handler) (owner, coach, root string) {
	t.Helper()
	owner, coach, root = adultAccepted(t, h)
	execSQLArgs(t, `UPDATE coaching_contracts SET status='ended',starts_at=now()-interval '8 days',ends_at=now()-interval '1 day',additional_consultation_price_yen=1200 WHERE id=$1`, root)
	return
}
func payConsultation(t *testing.T, h http.Handler, id string, amount int) int {
	t.Helper()
	var session string
	queryRowArgs(t, `SELECT checkout_session_id FROM coaching_consultations WHERE id=$1`, []any{id}, &session)
	r := stripeWebhook(t, h, testWebhookSecret, "checkout.session.completed", map[string]any{"id": session, "payment_status": "paid", "payment_intent": "pi_consult_" + id, "client_reference_id": id, "amount_total": amount, "currency": "jpy", "metadata": map[string]string{"kind": "consultation", "consultation_id": id}})
	return r.Code
}
func TestConsultationTicketConversationAndFreeConfirmation(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupStripe(t)
	owner, coach, root := answeredTicketRoom(t, h)
	path := "/v1/coaching/requests/" + root + "/consultations"
	c := consultationDraft(t, h, owner, path, uuid.NewString(), "次の大会ではどこを意識しますか")
	if c.Price != 500 || c.Funding != "ticket" {
		t.Fatalf("snapshot %+v", c)
	}
	consultationAction(t, h, owner, c.ID, "submit", nil)
	consultationAction(t, h, owner, c.ID, "submit", nil)
	if b := ticketSummary(t, h, owner).Balance; b.Available != 2 || b.Reserved != 1 {
		t.Fatalf("reserved %+v", b)
	}
	consultationAction(t, h, coach, c.ID, "accept", nil)
	consultationAction(t, h, coach, c.ID, "answer", map[string]any{"body": "前半の接地を意識してください"})
	if b := ticketSummary(t, h, owner).Balance; b.Available != 2 || b.Reserved != 0 {
		t.Fatalf("settled %+v", b)
	}
	if r := doJSON(t, h, "POST", "/v1/coaching/consultations/"+c.ID+"/answer", coach, map[string]any{"body": "二重回答"}); r.Code == 200 {
		t.Fatal("double answer")
	}
	var count, earnings int
	queryRowArgs(t, `SELECT count(*),sum(amount_yen)::integer FROM coach_earnings WHERE consultation_id=$1`, []any{c.ID}, &count, &earnings)
	if count != 1 || earnings != 350 {
		t.Fatalf("earnings %d %d", count, earnings)
	}
	consultationAction(t, h, owner, c.ID, "clarify", map[string]any{"body": "接地の練習方法を教えてください"})
	consultationAction(t, h, coach, c.ID, "clarification", map[string]any{"body": "その場での腿上げで確認できます"})
	if r := doJSON(t, h, "POST", "/v1/coaching/consultations/"+c.ID+"/clarify", owner, map[string]any{"body": "もう一度"}); r.Code == 200 {
		t.Fatal("second free question")
	}
	if b := ticketSummary(t, h, owner).Balance; b.Available != 2 {
		t.Fatalf("charged free confirmation %+v", b)
	}
	room := doJSON(t, h, "GET", path, owner, nil)
	if room.Code != 200 || !strings.Contains(room.Body.String(), "その場での腿上げ") {
		t.Fatal(room.Body.String())
	}
	if len(f.refunds) != 0 {
		t.Fatal("unexpected cash refund")
	}
	assertTicketAccounting(t, h, adminToken(t, h))
}
func TestConsultationDraftSurvivesTicketPurchaseAndConcurrentSubmit(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	owner, coach, root := answeredTicketRoom(t, h)
	// 初回添削後の残りを別の初回依頼で予約し、追加相談を一度送って不足させる。
	c0 := consultationDraft(t, h, owner, "/v1/coaching/requests/"+root+"/consultations", uuid.NewString(), "最初の相談")
	consultationAction(t, h, owner, c0.ID, "submit", nil)
	consultationAction(t, h, coach, c0.ID, "accept", nil)
	consultationAction(t, h, coach, c0.ID, "answer", map[string]any{"body": "回答です"})
	ticketReview(t, h, owner) // 残り2枚を確保
	path := "/v1/coaching/requests/" + root + "/consultations"
	key := uuid.NewString()
	c := consultationDraft(t, h, owner, path, key, "購入後も残る質問")
	if r := doJSON(t, h, "POST", "/v1/coaching/consultations/"+c.ID+"/submit", owner, nil); r.Code != 422 || !strings.Contains(r.Body.String(), "insufficient_tickets") {
		t.Fatalf("insufficient %d %s", r.Code, r.Body.String())
	}
	if r := doJSON(t, h, "GET", path, owner, nil); !strings.Contains(r.Body.String(), "購入後も残る質問") {
		t.Fatal(r.Body.String())
	}
	buyFiveTickets(t, h, owner)
	start := make(chan struct{})
	var wg sync.WaitGroup
	codes := make([]int, 2)
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			codes[i] = doJSON(t, h, "POST", "/v1/coaching/consultations/"+c.ID+"/submit", owner, nil).Code
		}(i)
	}
	close(start)
	wg.Wait()
	for _, code := range codes {
		if code != 200 {
			t.Fatalf("retry codes %v", codes)
		}
	}
	var n int
	queryRowArgs(t, `SELECT count(*) FROM coaching_ticket_reservations WHERE consultation_id=$1`, []any{c.ID}, &n)
	if n != 1 {
		t.Fatalf("duplicate reservation %d", n)
	}
	consultationAction(t, h, coach, c.ID, "decline", nil)
	if b := ticketSummary(t, h, owner).Balance; b.Available != 5 {
		t.Fatalf("declined ticket not released %+v", b)
	}
	assertTicketAccounting(t, h, adminToken(t, h))
}
func TestConsultationTicketDeadlineReturnsReservationOnce(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	owner, coach, root := answeredTicketRoom(t, h)
	c := consultationDraft(t, h, owner, "/v1/coaching/requests/"+root+"/consultations", uuid.NewString(), "期限付きの質問")
	consultationAction(t, h, owner, c.ID, "submit", nil)
	consultationAction(t, h, coach, c.ID, "accept", nil)
	execSQLArgs(t, `UPDATE coaching_consultations SET answer_by=now()-interval '1 minute' WHERE id=$1`, c.ID)
	if r := doJSON(t, h, "POST", "/v1/coaching/consultations/"+c.ID+"/answer", coach, map[string]any{"body": "遅い回答"}); r.Code == 200 {
		t.Fatal("late answer accepted")
	}
	for i := 0; i < 2; i++ {
		if r := tick(t, h, "tick-secret"); r.Code != 200 {
			t.Fatal(r.Body.String())
		}
	}
	if b := ticketSummary(t, h, owner).Balance; b.Available != 3 || b.Reserved != 0 {
		t.Fatalf("expired %+v", b)
	}
	assertTicketAccounting(t, h, adminToken(t, h))
}
func TestConsultationCashSnapshotWebhookAndRefund(t *testing.T) {
	h, f := setupStripe(t)
	owner, coach, root := endedCashRoom(t, h)
	c := consultationDraft(t, h, owner, "/v1/coaching/contracts/"+root+"/consultations", uuid.NewString(), "契約終了後の相談")
	execSQLArgs(t, `UPDATE coaching_plans SET additional_consultation_price_yen=2400 WHERE id=(SELECT plan_id FROM coaching_contracts WHERE id=$1)`, root)
	consultationAction(t, h, owner, c.ID, "submit", nil)
	consultationAction(t, h, owner, c.ID, "checkout", nil)
	if got := f.lastSession().Get("line_items[0][price_data][unit_amount]"); got != "1200" {
		t.Fatalf("price %s", got)
	}
	if code := payConsultation(t, h, c.ID, 1); code != 500 {
		t.Fatalf("wrong amount %d", code)
	}
	if code := payConsultation(t, h, c.ID, 1200); code != 200 {
		t.Fatalf("pay %d", code)
	}
	if code := payConsultation(t, h, c.ID, 1200); code != 200 {
		t.Fatalf("repeat %d", code)
	}
	consultationAction(t, h, coach, c.ID, "decline", nil)
	for i := 0; i < 2; i++ {
		if r := tick(t, h, "tick-secret"); r.Code != 200 {
			t.Fatal(r.Body.String())
		}
	}
	if len(f.refundList()) != 1 || f.refundList()[0].Get("amount") != "1200" {
		t.Fatalf("refunds %+v", f.refundList())
	}
	var state string
	queryRowArgs(t, `SELECT payment_state FROM coaching_consultations WHERE id=$1`, []any{c.ID}, &state)
	if state != "refunded" {
		t.Fatal(state)
	}
}
func TestConsultationLateCashPaymentRefundsInsteadOfReopening(t *testing.T) {
	h, f := setupStripe(t)
	owner, _, root := endedCashRoom(t, h)
	c := consultationDraft(t, h, owner, "/v1/coaching/contracts/"+root+"/consultations", uuid.NewString(), "取り下げる質問")
	consultationAction(t, h, owner, c.ID, "submit", nil)
	consultationAction(t, h, owner, c.ID, "cancel", nil)
	if code := payConsultation(t, h, c.ID, 1200); code != 200 {
		t.Fatalf("late pay %d", code)
	}
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	var status, state string
	queryRowArgs(t, `SELECT status,payment_state FROM coaching_consultations WHERE id=$1`, []any{c.ID}, &status, &state)
	if status != "expired" || state != "refunded" || len(f.refundList()) != 1 {
		t.Fatalf("late status %s %s %+v", status, state, f.refundList())
	}
}
func TestConsultationCashAnswerRecordsEarningsAndClarificationDeadline(t *testing.T) {
	h, _ := setupStripe(t)
	owner, coach, root := endedCashRoom(t, h)
	c := consultationDraft(t, h, owner, "/v1/coaching/contracts/"+root+"/consultations", uuid.NewString(), "指名の質問")
	consultationAction(t, h, owner, c.ID, "submit", nil)
	if code := payConsultation(t, h, c.ID, 1200); code != 200 {
		t.Fatal(code)
	}
	consultationAction(t, h, coach, c.ID, "accept", nil)
	consultationAction(t, h, coach, c.ID, "answer", map[string]any{"body": "指名の回答"})
	var amount int
	queryRowArgs(t, `SELECT amount_yen FROM coach_earnings WHERE consultation_id=$1`, []any{c.ID}, &amount)
	if amount != 960 {
		t.Fatalf("cash earnings %d", amount)
	}
	execSQLArgs(t, `UPDATE coaching_consultations SET answered_at=now()-interval '8 days' WHERE id=$1`, c.ID)
	if r := doJSON(t, h, "POST", "/v1/coaching/consultations/"+c.ID+"/clarify", owner, map[string]any{"body": "期限後の確認"}); r.Code == 200 {
		t.Fatal("late free confirmation")
	}
}
func TestConsultationPrivateReportsAndRetention(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	owner, coach, root := answeredTicketRoom(t, h)
	admin := adminToken(t, h)
	c := consultationDraft(t, h, owner, "/v1/coaching/requests/"+root+"/consultations", uuid.NewString(), "非公開の質問")
	_, outsider := demoUser(t, h, time.Date(1993, 1, 1, 0, 0, 0, 0, time.UTC))
	if r := doJSON(t, h, "GET", "/v1/coaching/requests/"+root+"/consultations", outsider, nil); r.Code != 404 {
		t.Fatalf("privacy %d", r.Code)
	}
	if r := doJSON(t, h, "POST", "/v1/coaching/consultations/"+c.ID+"/accept", coach, nil); r.Code != 404 {
		t.Fatalf("coach draft privacy %d", r.Code)
	}
	consultationAction(t, h, owner, c.ID, "submit", nil)
	consultationAction(t, h, coach, c.ID, "accept", nil)
	consultationAction(t, h, coach, c.ID, "answer", map[string]any{"body": "非公開の回答"})
	adminPath := "/v1/admin/coaching/consultations/" + c.ID
	if r := doJSON(t, h, "GET", adminPath, admin, nil); r.Code != 403 {
		t.Fatalf("admin needs report %d", r.Code)
	}
	if r := doJSON(t, h, "GET", "/v1/admin/coaching/consultations", admin, nil); strings.Contains(r.Body.String(), "非公開の質問") {
		t.Fatal("metadata leaks content")
	}
	rec := doJSON(t, h, "POST", "/v1/reports", owner, map[string]any{"target_type": "coaching_consultation", "target_id": c.ID, "reason": "other", "detail": "確認してください"})
	if rec.Code != 201 {
		t.Fatal(rec.Body.String())
	}
	var reportID string
	queryRowArgs(t, `SELECT id::text FROM reports WHERE target_id=$1`, []any{c.ID}, &reportID)
	if r := doJSON(t, h, "GET", adminPath+"?report="+reportID, admin, nil); r.Code != 200 || !strings.Contains(r.Body.String(), "非公開の回答") {
		t.Fatal(r.Body.String())
	}
	var logs int
	queryRowArgs(t, `SELECT count(*) FROM coaching_consultation_access_logs WHERE consultation_id=$1`, []any{c.ID}, &logs)
	if logs != 1 {
		t.Fatalf("access logs %d", logs)
	}
	execSQLArgs(t, `UPDATE coaching_consultations SET closed_at=now()-interval '13 months' WHERE id=$1`, c.ID)
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	var body string
	queryRowArgs(t, `SELECT question FROM coaching_consultations WHERE id=$1`, []any{c.ID}, &body)
	if body == "" {
		t.Fatal("open report purged")
	}
	if r := doJSON(t, h, "POST", "/v1/admin/reports/"+reportID+"/close", admin, nil); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	queryRowArgs(t, `SELECT question FROM coaching_consultations WHERE id=$1`, []any{c.ID}, &body)
	if body != "" {
		t.Fatal("old consultation retained")
	}
}

func TestConsultationGuardianPaymentAndMonthlyLimit(t *testing.T) {
	h, f := setupStripe(t)
	minor, guardian := guardianMinor(t, h)
	coach, root := minorAccepted(t, h, minor)
	gp := "/v1/guardian/" + guardian
	// 契約への保護者同意を経た終了済み契約を再現する。
	execSQLArgs(t, `INSERT INTO coaching_contract_consents(contract_id,party,profile_id,version,report_review,contact_detection) VALUES ($1,'guardian',NULL,1,true,true)`, root)
	execSQLArgs(t, `UPDATE coaching_contracts SET status='ended',starts_at=now()-interval '8 days',ends_at=now()-interval '1 day',additional_consultation_price_yen=1200 WHERE id=$1`, root)
	c := consultationDraft(t, h, minor, "/v1/coaching/contracts/"+root+"/consultations", uuid.NewString(), "保護者が支払う相談")
	r := doJSON(t, h, "POST", "/v1/coaching/consultations/"+c.ID+"/submit", minor, nil)
	if r.Code != 200 || strings.Contains(r.Body.String(), "checkout_url") {
		t.Fatalf("minor submit %d %s", r.Code, r.Body.String())
	}
	if r := doJSON(t, h, "POST", "/v1/coaching/consultations/"+c.ID+"/checkout", minor, nil); r.Code != 403 {
		t.Fatalf("child checkout %d", r.Code)
	}
	// 別の未成年の保護者は閲覧・支払いとも不可。
	_, other := guardianMinor(t, h)
	if r := doJSON(t, h, "POST", "/v1/guardian/"+other+"/consultations/"+c.ID+"/checkout", "", nil); r.Code != 404 {
		t.Fatalf("other guardian %d", r.Code)
	}
	if r := doJSON(t, h, "PATCH", gp, "", map[string]any{"monthly_limit_yen": 1000, "allow_publish": false}); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if r := doJSON(t, h, "POST", gp+"/consultations/"+c.ID+"/checkout", "", nil); r.Code != 422 || !strings.Contains(r.Body.String(), "monthly_limit") {
		t.Fatalf("limit %d %s", r.Code, r.Body.String())
	}
	if r := doJSON(t, h, "PATCH", gp, "", map[string]any{"monthly_limit_yen": 3000, "allow_publish": false}); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	r = doJSON(t, h, "POST", gp+"/consultations/"+c.ID+"/checkout", "", nil)
	if r.Code != 200 || !strings.Contains(r.Body.String(), "checkout_url") {
		t.Fatalf("guardian checkout %d %s", r.Code, r.Body.String())
	}
	if strings.Contains(f.lastSession().Encode(), guardian) {
		t.Fatal("guardian secret sent to payment provider")
	}
	if code := payConsultation(t, h, c.ID, 1200); code != 200 {
		t.Fatalf("guardian paid %d", code)
	}
	consultationAction(t, h, coach, c.ID, "accept", nil)
	consultationAction(t, h, coach, c.ID, "answer", map[string]any{"body": "保護者にも読める回答"})
	if r := doJSON(t, h, "GET", gp+"/contracts/"+root+"/consultations", "", nil); r.Code != 200 || !strings.Contains(r.Body.String(), "保護者にも読める回答") {
		t.Fatal(r.Body.String())
	}
	if r := doJSON(t, h, "POST", gp+"/consultations/"+c.ID+"/report", "", map[string]any{"reason": "other", "detail": "確認をお願いします"}); r.Code != 201 {
		t.Fatal(r.Body.String())
	}

	second := consultationDraft(t, h, minor, "/v1/coaching/contracts/"+root+"/consultations", uuid.NewString(), "同意取り消し後の決済")
	consultationAction(t, h, minor, second.ID, "submit", nil)
	if r := doJSON(t, h, "POST", gp+"/consultations/"+second.ID+"/checkout", "", nil); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if r := doJSON(t, h, "POST", gp+"/revoke", "", nil); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if code := payConsultation(t, h, second.ID, 1200); code != 200 {
		t.Fatal(code)
	}
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	var state string
	queryRowArgs(t, `SELECT payment_state FROM coaching_consultations WHERE id=$1`, []any{second.ID}, &state)
	if state != "refunded" {
		t.Fatalf("revoked guardian payment %s", state)
	}
}
func TestConsultationIncludedQuotaAndOwnedVideo(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setup(t)
	coach, owner, root := activeContract(t, h)
	execSQLArgs(t, `UPDATE coaching_contracts SET service_type='video_review',video_reviews=1,additional_consultation_price_yen=500 WHERE id=$1`, root)
	path := "/v1/coaching/contracts/" + root + "/consultations"
	if r := doJSON(t, h, "POST", path, owner, map[string]any{"request_key": uuid.NewString(), "body": "含有分がある質問"}); r.Code != 422 || !strings.Contains(r.Body.String(), "included_in_plan") {
		t.Fatalf("included %d %s", r.Code, r.Body.String())
	}
	video := signedCoachingVideo(t, h, owner)
	if r := doJSON(t, h, "POST", "/v1/coaching/contracts/"+root+"/messages", owner, map[string]any{"video_public_id": video}); r.Code != 201 {
		t.Fatal(r.Body.String())
	}
	if r := doJSON(t, h, "POST", "/v1/coaching/contracts/"+root+"/messages", owner, map[string]any{"video_public_id": signedCoachingVideo(t, h, owner)}); r.Code != 422 || !strings.Contains(r.Body.String(), "video_limit") {
		t.Fatalf("quota %d %s", r.Code, r.Body.String())
	}
	if r := doJSON(t, h, "POST", "/v1/coaching/contracts/"+root+"/messages", owner, map[string]any{"body": "期間内の文章は無料"}); r.Code != 201 {
		t.Fatal(r.Body.String())
	}
	if r := doJSON(t, h, "POST", path, owner, map[string]any{"request_key": uuid.NewString(), "body": "追加の文章"}); r.Code != 422 || !strings.Contains(r.Body.String(), "video_required") {
		t.Fatalf("text upsell %d %s", r.Code, r.Body.String())
	}
	if r := doJSON(t, h, "POST", path, owner, map[string]any{"request_key": uuid.NewString(), "body": "流用した動画", "video_public_id": video}); r.Code != 422 {
		t.Fatalf("video reuse %d", r.Code)
	}
	if r := doJSON(t, h, "POST", path, owner, map[string]any{"request_key": uuid.NewString(), "body": "他人の動画", "video_public_id": signedCoachingVideo(t, h, coach)}); r.Code != 422 {
		t.Fatalf("foreign video %d", r.Code)
	}
	if r := doJSON(t, h, "POST", path, owner, map[string]any{"request_key": uuid.NewString(), "body": "追加の動画", "video_public_id": signedCoachingVideo(t, h, owner)}); r.Code != 200 {
		t.Fatalf("own video %d %s", r.Code, r.Body.String())
	}
}
func TestConsultationFailedRefundCanBeRetriedAndDraftPurges(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupStripe(t)
	owner, coach, root := endedCashRoom(t, h)
	admin := adminToken(t, h)
	path := "/v1/coaching/contracts/" + root + "/consultations"
	c := consultationDraft(t, h, owner, path, uuid.NewString(), "返金の再試行")
	consultationAction(t, h, owner, c.ID, "submit", nil)
	if code := payConsultation(t, h, c.ID, 1200); code != 200 {
		t.Fatal(code)
	}
	consultationAction(t, h, coach, c.ID, "decline", nil)
	f.ticketRefundStatus = "failed"
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	f.ticketRefundStatus = "succeeded"
	if r := doJSON(t, h, "POST", "/v1/admin/coaching/consultations/"+c.ID+"/refund", admin, nil); r.Code != 202 {
		t.Fatal(r.Body.String())
	}
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	var state string
	queryRowArgs(t, `SELECT payment_state FROM coaching_consultations WHERE id=$1`, []any{c.ID}, &state)
	if state != "refunded" || len(f.refundList()) != 2 {
		t.Fatalf("retry %s %+v", state, f.refundList())
	}
	draft := consultationDraft(t, h, owner, path, uuid.NewString(), "期限切れの下書き")
	execSQLArgs(t, `UPDATE coaching_consultations SET expires_at=now()-interval '1 minute' WHERE id=$1`, draft.ID)
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	var purged bool
	queryRowArgs(t, `SELECT purged_at IS NOT NULL FROM coaching_consultations WHERE id=$1`, []any{draft.ID}, &purged)
	if !purged {
		t.Fatal("expired draft not purged")
	}
}
func TestConsultationReportHideStopsAndReleasesTicket(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	owner, coach, root := answeredTicketRoom(t, h)
	admin := adminToken(t, h)
	c := consultationDraft(t, h, owner, "/v1/coaching/requests/"+root+"/consultations", uuid.NewString(), "通報で停止する相談")
	consultationAction(t, h, owner, c.ID, "submit", nil)
	r := doJSON(t, h, "POST", "/v1/reports", coach, map[string]any{"target_type": "coaching_consultation", "target_id": c.ID, "reason": "other", "detail": "対応をお願いします"})
	if r.Code != 201 {
		t.Fatal(r.Body.String())
	}
	var report string
	queryRowArgs(t, `SELECT id::text FROM reports WHERE target_id=$1`, []any{c.ID}, &report)
	if r := doJSON(t, h, "POST", "/v1/admin/coaching/consultations/"+c.ID+"/hide?report="+report, admin, nil); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if b := ticketSummary(t, h, owner).Balance; b.Available != 3 || b.Reserved != 0 {
		t.Fatalf("hide refund %+v", b)
	}
	if r := doJSON(t, h, "POST", "/v1/coaching/consultations/"+c.ID+"/accept", coach, nil); r.Code == 200 {
		t.Fatal("hidden action")
	}
	if r := doJSON(t, h, "GET", "/v1/coaching/requests/"+root+"/consultations", owner, nil); strings.Contains(r.Body.String(), "通報で停止する相談") {
		t.Fatal("hidden content leaked")
	}
}

func TestConsultationRetainsWholeRoomUntilLastConsultationAgesOut(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	owner, coach, root := endedCashRoom(t, h)
	path := "/v1/coaching/contracts/" + root + "/consultations"
	answer := func(body string) string {
		t.Helper()
		c := consultationDraft(t, h, owner, path, uuid.NewString(), body)
		consultationAction(t, h, owner, c.ID, "submit", nil)
		if code := payConsultation(t, h, c.ID, 1200); code != 200 {
			t.Fatal(code)
		}
		consultationAction(t, h, coach, c.ID, "accept", nil)
		consultationAction(t, h, coach, c.ID, "answer", map[string]any{"body": "保存される回答"})
		return c.ID
	}
	older := answer("古い相談")
	execSQLArgs(t, `UPDATE coaching_contracts SET starts_at=now()-interval '14 months',ends_at=now()-interval '13 months' WHERE id=$1`, root)
	execSQLArgs(t, `UPDATE coaching_consultations SET closed_at=now()-interval '13 months' WHERE id=$1`, older)
	latest := answer("新しい相談")
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	var purged bool
	queryRowArgs(t, `SELECT purged_at IS NOT NULL FROM coaching_consultations WHERE id=$1`, []any{older}, &purged)
	if purged {
		t.Fatal("older conversation purged while room active")
	}
	queryRowArgs(t, `SELECT messages_purged_at IS NOT NULL FROM coaching_contracts WHERE id=$1`, []any{root}, &purged)
	if purged {
		t.Fatal("base conversation purged")
	}
	execSQLArgs(t, `UPDATE coaching_consultations SET closed_at=now()-interval '13 months' WHERE id=$1`, latest)
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	queryRowArgs(t, `SELECT messages_purged_at IS NOT NULL FROM coaching_contracts WHERE id=$1`, []any{root}, &purged)
	if !purged {
		t.Fatal("aged room retained")
	}
}
func TestConsultationGuardianCanFindRandomReviewRoom(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	admin := adminToken(t, h)
	_, coach := onDutyCoach(t, h, admin)
	minor, g := guardianMinor(t, h)
	buyFiveTickets(t, h, minor)
	root := ticketReview(t, h, minor)
	offers := coachRequests(t, h, coach).Offers
	if r := doJSON(t, h, "POST", "/v1/coach/offers/"+offers[0].ID+"/accept", coach, nil); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if r := doJSON(t, h, "POST", "/v1/coaching/requests/"+root+"/response", coach, responseBody()); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	c := consultationDraft(t, h, minor, "/v1/coaching/requests/"+root+"/consultations", uuid.NewString(), "保護者にも見える相談")
	consultationAction(t, h, minor, c.ID, "submit", nil)
	gp := "/v1/guardian/" + g
	if r := doJSON(t, h, "GET", gp+"/consultation-reviews", "", nil); r.Code != 200 || !strings.Contains(r.Body.String(), root) {
		t.Fatalf("guardian rooms %d %s", r.Code, r.Body.String())
	}
	if r := doJSON(t, h, "GET", gp+"/reviews/"+root+"/consultations", "", nil); r.Code != 200 || !strings.Contains(r.Body.String(), "保護者にも見える相談") {
		t.Fatal(r.Body.String())
	}
	if r := doJSON(t, h, "POST", gp+"/consultations/"+c.ID+"/report", "", map[string]any{"reason": "other", "detail": "回答前でも子の相談を確認したい"}); r.Code != 201 {
		t.Fatalf("guardian report before answer %d %s", r.Code, r.Body.String())
	}
}
func TestConsultationPreventsConcurrentCrossRoomVideoReuse(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setup(t)
	_, owner, root := activeContract(t, h)
	// 同じ動画が通常チャットと終了後の追加相談に同時に使われても、片方だけが保存できる。
	execSQLArgs(t, `UPDATE coaching_contracts SET additional_consultation_price_yen=500,service_type='video_review',video_reviews=1 WHERE id=$1`, root)
	video := signedCoachingVideo(t, h, owner)
	// 別の終了済み契約を用意する。同じ動画の所有者は同一。
	other := uuid.NewString()
	execSQLArgs(t, `INSERT INTO coaching_contracts(id,plan_id,coach_id,requester_id,plan_kind,plan_title,duration_days,price_yen,event,goal,current_record,message,minor,status,respond_by,starts_at,ends_at,additional_consultation_price_yen) SELECT $1,plan_id,coach_id,requester_id,plan_kind,plan_title,duration_days,price_yen,event,goal,current_record,message,minor,'ended',respond_by,now()-interval '8 days',now()-interval '1 day',500 FROM coaching_contracts WHERE id=$2`, other, root)
	start := make(chan struct{})
	codes := make([]int, 2)
	var wg sync.WaitGroup
	paths := []string{"/v1/coaching/contracts/" + root + "/messages", "/v1/coaching/contracts/" + other + "/consultations"}
	for i := range paths {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			body := map[string]any{"body": "同じ動画", "video_public_id": video}
			if i == 1 {
				body["request_key"] = uuid.NewString()
			}
			codes[i] = doJSON(t, h, "POST", paths[i], owner, body).Code
		}(i)
	}
	close(start)
	wg.Wait()
	success := 0
	for _, code := range codes {
		if code == 200 || code == 201 {
			success++
		} else if code != 422 {
			t.Fatalf("unexpected codes %v", codes)
		}
	}
	if success != 1 {
		t.Fatalf("video stored twice %v", codes)
	}
}
