package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func nominationFee(t *testing.T, h http.Handler, tc string, fee any) {
	t.Helper()
	r := doJSON(t, h, "PATCH", "/v1/coach/me", tc, map[string]any{"nomination_fee_yen": fee})
	if r.Code != 200 {
		t.Fatalf("fee %d %s", r.Code, r.Body.String())
	}
}
func nominatedReview(t *testing.T, h http.Handler, tu, coach, method string, tickets bool) string {
	t.Helper()
	b := reviewBodyFor(t, h, tu)
	b["method"] = method
	b["use_tickets"] = tickets
	b["requested_coach_id"] = coach
	b["expected_nomination_fee_yen"] = 800
	r := doJSON(t, h, "POST", "/v1/coaching/requests", tu, b)
	if r.Code != 201 {
		t.Fatalf("nomination %d %s", r.Code, r.Body.String())
	}
	var v struct{ ID string }
	_ = json.Unmarshal(r.Body.Bytes(), &v)
	return v.ID
}
func payNomination(t *testing.T, h http.Handler, id, method string) {
	t.Helper()
	var session string
	queryRowArgs(t, `SELECT checkout_session_id FROM review_payments WHERE request_id=$1`, []any{id}, &session)
	status := "paid"
	if method == "card" {
		status = "unpaid"
	}
	r := stripeWebhook(t, h, testWebhookSecret, "checkout.session.completed", map[string]any{"id": session, "payment_status": status, "payment_intent": "pi_card_nomination_" + id, "metadata": map[string]any{"kind": "review"}})
	if r.Code != 200 {
		t.Fatal(r.Body.String())
	}
}

func TestNominationPaymentSnapshotAndReward(t *testing.T) {
	for _, tickets := range []bool{false, true} {
		name := "cash"
		if tickets {
			name = "tickets"
		}
		t.Run(name, func(t *testing.T) {
			fakeCoachingCloudinary(t)
			h, f := setupStripe(t)
			admin := adminToken(t, h)
			_, other := onDutyCoach(t, h, admin)
			coach, tc := onDutyCoach(t, h, admin)
			_, tu := demoRequester(t, h, admin)
			nominationFee(t, h, tc, 800)
			if tickets {
				buyFiveTickets(t, h, tu)
			}
			id := nominatedReview(t, h, tu, coach.String(), "card", tickets)
			expected := "2000"
			if tickets {
				expected = "800"
			}
			if got := f.lastSession().Get("line_items[0][price_data][unit_amount]"); got != expected {
				t.Fatalf("price %s want %s", got, expected)
			}
			nominationFee(t, h, tc, 1800)
			// Reopening checkout keeps the purchased price even after the coach changes their fee.
			r := doJSON(t, h, "POST", "/v1/coaching/requests/"+id+"/checkout", tu, map[string]any{"method": "card"})
			if r.Code != 200 {
				t.Fatal(r.Body.String())
			}
			payNomination(t, h, id, "card")
			offers := coachRequests(t, h, tc).Offers
			if len(offers) != 1 || offers[0].Request.ID != id || len(coachRequests(t, h, other).Offers) != 0 {
				t.Fatalf("wrong assignment %+v", offers)
			}
			if r := doJSON(t, h, "POST", "/v1/coach/offers/"+offers[0].ID+"/accept", tc, nil); r.Code != 200 {
				t.Fatal(r.Body.String())
			}
			if r := doJSON(t, h, "POST", "/v1/coaching/requests/"+id+"/response", tc, responseBody()); r.Code != 200 {
				t.Fatal(r.Body.String())
			}
			for range 2 {
				if r := tick(t, h, "tick-secret"); r.Code != 200 {
					t.Fatal(r.Body.String())
				}
			}
			audit := doJSON(t, h, "GET", "/v1/admin/coaching/reconciliation", admin, nil)
			if audit.Code != 200 || !strings.Contains(audit.Body.String(), `"consumed_ticket_earnings_mismatches":0`) {
				t.Fatalf("reconciliation %s", audit.Body.String())
			}
			e := coachEarnings(t, h, tc)
			if e.UnpaidYen != 1480 || len(e.Earnings) != 1 || e.Earnings[0].GrossYen != 2000 {
				t.Fatalf("reward %+v", e)
			}
			if tickets {
				b := ticketSummary(t, h, tu).Balance
				if b.Available != 3 || b.Reserved != 0 {
					t.Fatalf("balance %+v", b)
				}
			}
		})
	}
}

func TestNominationDeclineAndTimeoutNeverReassign(t *testing.T) {
	for _, action := range []string{"decline", "timeout"} {
		t.Run(action, func(t *testing.T) {
			fakeCoachingCloudinary(t)
			h, f := setupStripe(t)
			admin := adminToken(t, h)
			coach, tc := onDutyCoach(t, h, admin)
			_, other := onDutyCoach(t, h, admin)
			_, tu := demoRequester(t, h, admin)
			nominationFee(t, h, tc, 800)
			buyFiveTickets(t, h, tu)
			id := nominatedReview(t, h, tu, coach.String(), "paypay", true)
			payNomination(t, h, id, "paypay")
			offers := coachRequests(t, h, tc).Offers
			if len(offers) != 1 {
				t.Fatalf("offers %+v", offers)
			}
			if action == "decline" {
				if r := doJSON(t, h, "POST", "/v1/coach/offers/"+offers[0].ID+"/decline", tc, nil); r.Code != 204 {
					t.Fatal(r.Body.String())
				}
			} else {
				execSQLArgs(t, `UPDATE review_offers SET expires_at=now()-interval '1 minute' WHERE request_id=$1`, id)
			}
			execSQLArgs(t, `UPDATE review_payments SET authorized_at=now()-interval '2 minutes' WHERE request_id=$1`, id)
			if r := tick(t, h, "tick-secret"); r.Code != 200 {
				t.Fatal(r.Body.String())
			}
			if status := reviewStatus(t, h, tu, id); status != "expired" {
				t.Fatalf("status %s", status)
			}
			if len(coachRequests(t, h, other).Offers) != 0 {
				t.Fatal("nomination reassigned")
			}
			var notified int
			queryRowArgs(t, `SELECT count(*) FROM notifications WHERE kind='review_expired' AND review_request_id=$1`, []any{id}, &notified)
			if notified != 1 {
				t.Fatalf("requester notified %d times", notified)
			}
			b := ticketSummary(t, h, tu).Balance
			if b.Available != 5 || b.Reserved != 0 {
				t.Fatalf("balance %+v", b)
			}
			refunds := f.refundList()
			if len(refunds) != 1 || refunds[0].Get("amount") != "800" {
				t.Fatalf("refunds %v", refunds)
			}
			if e := coachEarnings(t, h, tc); len(e.Earnings) != 0 {
				t.Fatalf("unearned reward %+v", e)
			}
		})
	}
}

func TestNominationValidationAndAvailability(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)
	_, tu := demoRequester(t, h, admin)
	for _, bad := range []any{-100, 1, 100.5, 10000000, "800"} {
		r := doJSON(t, h, "PATCH", "/v1/coach/me", tc, map[string]any{"nomination_fee_yen": bad})
		if r.Code != 422 {
			t.Fatalf("invalid fee %v: %d", bad, r.Code)
		}
	}
	nominationFee(t, h, tc, 800)
	r := doJSON(t, h, "GET", "/v1/coaching/nominees?event=100m", tu, nil)
	if r.Code != 200 || !strings.Contains(r.Body.String(), coach.String()) || !strings.Contains(r.Body.String(), `"total_price_yen":2000`) {
		t.Fatal(r.Body.String())
	}
	b := reviewBodyFor(t, h, tu)
	b["method"] = "card"
	b["requested_coach_id"] = coach
	b["expected_nomination_fee_yen"] = 500
	if r := doJSON(t, h, "POST", "/v1/coaching/requests", tu, b); r.Code != 409 {
		t.Fatalf("stale quote %d %s", r.Code, r.Body.String())
	}
	nominationFee(t, h, tc, nil)
	r = doJSON(t, h, "GET", "/v1/coaching/nominees?event=100m", tu, nil)
	if strings.Contains(r.Body.String(), coach.String()) {
		t.Fatal("opted-out coach visible")
	}
	b["expected_nomination_fee_yen"] = 800
	if r := doJSON(t, h, "POST", "/v1/coaching/requests", tu, b); r.Code != 422 {
		t.Fatalf("disabled %d %s", r.Code, r.Body.String())
	}
	nominationFee(t, h, tc, 800)
	execSQLArgs(t, `UPDATE coach_profiles SET daily_cap=1 WHERE profile_id=$1`, coach)
	id := nominatedReview(t, h, tu, coach.String(), "card", false)
	payNomination(t, h, id, "card")
	r = doJSON(t, h, "GET", "/v1/coaching/nominees?event=100m", tu, nil)
	if strings.Contains(r.Body.String(), coach.String()) {
		t.Fatal("full coach visible")
	}
}

func TestNominationUnpaidTicketsReleased(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)
	_, tu := demoRequester(t, h, admin)
	nominationFee(t, h, tc, 800)
	buyFiveTickets(t, h, tu)
	id := nominatedReview(t, h, tu, coach.String(), "card", true)
	execSQLArgs(t, `UPDATE review_requests SET created_at=now()-interval '3 hours' WHERE id=$1`, id)
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	b := ticketSummary(t, h, tu).Balance
	if b.Available != 5 || b.Reserved != 0 {
		t.Fatalf("abandoned checkout %+v", b)
	}
}

func TestVideoSupportPlanRetired(t *testing.T) {
	h := setup(t)
	coach, tc, plan := planCoach(t, h, 3)
	b := planBody()
	b["service_type"] = "video_review"
	if r := doJSON(t, h, "POST", "/v1/coach/me/plans", tc, b); r.Code != 422 {
		t.Fatalf("new video support %d %s", r.Code, r.Body.String())
	}
	execSQLArgs(t, `UPDATE coach_profiles SET service_types=ARRAY['video_review'] WHERE profile_id=$1`, coach)
	execSQLArgs(t, `UPDATE coaching_plans SET service_type='video_review' WHERE id=$1`, plan)
	if r := doJSON(t, h, "GET", "/v1/coaching/plans/"+plan, "", nil); r.Code != 404 {
		t.Fatalf("old video for sale %d", r.Code)
	}
	r := doJSON(t, h, "GET", "/v1/coaches", "", nil)
	if strings.Contains(r.Body.String(), plan) {
		t.Fatal("retired plan listed")
	}
	b["published"] = false
	if r := doJSON(t, h, "PATCH", "/v1/coach/me/plans/"+plan, tc, b); r.Code != 200 {
		t.Fatalf("archive %d %s", r.Code, r.Body.String())
	}
}
