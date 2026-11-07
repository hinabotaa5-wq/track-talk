package httpx

import (
	"testing"

	"github.com/google/uuid"
)

func TestReviewWithdrawalPushText(t *testing.T) {
	id := uuid.New()
	const body = "担当のコーチの都合で、この依頼は終了しました"
	for _, tc := range []struct {
		paymentConfigured bool
		want              string
	}{
		{false, body},
		{true, body + "。お支払いは取り消します（PayPay は返金します）"},
	} {
		got := reviewWithdrawalPush(id, tc.paymentConfigured)
		if got.Body != tc.want || got.URL != "/coaching/requests/"+id.String() {
			t.Fatalf("withdrawal message: %+v", got)
		}
	}
}
