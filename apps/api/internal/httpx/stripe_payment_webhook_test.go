package httpx_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/auth"
	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/hinata/track-talk/apps/api/internal/httpx"
	"github.com/jackc/pgx/v5/pgxpool"
)

// 公開中に作った既存注文へ、停止後に通知が届く場面を同じテストDBで再現する。
func setupStripeBeforeAndAfterClose(t *testing.T) (http.Handler, http.Handler, *fakeStripe) {
	t.Helper()
	var cfg config.Config
	before, f := setupStripeConfig(t, func(c *config.Config) { cfg = *c })
	pool, err := pgxpool.New(context.Background(), testDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	verifier, err := auth.NewVerifier(cfg.JWTSecret, "", cfg.JWTAUD)
	if err != nil {
		t.Fatal(err)
	}
	cfg.CoachingEnabled = false
	cfg.CoachingNewAcceptance = false
	cfg.CoachingMessaging = false
	cfg.CoachingTicketSales = false
	cfg.ReviewRandomAssign = false
	return before, httpx.New(cfg, pool, verifier), f
}

func TestStripePaymentWebhooksCompleteExistingOrdersWhileClosed(t *testing.T) {
	for _, kind := range []string{"personal", "review-card", "review-paypay", "consultation", "tickets"} {
		t.Run(kind, func(t *testing.T) {
			fakeCoachingCloudinary(t)
			before, after, f := setupStripeBeforeAndAfterClose(t)
			var session, id, query, expected string
			object := map[string]any{"payment_status": "paid", "payment_intent": "pi_closed_" + kind, "currency": "jpy"}
			switch kind {
			case "personal":
				user, _, contract := adultAccepted(t, before)
				checkout(t, before, user, contract)
				id = contract
				session = sessionIDFor(t, id)
				object["amount_total"] = 8000
				object["client_reference_id"] = f.lastSession().Get("metadata[payment_id]")
				query, expected = `SELECT status FROM coaching_contracts WHERE id=$1`, "active"
			case "review-card", "review-paypay":
				admin := adminToken(t, before)
				_, user := demoRequester(t, before, admin)
				method := "paypay"
				expected = "captured"
				if kind == "review-card" {
					method, expected = "card", "authorized"
					object["payment_status"] = "unpaid"
					object["payment_intent"] = "pi_card_closed"
				}
				body := reviewBodyFor(t, before, user)
				body["method"] = method
				rec := doJSON(t, before, http.MethodPost, "/v1/coaching/requests", user, body)
				if rec.Code != http.StatusCreated {
					t.Fatalf("request %d %s", rec.Code, rec.Body.String())
				}
				var created struct{ ID string }
				if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
					t.Fatal(err)
				}
				id = created.ID
				queryRowArgs(t, `SELECT checkout_session_id FROM review_payments WHERE request_id=$1`, []any{id}, &session)
				object["metadata"] = map[string]string{"kind": "review"}
				query = `SELECT status FROM review_payments WHERE request_id=$1`
			case "consultation":
				user, _, root := endedCashRoom(t, before)
				c := consultationDraft(t, before, user, "/v1/coaching/contracts/"+root+"/consultations", uuid.NewString(), "停止前に作成した追加相談")
				consultationAction(t, before, user, c.ID, "submit", nil)
				id = c.ID
				queryRowArgs(t, `SELECT checkout_session_id FROM coaching_consultations WHERE id=$1`, []any{id}, &session)
				object["client_reference_id"], object["amount_total"] = id, c.Price
				object["metadata"] = map[string]string{"kind": "consultation", "consultation_id": id}
				query, expected = `SELECT payment_state FROM coaching_consultations WHERE id=$1`, "paid"
			case "tickets":
				_, user := demoUser(t, before, time.Date(1995, 4, 1, 0, 0, 0, 0, time.UTC))
				id = startTicketPurchase(t, before, user, "five", uuid.NewString(), "/coaching/new")
				queryRowArgs(t, `SELECT checkout_session_id FROM coaching_ticket_purchases WHERE id=$1`, []any{id}, &session)
				object["client_reference_id"], object["amount_total"] = id, 2880
				object["metadata"] = map[string]string{"kind": "tickets", "purchase_id": id}
				query, expected = `SELECT status FROM coaching_ticket_purchases WHERE id=$1`, "paid"
			}
			object["id"] = session
			// 2種類の完了通知・再送を受けても既存の成立処理が一度だけ適用される。
			for _, typ := range []string{"checkout.session.completed", "checkout.session.async_payment_succeeded", "checkout.session.completed"} {
				if rec := stripeWebhook(t, after, testWebhookSecret, typ, object); rec.Code != http.StatusOK {
					t.Fatalf("closed webhook %s: %d %s", typ, rec.Code, rec.Body.String())
				}
				var state string
				queryRowArgs(t, query, []any{id}, &state)
				if state != expected {
					t.Fatalf("state %s want %s", state, expected)
				}
			}
			if kind == "tickets" {
				var credited int
				queryRowArgs(t, `SELECT sum(available_delta)::int FROM coaching_ticket_ledger WHERE lot_id=$1 AND kind='purchase'`, []any{id}, &credited)
				if credited != 5 {
					t.Fatalf("duplicate ticket credit %d", credited)
				}
			}
			if len(f.refundList()) != 0 || len(f.transferList()) != 0 {
				t.Fatal("duplicate notification caused a refund or transfer")
			}
		})
	}
}

func TestStripePaymentWebhookDoesNotOpenPurchases(t *testing.T) {
	h, f := setupStripeConfig(t, func(c *config.Config) {
		c.CoachingEnabled = false
		c.CoachingNewAcceptance = false
		c.CoachingMessaging = false
		c.CoachingTicketSales = false
	})
	tok := token(t, uuid.New())
	id := uuid.NewString()
	paths := []string{
		"/v1/coaching/contracts",
		"/v1/coaching/contracts/" + id + "/checkout",
		"/v1/guardian/unused/contracts/" + id + "/consent",
		"/v1/guardian/unused/contracts/" + id + "/checkout",
		"/v1/coaching/requests",
		"/v1/coaching/requests/" + id + "/checkout",
		"/v1/coaching/consultations/" + id + "/submit",
		"/v1/coaching/consultations/" + id + "/checkout",
		"/v1/guardian/unused/consultations/" + id + "/checkout",
		"/v1/coaching/tickets/checkout",
	}
	for _, path := range paths {
		for _, token := range []string{"", tok} {
			if rec := doJSON(t, h, http.MethodPost, path, token, nil); rec.Code != http.StatusNotFound {
				t.Fatalf("purchase opened %s: %d", path, rec.Code)
			}
		}
	}
	for _, key := range []string{testConnectWebhookSecret, "whsec_wrong"} {
		if rec := stripeWebhook(t, h, key, "checkout.session.completed", map[string]any{}); rec.Code != http.StatusBadRequest {
			t.Fatalf("non-payment signing key accepted: %d", rec.Code)
		}
	}
	for _, typ := range []string{"account.updated", "customer.created"} {
		if rec := stripeWebhook(t, h, testWebhookSecret, typ, map[string]any{}); rec.Code != http.StatusNotFound {
			t.Fatalf("non-payment event accepted while closed %s: %d", typ, rec.Code)
		}
	}
	if len(f.sessions) != 0 {
		t.Fatal("checkout session created while closed")
	}
}

func TestStripePaymentWebhookRequiresOwnKeysWhileClosed(t *testing.T) {
	for _, missing := range []string{"stripe", "payment-signature"} {
		t.Run(missing, func(t *testing.T) {
			h, _ := setupStripeConfig(t, func(c *config.Config) {
				c.CoachingEnabled = false
				if missing == "stripe" {
					c.StripeSecretKey = ""
				} else {
					c.StripeWebhookSecret = ""
				}
			})
			if rec := stripeWebhook(t, h, testConnectWebhookSecret, "checkout.session.completed", map[string]any{}); rec.Code != http.StatusNotFound {
				t.Fatalf("missing %s: %d", missing, rec.Code)
			}
		})
	}
}

// コーチングを止めている間も、見覚えのない返金・送金の検知だけは受け付ける。
func TestStripeMoneyAlertsAcceptedWhileClosed(t *testing.T) {
	h, _ := setupStripeConfig(t, func(c *config.Config) {
		c.CoachingEnabled = false
		c.CoachingNewAcceptance = false
		c.CoachingMessaging = false
		c.CoachingTicketSales = false
	})
	cases := []struct {
		typ string
		obj map[string]any
	}{
		{"refund.created", map[string]any{"id": "re_unknown_1", "amount": 500, "payment_intent": "pi_unknown_1"}},
		{"charge.refunded", map[string]any{"id": "ch_unknown_1", "amount_refunded": 500, "payment_intent": "pi_unknown_2"}},
		{"transfer.created", map[string]any{"id": "tr_unknown_1", "amount": 3000, "destination": "acct_unknown"}},
		{"transfer.reversed", map[string]any{"id": "tr_unknown_2", "amount": 3000}},
		{"payout.created", map[string]any{"id": "po_1", "amount": 3000}},
	}
	for _, tc := range cases {
		if rec := stripeWebhook(t, h, testWebhookSecret, tc.typ, tc.obj); rec.Code != http.StatusOK {
			t.Fatalf("%s while closed: %d %s", tc.typ, rec.Code, rec.Body.String())
		}
	}
	// 検知対象でないイベントは、これまでどおり閉じたまま。
	if rec := stripeWebhook(t, h, testWebhookSecret, "customer.created", map[string]any{}); rec.Code != http.StatusNotFound {
		t.Fatalf("customer.created while closed: %d", rec.Code)
	}
}
