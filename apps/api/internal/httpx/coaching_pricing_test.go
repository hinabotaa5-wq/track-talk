package httpx_test

import (
	"github.com/google/uuid"
	"net/http"
	"strings"
	"testing"
)

func TestTicketPriceChangePreservesPurchaseSnapshot(t *testing.T) {
	h, _ := setupStripe(t)
	_, owner := demoRequester(t, h, adminToken(t, h))
	summary := doJSON(t, h, http.MethodGet, "/v1/me/coaching/tickets", owner, nil)
	for _, amount := range []string{`"amount_yen":600`, `"amount_yen":2880`, `"unit_yen":600`} {
		if !strings.Contains(summary.Body.String(), amount) {
			t.Fatalf("new prices: %s", summary.Body.String())
		}
	}
	// 値上げ前に作られた決済は、保存された購入金額のまま完了できる。
	execSQLArgs(t, `UPDATE coaching_ticket_products SET amount_yen=500 WHERE id='single'`)
	defer execSQLArgs(t, `UPDATE coaching_ticket_products SET amount_yen=600 WHERE id='single'`)
	old := startTicketPurchase(t, h, owner, "single", uuid.NewString(), "/coaching/new")
	execSQLArgs(t, `UPDATE coaching_ticket_products SET amount_yen=600 WHERE id='single'`)
	current := startTicketPurchase(t, h, owner, "single", uuid.NewString(), "/coaching/new")
	payTicketPurchase(t, h, old, 500)
	payTicketPurchase(t, h, current, 600)
	state := ticketSummary(t, h, owner)
	if state.Balance.Available != 2 {
		t.Fatalf("balance: %+v", state.Balance)
	}
	amounts := map[int]bool{}
	for _, purchase := range state.Purchases {
		amounts[purchase.AmountYen] = true
	}
	if !amounts[500] || !amounts[600] {
		t.Fatalf("purchase snapshots: %+v", state.Purchases)
	}
}
