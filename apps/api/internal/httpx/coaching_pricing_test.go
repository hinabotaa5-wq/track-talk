package httpx_test

import (
	"github.com/google/uuid"
	"net/http"
	"strings"
	"testing"
)

// チケットの販売は全商品とも終了した（本体 #1031、S1・S2）。サマリは商品を出さず sales_open=false、
// 購入の API は商品（現行の6枚・旧商品の1枚・5枚）によらず 409 ticket_sales_closed。単発の料金（1,000円）と
// 1回あたりの枚数（1枚）・有効期限（6か月）は、発行済みのチケットの説明のため引き続き返す。
func TestTicketSalesEndedForAllProducts(t *testing.T) {
	h, _ := setupStripe(t)
	_, owner := demoRequester(t, h, adminToken(t, h))
	summary := doJSON(t, h, http.MethodGet, "/v1/me/coaching/tickets", owner, nil).Body.String()
	for _, want := range []string{`"products":[]`, `"sales_open":false`, `"can_purchase":false`, `"single_price_yen":1000`, `"review_units":1`, `"validity_months":6`, `"terms_version":"v7"`} {
		if !strings.Contains(summary, want) {
			t.Fatalf("summary %s missing %s", summary, want)
		}
	}
	for _, product := range []string{"six", "single", "five"} {
		rec := doJSON(t, h, http.MethodPost, "/v1/coaching/tickets/checkout", owner, map[string]any{"guardian_permission": true, "product_id": product, "request_key": uuid.NewString(), "return_path": "/coaching/new"})
		if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "ticket_sales_closed") {
			t.Fatalf("product %s %d %s", product, rec.Code, rec.Body.String())
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
