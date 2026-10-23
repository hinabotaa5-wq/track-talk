package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func inPersonPlan(t *testing.T, h http.Handler) (coach, plan string) {
	t.Helper()
	_, coach, plan = planCoach(t, h, 5)
	r := doJSON(t, h, "PATCH", "/v1/coach/me/services", coach, map[string]any{"service_types": []string{"in_person"}})
	if r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	body := planBody()
	body["service_type"] = "in_person"
	body["venue_area"] = "東京都"
	body["venue_name"] = "国立競技場"
	body["session_count"] = 2
	body["session_minutes"] = 60
	body["travel_fee_yen"] = 1000
	body["max_participants"] = 3
	body["price_yen"] = 8000
	r = doJSON(t, h, "PATCH", "/v1/coach/me/plans/"+plan, coach, body)
	if r.Code != 200 {
		t.Fatalf("in-person plan %d %s", r.Code, r.Body.String())
	}
	return
}
func applyInPerson(t *testing.T, h http.Handler, owner, coach, plan string) string {
	t.Helper()
	r := doJSON(t, h, "POST", "/v1/coaching/contracts", owner, contractBody(plan))
	if r.Code != 201 {
		t.Fatalf("in-person apply %d %s", r.Code, r.Body.String())
	}
	id := decodeContract(t, r.Body.Bytes()).ID
	r = doJSON(t, h, "POST", "/v1/coach/contracts/"+id+"/accept", coach, map[string]any{"agree_report": true, "agree_detection": true})
	if r.Code != 200 {
		t.Fatalf("in-person accept %d %s", r.Code, r.Body.String())
	}
	return id
}
func payInPerson(t *testing.T, h http.Handler, root string) {
	t.Helper()
	var sid, pid string
	queryRowArgs(t, `SELECT checkout_session_id,id::text FROM coaching_payments WHERE contract_id=$1 ORDER BY created_at DESC LIMIT 1`, []any{root}, &sid, &pid)
	r := stripeWebhook(t, h, testWebhookSecret, "checkout.session.completed", map[string]any{"id": sid, "payment_status": "paid", "payment_intent": "pi_inperson_" + root, "amount_total": 9000, "currency": "jpy", "client_reference_id": pid})
	if r.Code != 200 {
		t.Fatalf("in-person pay %d %s", r.Code, r.Body.String())
	}
}
func proposeInPerson(t *testing.T, h http.Handler, coach, root string, at time.Time) string {
	t.Helper()
	r := doJSON(t, h, "POST", "/v1/coaching/contracts/"+root+"/in-person", coach, map[string]any{"request_key": uuid.NewString(), "starts_at": at.Format(time.RFC3339Nano), "venue_name": "国立競技場", "venue_area": "東京都", "meeting_note": "正門前", "participants": 2})
	if r.Code != 200 {
		t.Fatalf("propose in-person %d %s", r.Code, r.Body.String())
	}
	var out struct {
		Session struct{ ID string }
	}
	if err := json.Unmarshal(r.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out.Session.ID
}
func TestInPersonPlanSnapshotIncludesTravelFee(t *testing.T) {
	h := setup(t)
	coach, plan := inPersonPlan(t, h)
	_, owner := demoUser(t, h, time.Date(1995, 1, 1, 0, 0, 0, 0, time.UTC))
	root := applyInPerson(t, h, owner, coach, plan)
	r := doJSON(t, h, "GET", "/v1/coaching/contracts/"+root, owner, nil)
	if r.Code != 200 || !strings.Contains(r.Body.String(), `"fulfillment_version":2`) || !strings.Contains(r.Body.String(), "国立競技場") || !strings.Contains(r.Body.String(), `"price_yen":9000`) {
		t.Fatal(r.Body.String())
	}
	execSQLArgs(t, `UPDATE coaching_plans SET venue_name='別の会場',session_count=5 WHERE id=$1`, plan)
	r = doJSON(t, h, "GET", "/v1/coaching/contracts/"+root, owner, nil)
	if !strings.Contains(r.Body.String(), "国立競技場") || !strings.Contains(r.Body.String(), `"session_count":2`) {
		t.Fatal("contract terms changed: " + r.Body.String())
	}
}
func TestInPersonRequiresBothConfirmationsAndReturnsQuota(t *testing.T) {
	h := setup(t)
	coach, plan := inPersonPlan(t, h)
	_, owner := demoUser(t, h, time.Date(1995, 1, 1, 0, 0, 0, 0, time.UTC))
	_, other := demoUser(t, h, time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC))
	root := applyInPerson(t, h, owner, coach, plan)
	if r := doJSON(t, h, "GET", "/v1/coaching/contracts/"+root+"/in-person", other, nil); r.Code != 404 {
		t.Fatal("private venue leaked")
	}
	id := proposeInPerson(t, h, coach, root, time.Now().Add(time.Hour))
	if r := doJSON(t, h, "POST", "/v1/coaching/contracts/"+root+"/in-person/"+id+"/confirm", coach, nil); r.Code != 422 {
		t.Fatal("coach confirmed own session")
	}
	r := doJSON(t, h, "POST", "/v1/coaching/contracts/"+root+"/in-person/"+id+"/confirm", owner, nil)
	if r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if r := doJSON(t, h, "POST", "/v1/coaching/contracts/"+root+"/in-person/"+id+"/report-completed", coach, nil); r.Code != 422 {
		t.Fatal("future completion")
	}
	execSQLArgs(t, `UPDATE coaching_in_person_sessions SET starts_at=now()-interval '2 hours',ends_at=now()-interval '1 hour' WHERE id=$1`, id)
	for _, a := range []struct {
		tok, act string
	}{{coach, "report-completed"}, {owner, "confirm-completed"}} {
		r := doJSON(t, h, "POST", "/v1/coaching/contracts/"+root+"/in-person/"+id+"/"+a.act, a.tok, nil)
		if r.Code != 200 {
			t.Fatalf("%s %d %s", a.act, r.Code, r.Body.String())
		}
	}
	r = doJSON(t, h, "GET", "/v1/coaching/contracts/"+root+"/in-person", owner, nil)
	if !strings.Contains(r.Body.String(), `"completed":1`) {
		t.Fatal(r.Body.String())
	}
	if r := doJSON(t, h, "POST", "/v1/coaching/contracts/"+root+"/in-person/"+id+"/cancel", owner, nil); r.Code != 422 {
		t.Fatal("completed session refunded quota")
	}
}
func TestInPersonUnprovidedRefundAfterEnd(t *testing.T) {
	h, f := setupStripe(t)
	coach, plan := inPersonPlan(t, h)
	_, owner := demoUser(t, h, time.Date(1995, 1, 1, 0, 0, 0, 0, time.UTC))
	root := applyInPerson(t, h, owner, coach, plan)
	checkout(t, h, owner, root)
	payInPerson(t, h, root)
	id := proposeInPerson(t, h, coach, root, time.Now().Add(time.Hour))
	if r := doJSON(t, h, "POST", "/v1/coaching/contracts/"+root+"/in-person/"+id+"/confirm", owner, nil); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	execSQLArgs(t, `UPDATE coaching_in_person_sessions SET starts_at=now()-interval '2 hours',ends_at=now()-interval '1 hour' WHERE id=$1`, id)
	if r := doJSON(t, h, "POST", "/v1/coaching/contracts/"+root+"/in-person/"+id+"/report-completed", coach, nil); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if r := doJSON(t, h, "POST", "/v1/coaching/contracts/"+root+"/in-person/"+id+"/confirm-completed", owner, nil); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	// 2回のうち1回だけ実施。終了48時間後に残り4500円を返す。
	execSQLArgs(t, `UPDATE coaching_contracts SET ends_at=now()-interval '49 hours' WHERE id=$1`, root)
	for i := 0; i < 2; i++ {
		if r := tick(t, h, "tick-secret"); r.Code != 200 {
			t.Fatal(r.Body.String())
		}
	}
	if len(f.refundList()) != 1 || f.refundList()[0].Get("amount") != "4500" {
		t.Fatalf("refund %+v", f.refundList())
	}
}
func TestInPersonGuardianConfirmsForMinor(t *testing.T) {
	h, _ := setupStripe(t)
	coach, plan := inPersonPlan(t, h)
	minor, g := guardianMinor(t, h)
	root := applyInPerson(t, h, minor, coach, plan)
	gp := "/v1/guardian/" + g
	if r := doJSON(t, h, "PATCH", gp, "", map[string]any{"monthly_limit_yen": nil, "allow_publish": false}); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if r := doJSON(t, h, "POST", gp+"/contracts/"+root+"/consent", "", map[string]any{"agree_report": true, "agree_detection": true}); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	payInPerson(t, h, root)
	id := proposeInPerson(t, h, coach, root, time.Now().Add(time.Hour))
	if r := doJSON(t, h, "POST", "/v1/coaching/contracts/"+root+"/in-person/"+id+"/confirm", minor, nil); r.Code != 422 {
		t.Fatal("minor confirmed guardian session")
	}
	if r := doJSON(t, h, "POST", gp+"/contracts/"+root+"/in-person/"+id+"/confirm", "", nil); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if r := doJSON(t, h, "GET", gp+"/contracts/"+root+"/in-person", "", nil); r.Code != 200 || !strings.Contains(r.Body.String(), "国立競技場") {
		t.Fatal(r.Body.String())
	}
}
