package httpx_test

import (
	"github.com/google/uuid"
	"net/http"
	"strings"
	"testing"
)

// 売るのは6枚つづり5,000円だけ（本体 #569、価格は #983）。旧商品（1枚・5枚）は出さず、買えない。
// 単発の料金（1,000円）と1回あたりの枚数（1枚）・有効期限（6か月）も一緒に返す。
func TestTicketProductsAreSixForFiveThousand(t *testing.T) {
	h, _ := setupStripe(t)
	_, owner := demoRequester(t, h, adminToken(t, h))
	summary := doJSON(t, h, http.MethodGet, "/v1/me/coaching/tickets", owner, nil).Body.String()
	for _, want := range []string{`"products":[{"id":"six","title":"チケット6枚","quantity":6,"amount_yen":5000`, `"single_price_yen":1000`, `"review_units":1`, `"validity_months":6`} {
		if !strings.Contains(summary, want) {
			t.Fatalf("summary %s missing %s", summary, want)
		}
	}
	for _, old := range []string{"single", "five"} {
		rec := doJSON(t, h, http.MethodPost, "/v1/coaching/tickets/checkout", owner, map[string]any{"guardian_permission": true, "product_id": old, "request_key": uuid.NewString(), "return_path": "/coaching/new"})
		if rec.Code != 422 || !strings.Contains(rec.Body.String(), "product_unavailable") {
			t.Fatalf("old product %s %d %s", old, rec.Code, rec.Body.String())
		}
	}
}

// 価格を変えても、変える前に作った決済は保存した購入金額のまま完了できる。
func TestTicketPriceChangePreservesPurchaseSnapshot(t *testing.T) {
	h, _ := setupStripe(t)
	_, owner := demoRequester(t, h, adminToken(t, h))
	execSQLArgs(t, `UPDATE coaching_ticket_products SET amount_yen=4800 WHERE id='six'`)
	defer execSQLArgs(t, `UPDATE coaching_ticket_products SET amount_yen=5000 WHERE id='six'`)
	old := startTicketPurchase(t, h, owner, "six", uuid.NewString(), "/coaching/new")
	execSQLArgs(t, `UPDATE coaching_ticket_products SET amount_yen=5000 WHERE id='six'`)
	current := startTicketPurchase(t, h, owner, "six", uuid.NewString(), "/coaching/new")
	payTicketPurchase(t, h, old, 4800)
	payTicketPurchase(t, h, current, 5000)
	state := ticketSummary(t, h, owner)
	if state.Balance.Available != 12 {
		t.Fatalf("balance: %+v", state.Balance)
	}
	amounts := map[int]bool{}
	for _, purchase := range state.Purchases {
		amounts[purchase.AmountYen] = true
	}
	if !amounts[4800] || !amounts[5000] {
		t.Fatalf("purchase snapshots: %+v", state.Purchases)
	}
}
