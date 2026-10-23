package stripe

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFindTicketRefundUsesAllPagesAndOperationID(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		user, _, ok := r.BasicAuth()
		if !ok || user != "sk_test" || r.URL.Query().Get("payment_intent") != "pi_ticket" || r.URL.Query().Get("limit") != "100" {
			t.Errorf("incorrect request: %s", r.URL)
		}
		if r.URL.Query().Get("starting_after") == "" {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []Refund{{ID: "re_other", Amount: 500, Metadata: map[string]string{"ticket_refund_id": "other"}}}, "has_more": true})
			return
		}
		if r.URL.Query().Get("starting_after") != "re_other" {
			t.Errorf("bad cursor: %s", r.URL)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []Refund{{ID: "re_found", Amount: 1440, Status: "succeeded", Metadata: map[string]string{"ticket_refund_id": "operation"}}}, "has_more": false})
	}))
	defer server.Close()
	c := Client{Secret: "sk_test", APIBase: server.URL}
	refund, err := c.FindTicketRefund(context.Background(), "pi_ticket", "operation")
	if err != nil || refund == nil || refund.ID != "re_found" || calls != 2 {
		t.Fatalf("refund %+v calls=%d err=%v", refund, calls, err)
	}
}

func TestCreateTicketRefundKeepsPermanentOperationMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.PostForm.Get("amount") != "1440" || r.PostForm.Get("payment_intent") != "pi_ticket" || r.PostForm.Get("metadata[ticket_refund_id]") != "operation" || r.Header.Get("Idempotency-Key") != "ticket-refund-operation" {
			t.Errorf("refund request %v", r.PostForm)
		}
		_ = json.NewEncoder(w).Encode(Refund{ID: "re_ticket", Amount: 1440, Status: "pending", Metadata: map[string]string{"ticket_refund_id": "operation"}})
	}))
	defer server.Close()
	c := Client{Secret: "sk_test", APIBase: server.URL}
	refund, err := c.CreateRefundWithMetadata(context.Background(), "pi_ticket", 1440, "operation")
	if err != nil || refund.ID != "re_ticket" || refund.Status != "pending" {
		t.Fatalf("refund %+v %v", refund, err)
	}
}

func TestFindTicketRefundRejectsBrokenPagination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[],"has_more":true}`))
	}))
	defer server.Close()
	c := Client{Secret: "sk_test", APIBase: server.URL}
	if _, err := c.FindTicketRefund(context.Background(), "pi_ticket", "operation"); err == nil {
		t.Fatal("expected pagination error")
	}
}
