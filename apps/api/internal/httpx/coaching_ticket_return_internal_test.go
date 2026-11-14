package httpx

import (
	"encoding/json"
	"os"
	"testing"
)

// 本体 #691：チケット購入の戻り先の許可・不許可を、Web（lib/coaching-tickets.ts）と同じ表で固定する。
func TestTicketReturnPathMatchesSharedTable(t *testing.T) {
	raw, err := os.ReadFile("testdata/ticket_return_paths.json")
	if err != nil {
		t.Fatal(err)
	}
	var table struct{ Allowed, Rejected []string }
	if err := json.Unmarshal(raw, &table); err != nil {
		t.Fatal(err)
	}
	if len(table.Allowed) == 0 || len(table.Rejected) == 0 {
		t.Fatal("empty table")
	}
	for _, p := range table.Allowed {
		if got := ticketReturnPath(p); got != p {
			t.Errorf("allowed %q -> %q", p, got)
		}
	}
	for _, p := range table.Rejected {
		if got := ticketReturnPath(p); got != "/me/coaching/tickets" {
			t.Errorf("rejected %q -> %q", p, got)
		}
	}
}
