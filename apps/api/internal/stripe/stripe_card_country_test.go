package stripe

import (
	"encoding/json"
	"testing"
)

// TestPaymentIntentCardCountry は、カード発行国の読み取り（本体 #796）。
// payment_method の展開を先に見て、なければ charges を見る。
// 未展開・不明のときは空（ブロックしない）。
func TestPaymentIntentCardCountry(t *testing.T) {
	cases := map[string]struct {
		body    string
		country string
		nonJP   bool
	}{
		"expanded us": {
			body:    `{"id":"pi_1","status":"requires_capture","amount":500,"payment_method":{"id":"pm_1","card":{"country":"us"}}}`,
			country: "US",
			nonJP:   true,
		},
		"expanded jp": {
			body:    `{"id":"pi_1","status":"requires_capture","amount":500,"payment_method":{"id":"pm_1","card":{"country":"jp"}}}`,
			country: "JP",
			nonJP:   false,
		},
		"unexpanded": {
			body:    `{"id":"pi_1","status":"requires_capture","amount":500,"payment_method":"pm_1"}`,
			country: "",
			nonJP:   false,
		},
		"charges fallback": {
			body:    `{"id":"pi_1","status":"succeeded","amount":500,"charges":{"data":[{"payment_method_details":{"card":{"country":"GB"}}}]}}`,
			country: "GB",
			nonJP:   true,
		},
		"no card": {
			body:    `{"id":"pi_1","status":"succeeded","amount":500}`,
			country: "",
			nonJP:   false,
		},
	}
	for name, tc := range cases {
		var pi PaymentIntent
		if err := json.Unmarshal([]byte(tc.body), &pi); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if pi.CardCountry != tc.country || pi.IsNonJPCard() != tc.nonJP {
			t.Fatalf("%s: country=%q nonJP=%v", name, pi.CardCountry, pi.IsNonJPCard())
		}
	}
}
