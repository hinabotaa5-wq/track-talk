package stripe

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// 返金には永続的な操作IDを付ける。再送前に検索し、Stripeの冪等キーの保持期間を過ぎても二重返金を防ぐ。
func (c Client) CreateRefundWithMetadata(ctx context.Context, intent string, amount int, operation string) (Refund, error) {
	form := url.Values{}
	form.Set("payment_intent", intent)
	form.Set("amount", strconv.Itoa(amount))
	form.Set("metadata[ticket_refund_id]", operation)
	var out Refund
	err := c.post(ctx, "/v1/refunds", form, "ticket-refund-"+operation, &out)
	return out, err
}

func (c Client) FindTicketRefund(ctx context.Context, intent, operation string) (*Refund, error) {
	return c.findRefund(ctx, intent, operation, "")
}

// FindRefundByID never authorizes creating another refund if a known refund is absent.
func (c Client) FindRefundByID(ctx context.Context, intent, refundID string) (*Refund, error) {
	return c.findRefund(ctx, intent, "", refundID)
}
func (c Client) findRefund(ctx context.Context, intent, operation, refundID string) (*Refund, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	cursor := ""
	for {
		query := url.Values{"payment_intent": {intent}, "limit": {"100"}}
		if cursor != "" {
			query.Set("starting_after", cursor)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base()+"/v1/refunds?"+query.Encode(), nil)
		if err != nil {
			return nil, err
		}
		req.SetBasicAuth(c.Secret, "")
		var out struct {
			Data    []Refund `json:"data"`
			HasMore bool     `json:"has_more"`
		}
		if err := c.do(req, &out); err != nil {
			return nil, err
		}
		for _, r := range out.Data {
			if (refundID != "" && r.ID == refundID) || (refundID == "" && r.Metadata["ticket_refund_id"] == operation) {
				return &r, nil
			}
		}
		if !out.HasMore {
			return nil, nil
		}
		if len(out.Data) == 0 || out.Data[len(out.Data)-1].ID == cursor {
			return nil, fmt.Errorf("refund pagination did not advance")
		}
		cursor = out.Data[len(out.Data)-1].ID
	}
}
