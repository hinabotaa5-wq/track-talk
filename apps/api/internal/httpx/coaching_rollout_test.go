package httpx_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/auth"
	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/hinata/track-talk/apps/api/internal/httpx"
	"github.com/jackc/pgx/v5/pgxpool"
)

// 同じDBと決済設定のまま、新規受付のみを停止したサーバーに切り替える。
func closedAcceptanceHandler(t *testing.T, cfg config.Config) http.Handler {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	v, err := auth.NewVerifier(cfg.JWTSecret, "", "authenticated")
	if err != nil {
		t.Fatal(err)
	}
	cfg.CoachingNewAcceptance = false
	return httpx.New(cfg, pool, v)
}

func TestAcceptanceClosedPreservesExistingFulfillmentAndLatePayments(t *testing.T) {
	fakeCoachingCloudinary(t)
	var cfg config.Config
	open, _ := setupStripeConfig(t, func(c *config.Config) { cfg = *c })
	admin := adminToken(t, open)
	_, coach := onDutyCoach(t, open, admin)
	_, owner := demoRequester(t, open, admin)
	buyFiveTickets(t, open, owner)
	request := ticketReview(t, open, owner)
	offer := coachRequests(t, open, coach).Offers[0].ID
	late := startTicketPurchase(t, open, owner, "single", uuid.NewString(), "/coaching/new")
	closed := closedAcceptanceHandler(t, cfg)

	id := uuid.NewString()
	for _, path := range []string{
		"/v1/coaching/contracts", "/v1/coaching/requests", "/v1/coaching/tickets/checkout",
		"/v1/coaching/requests/" + id + "/checkout", "/v1/coaching/consultations/" + id + "/submit",
		"/v1/coaching/consultations/" + id + "/checkout", "/v1/guardian/secret/consultations/" + id + "/checkout",
	} {
		rec := doJSON(t, closed, http.MethodPost, path, owner, nil)
		if rec.Code != 409 || !strings.Contains(rec.Body.String(), "new_acceptance_closed") {
			t.Fatalf("closed %s: %d %s", path, rec.Code, rec.Body.String())
		}
	}
	// 停止前に開いた決済の遅延通知と再送を受け付ける。
	payTicketPurchase(t, closed, late, 600)
	payTicketPurchase(t, closed, late, 600)
	if got := ticketSummary(t, closed, owner).Balance.Available; got != 4 {
		t.Fatalf("balance %d", got)
	}
	if rec := doJSON(t, closed, "POST", "/v1/coach/offers/"+offer+"/accept", coach, nil); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	if rec := doJSON(t, closed, "POST", "/v1/coaching/requests/"+request+"/response", coach, responseBody()); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	if rec := doJSON(t, closed, "GET", "/v1/coaching/requests/"+request, owner, nil); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	if rec := doJSON(t, closed, "POST", "/v1/admin/coaching/tickets/"+late+"/refund", admin, nil); rec.Code != http.StatusAccepted {
		t.Fatal(rec.Body.String())
	}
	if rec := tick(t, closed, "tick-secret"); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
}

func reconciliation(t *testing.T, h http.Handler, admin string) map[string]int {
	t.Helper()
	rec := doJSON(t, h, "GET", "/v1/admin/coaching/reconciliation", admin, nil)
	if rec.Code != 200 {
		t.Fatalf("reconciliation %d %s", rec.Code, rec.Body.String())
	}
	var result map[string]int
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestReconciliationDetectsAccountingIssuesAndProtectsAdminAccess(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	admin := adminToken(t, h)
	_, coach := onDutyCoach(t, h, admin)
	_, owner := demoRequester(t, h, admin)
	path := "/v1/admin/coaching/reconciliation"
	for _, tc := range []struct {
		token string
		code  int
	}{{"", 401}, {owner, 403}} {
		if rec := doJSON(t, h, "GET", path, tc.token, nil); rec.Code != tc.code {
			t.Fatalf("access %d %s", rec.Code, rec.Body.String())
		}
	}
	lot := buyFiveTickets(t, h, owner)
	request := ticketReview(t, h, owner)
	offer := coachRequests(t, h, coach).Offers[0].ID
	if rec := doJSON(t, h, "POST", "/v1/coach/offers/"+offer+"/accept", coach, nil); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	if rec := doJSON(t, h, "POST", "/v1/coaching/requests/"+request+"/response", coach, responseBody()); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	result := reconciliation(t, h, admin)
	if result["unpaid_coach_earnings"] != 1 {
		t.Fatalf("earnings %+v", result)
	}
	for _, key := range []string{"ticket_lot_ledger_mismatches", "paid_purchases_without_lot", "consumed_ticket_earnings_mismatches", "pending_ticket_refunds", "pending_consultation_refunds", "pending_support_refunds"} {
		if value, ok := result[key]; !ok || value != 0 {
			t.Fatalf("healthy %s: %+v", key, result)
		}
	}
	// 制約内で金額を壊しても検出し、通常の送金待ちと区別する。
	execSQLArgs(t, `UPDATE coach_earnings SET amount_yen=amount_yen-1 WHERE review_request_id=$1`, request)
	execSQLArgs(t, `UPDATE coaching_ticket_ledger SET available_delta=available_delta-1 WHERE lot_id=$1 AND kind='purchase'`, lot)
	execSQLArgs(t, `INSERT INTO coaching_ticket_purchases(id,owner_id,request_key,product_id,quantity,amount_yen,status,session_expires_at,return_path,payment_intent_id,paid_at) SELECT $1,owner_id,$3,'single',1,500,'paid',now(),'/coaching/new',$4,now() FROM coaching_ticket_lots WHERE id=$2`, uuid.New(), lot, uuid.New(), "pi_missing_lot")
	result = reconciliation(t, h, admin)
	if result["ticket_lot_ledger_mismatches"] != 1 || result["paid_purchases_without_lot"] != 1 || result["consumed_ticket_earnings_mismatches"] != 1 {
		t.Fatalf("issues %+v", result)
	}
}

func TestAcceptanceClosedKeepsExistingContractCheckoutAndMessages(t *testing.T) {
	var cfg config.Config
	open, _ := setupStripeConfig(t, func(c *config.Config) { cfg = *c })
	_, coach, plan := planCoach(t, open, 5)
	_, owner := demoUser(t, open, time.Date(1995, 1, 1, 0, 0, 0, 0, time.UTC))
	rec := doJSON(t, open, "POST", "/v1/coaching/contracts", owner, contractBody(plan))
	if rec.Code != 201 {
		t.Fatal(rec.Body.String())
	}
	root := decodeContract(t, rec.Body.Bytes()).ID
	closed := closedAcceptanceHandler(t, cfg)
	rec = doJSON(t, closed, "POST", "/v1/coach/contracts/"+root+"/accept", coach, map[string]any{"agree_report": true, "agree_detection": true})
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	rec = doJSON(t, closed, "POST", "/v1/coaching/contracts/"+root+"/checkout", owner, map[string]any{"method": "card"})
	if rec.Code != 200 {
		t.Fatalf("existing checkout %d %s", rec.Code, rec.Body.String())
	}
	paidSession(t, closed, sessionIDFor(t, root), "pi_existing_contract", 8000)
	rec = doJSON(t, closed, "GET", "/v1/coaching/contracts/"+root+"/messages", owner, nil)
	if rec.Code != 200 {
		t.Fatalf("messages %d %s", rec.Code, rec.Body.String())
	}
}
