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

func supportPlan(t *testing.T, h http.Handler) (coach, plan string) {
	t.Helper()
	_, coach, plan = planCoach(t, h, 5)
	r := doJSON(t, h, "PATCH", "/v1/coach/me/services", coach, map[string]any{"service_types": []string{"video_review", "competition_support"}})
	if r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	body := planBody()
	body["service_type"] = "competition_support"
	body["competition_name"] = "秋季記録会"
	body["support_start_date"] = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	body["competition_date"] = time.Now().AddDate(0, 0, 14).Format("2006-01-02")
	body["video_reviews"] = 2
	body["call_count"] = 1
	body["call_minutes"] = 30
	body["price_yen"] = 9000
	r = doJSON(t, h, "PATCH", "/v1/coach/me/plans/"+plan, coach, body)
	if r.Code != 200 {
		t.Fatalf("support plan %d %s", r.Code, r.Body.String())
	}
	return
}
func applySupport(t *testing.T, h http.Handler, owner, coach, plan string) string {
	t.Helper()
	r := doJSON(t, h, "POST", "/v1/coaching/contracts", owner, contractBody(plan))
	if r.Code != 201 {
		t.Fatalf("support apply %d %s", r.Code, r.Body.String())
	}
	id := decodeContract(t, r.Body.Bytes()).ID
	r = doJSON(t, h, "POST", "/v1/coach/contracts/"+id+"/accept", coach, map[string]any{"agree_report": true, "agree_detection": true})
	if r.Code != 200 {
		t.Fatalf("support accept %d %s", r.Code, r.Body.String())
	}
	return id
}
func paySupport(t *testing.T, h http.Handler, root string) {
	t.Helper()
	var sid, pid string
	queryRowArgs(t, `SELECT checkout_session_id,id::text FROM coaching_payments WHERE contract_id=$1 ORDER BY created_at DESC LIMIT 1`, []any{root}, &sid, &pid)
	r := stripeWebhook(t, h, testWebhookSecret, "checkout.session.completed", map[string]any{"id": sid, "payment_status": "paid", "payment_intent": "pi_support_" + root, "amount_total": 9000, "currency": "jpy", "client_reference_id": pid})
	if r.Code != 200 {
		t.Fatalf("support pay %d %s", r.Code, r.Body.String())
	}
}
func activeSupport(t *testing.T, h http.Handler, paid bool) (owner, coach, root string) {
	t.Helper()
	coach, plan := supportPlan(t, h)
	_, owner = demoUser(t, h, time.Date(1995, 1, 1, 0, 0, 0, 0, time.UTC))
	root = applySupport(t, h, owner, coach, plan)
	if paid {
		checkout(t, h, owner, root)
		paySupport(t, h, root)
	}
	return
}
func proposeSupport(t *testing.T, h http.Handler, coach, root string, at time.Time) string {
	t.Helper()
	r := doJSON(t, h, "POST", "/v1/coaching/contracts/"+root+"/calls", coach, map[string]any{"request_key": uuid.NewString(), "starts_at": at.Format(time.RFC3339Nano), "join_url": "https://meet.google.com/abc-defg-hij"})
	if r.Code != 200 {
		t.Fatalf("propose %d %s", r.Code, r.Body.String())
	}
	var out struct{ Call struct{ ID string } }
	if err := json.Unmarshal(r.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out.Call.ID
}
func callAction(t *testing.T, h http.Handler, tok, root, id, action string) {
	t.Helper()
	r := doJSON(t, h, "POST", "/v1/coaching/contracts/"+root+"/calls/"+id+"/"+action, tok, nil)
	if r.Code != 200 {
		t.Fatalf("call %s %d %s", action, r.Code, r.Body.String())
	}
}
func supportMessage(t *testing.T, h http.Handler, tok, root string, body map[string]any) string {
	t.Helper()
	r := doJSON(t, h, "POST", "/v1/coaching/contracts/"+root+"/messages", tok, body)
	if r.Code != 201 {
		t.Fatalf("support message %d %s", r.Code, r.Body.String())
	}
	var out struct{ Message struct{ ID string } }
	if err := json.Unmarshal(r.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out.Message.ID
}
func TestSupportPlanSnapshotFixedDatesAndPublication(t *testing.T) {
	h := setup(t)
	owner, coach, root := activeSupport(t, h, false)
	r := doJSON(t, h, "GET", "/v1/coaching/contracts/"+root, owner, nil)
	if r.Code != 200 || !strings.Contains(r.Body.String(), `"fulfillment_version":1`) || !strings.Contains(r.Body.String(), "秋季記録会") {
		t.Fatal(r.Body.String())
	}
	var actual, expected time.Time
	queryRowArgs(t, `SELECT ends_at,(competition_date+1)::timestamp AT TIME ZONE 'Asia/Tokyo' FROM coaching_contracts WHERE id=$1`, []any{root}, &actual, &expected)
	if !actual.Equal(expected) {
		t.Fatalf("fixed end %s %s", actual, expected)
	}
	execSQLArgs(t, `UPDATE coaching_plans SET competition_name='別の大会',call_count=3 WHERE id=(SELECT plan_id FROM coaching_contracts WHERE id=$1)`, root)
	r = doJSON(t, h, "GET", "/v1/coaching/contracts/"+root, owner, nil)
	if !strings.Contains(r.Body.String(), "秋季記録会") || !strings.Contains(r.Body.String(), `"call_count":1`) {
		t.Fatal("contract terms changed")
	}
	if r := doJSON(t, h, "POST", "/v1/coaching/contracts/"+root+"/messages", coach, map[string]any{"body": "https://meet.google.com/abc-defg-hij"}); r.Code != 422 {
		t.Fatalf("chat URL bypass %d", r.Code)
	}
}
func TestSupportVideoAnswersConsumeOnlyMatchingPair(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setup(t)
	owner, coach, root := activeSupport(t, h, false)
	question := supportMessage(t, h, owner, root, map[string]any{"video_public_id": signedCoachingVideo(t, h, owner)})
	supportMessage(t, h, coach, root, map[string]any{"body": "事務連絡です"})
	var n int
	queryRowArgs(t, `SELECT count(*) FROM coaching_video_reviews WHERE contract_id=$1 AND answer_id IS NOT NULL`, []any{root}, &n)
	if n != 0 {
		t.Fatal("administration consumed video quota")
	}
	supportMessage(t, h, coach, root, map[string]any{"body": "動画への具体的な回答", "review_submission_id": question})
	for _, tok := range []string{owner, coach} {
		r := doJSON(t, h, "POST", "/v1/coaching/contracts/"+root+"/messages", tok, map[string]any{"body": "二重回答", "review_submission_id": question})
		if r.Code != 422 {
			t.Fatalf("duplicate/unauthorized answer %d %s", r.Code, r.Body.String())
		}
	}
	r := doJSON(t, h, "GET", "/v1/coaching/contracts/"+root+"/support", owner, nil)
	if !strings.Contains(r.Body.String(), `"video_answered":1`) {
		t.Fatal(r.Body.String())
	}
}
func TestSupportCallsRequireParticipantConfirmationAndReturnQuota(t *testing.T) {
	h := setup(t)
	owner, coach, root := activeSupport(t, h, false)
	at := time.Now().Add(time.Hour)
	id := proposeSupport(t, h, coach, root, at)
	if r := doJSON(t, h, "POST", "/v1/coaching/contracts/"+root+"/calls/"+id+"/confirm", coach, nil); r.Code != 422 {
		t.Fatal("coach confirmed own call")
	}
	callAction(t, h, owner, root, id, "confirm")
	if r := doJSON(t, h, "POST", "/v1/coaching/contracts/"+root+"/calls/"+id+"/report-completed", coach, nil); r.Code != 422 {
		t.Fatal("future completion")
	}
	execSQLArgs(t, `UPDATE coaching_calls SET starts_at=now()-interval '1 hour',ends_at=now()-interval '30 minutes' WHERE id=$1`, id)
	callAction(t, h, coach, root, id, "report-completed")
	var n int
	queryRowArgs(t, `SELECT count(*) FROM coaching_calls WHERE contract_id=$1 AND status='completed'`, []any{root}, &n)
	if n != 0 {
		t.Fatal("unconfirmed service counted")
	}
	callAction(t, h, owner, root, id, "cancel")
	id = proposeSupport(t, h, coach, root, at.Add(time.Hour))
	callAction(t, h, owner, root, id, "confirm")
	execSQLArgs(t, `UPDATE coaching_calls SET starts_at=now()-interval '1 hour',ends_at=now()-interval '30 minutes' WHERE id=$1`, id)
	callAction(t, h, coach, root, id, "report-completed")
	callAction(t, h, owner, root, id, "confirm-completed")
	if r := doJSON(t, h, "POST", "/v1/coaching/contracts/"+root+"/calls/"+id+"/cancel", owner, nil); r.Code != 422 {
		t.Fatal("completed call refunded quota")
	}
}
func TestSupportCallPrivacyURLsAndOverlappingReservations(t *testing.T) {
	h := setup(t)
	owner, coach, root := activeSupport(t, h, false)
	_, other := demoUser(t, h, time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC))
	if r := doJSON(t, h, "GET", "/v1/coaching/contracts/"+root+"/support", other, nil); r.Code != 404 {
		t.Fatal("private calls leaked")
	}
	for _, url := range []string{"https://evil.test/j/123456789", "https://zoom.us.evil.test/j/123456789", "https://meet.google.com@evil.test/abc-defg-hij", "http://meet.google.com/abc-defg-hij", "https://zoom.us/j/123456789?redirect=https://evil.test"} {
		r := doJSON(t, h, "POST", "/v1/coaching/contracts/"+root+"/calls", coach, map[string]any{"request_key": uuid.NewString(), "starts_at": time.Now().Add(time.Hour), "join_url": url})
		if r.Code != 422 {
			t.Fatalf("bad URL %s %d", url, r.Code)
		}
	}
	execSQLArgs(t, `UPDATE coaching_contracts SET call_count=2 WHERE id=$1`, root)
	start := make(chan struct{})
	codes := make([]int, 2)
	var wg sync.WaitGroup
	at := time.Now().Add(time.Hour)
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			codes[i] = doJSON(t, h, "POST", "/v1/coaching/contracts/"+root+"/calls", coach, map[string]any{"request_key": uuid.NewString(), "starts_at": at, "join_url": "https://zoom.us/j/123456789?pwd=secret"}).Code
		}(i)
	}
	close(start)
	wg.Wait()
	successes := 0
	for _, code := range codes {
		if code == 200 {
			successes++
		} else if code != 422 {
			t.Fatalf("race %v", codes)
		}
	}
	if successes != 1 {
		t.Fatalf("overlapping calls %v", codes)
	}
	r := doJSON(t, h, "GET", "/v1/coaching/contracts/"+root+"/messages", owner, nil)
	if strings.Contains(r.Body.String(), "pwd=secret") {
		t.Fatal("call URL in ordinary chat")
	}
}
func TestSupportUnprovidedRefundAndNetEarnings(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupStripe(t)
	owner, coach, root := activeSupport(t, h, true)
	question := supportMessage(t, h, owner, root, map[string]any{"video_public_id": signedCoachingVideo(t, h, owner)})
	supportMessage(t, h, coach, root, map[string]any{"body": "提供済みの動画回答", "review_submission_id": question})
	// 動画2組と通話1回の合計3単位のうち1組だけ提供済み。終了後48時間は確認猶予。
	execSQLArgs(t, `UPDATE coaching_contracts SET ends_at=now()-interval '1 hour' WHERE id=$1`, root)
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if len(f.refundList()) != 0 {
		t.Fatal("refund before grace")
	}
	var n int
	queryRowArgs(t, `SELECT count(*) FROM coach_earnings WHERE contract_id=$1`, []any{root}, &n)
	if n != 0 {
		t.Fatal("earnings before settlement")
	}
	execSQLArgs(t, `UPDATE coaching_contracts SET ends_at=now()-interval '49 hours' WHERE id=$1`, root)
	for i := 0; i < 2; i++ {
		if r := tick(t, h, "tick-secret"); r.Code != 200 {
			t.Fatal(r.Body.String())
		}
	}
	if len(f.refundList()) != 1 || f.refundList()[0].Get("amount") != "6000" {
		t.Fatalf("refund %+v", f.refundList())
	}
	var amount int
	queryRowArgs(t, `SELECT amount_yen FROM coach_earnings WHERE contract_id=$1`, []any{root}, &amount)
	if amount != 2400 {
		t.Fatalf("net earnings %d", amount)
	}
	// 保存期間で質問・回答が消えても提供数は会計情報として残す。
	execSQLArgs(t, `UPDATE coaching_contracts SET ends_at=now()-interval '13 months' WHERE id=$1`, root)
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	r := doJSON(t, h, "GET", "/v1/coaching/contracts/"+root+"/support", owner, nil)
	if !strings.Contains(r.Body.String(), `"video_answered":1`) {
		t.Fatal("delivery accounting lost on purge: " + r.Body.String())
	}
}
func TestSupportCompletedCallPreventsSilentRefund(t *testing.T) {
	h, f := setupStripe(t)
	owner, coach, root := activeSupport(t, h, true)
	id := proposeSupport(t, h, coach, root, time.Now().Add(time.Hour))
	callAction(t, h, owner, root, id, "confirm")
	execSQLArgs(t, `UPDATE coaching_calls SET starts_at=now()-interval '1 hour',ends_at=now()-interval '30 minutes' WHERE id=$1`, id)
	callAction(t, h, coach, root, id, "report-completed")
	callAction(t, h, owner, root, id, "confirm-completed")
	execSQLArgs(t, `UPDATE coaching_contracts SET starts_at=now()-interval '4 days' WHERE id=$1`, root)
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if len(f.refundList()) != 0 {
		t.Fatal("completed call wrongly refunded as silent")
	}
}
func TestSupportGuardianApprovesCallAndOnlyPartiesReadLinks(t *testing.T) {
	h, f := setupStripe(t)
	coach, plan := supportPlan(t, h)
	minor, g := guardianMinor(t, h)
	root := applySupport(t, h, minor, coach, plan)
	gp := "/v1/guardian/" + g
	if r := doJSON(t, h, "PATCH", gp, "", map[string]any{"monthly_limit_yen": nil, "allow_publish": false}); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	r := doJSON(t, h, "POST", gp+"/contracts/"+root+"/consent", "", map[string]any{"agree_report": true, "agree_detection": true})
	if r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	paySupport(t, h, root)
	id := proposeSupport(t, h, coach, root, time.Now().Add(time.Hour))
	if r := doJSON(t, h, "POST", "/v1/coaching/contracts/"+root+"/calls/"+id+"/confirm", minor, nil); r.Code != 422 {
		t.Fatal("minor confirmed guardian's call")
	}
	if r := doJSON(t, h, "POST", gp+"/contracts/"+root+"/calls/"+id+"/confirm", "", nil); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if r := doJSON(t, h, "GET", gp+"/contracts/"+root+"/support", "", nil); r.Code != 200 || !strings.Contains(r.Body.String(), "meet.google.com") {
		t.Fatal(r.Body.String())
	}
	if strings.Contains(f.lastSession().Encode(), g) {
		t.Fatal("guardian token sent to payment provider")
	}
}
