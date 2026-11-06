package httpx_test

import (
	"net/url"
	"strconv"
	"testing"

	"github.com/google/uuid"
)

// paymentMethodTypes は、支払い画面を作るときに Stripe へ渡した支払い方法の並び。
func paymentMethodTypes(form url.Values) []string {
	var out []string
	for i := 0; ; i++ {
		v := form.Get("payment_method_types[" + strconv.Itoa(i) + "]")
		if v == "" {
			return out
		}
		out = append(out, v)
	}
}

func sameMethods(got []string, want ...string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// サポートプランの契約とその追加相談の支払い画面はカードだけ。PayPay を有効にしても出ない。
func TestSupportPlanPaymentIsCardOnly(t *testing.T) {
	h, f, reviewOwner, _, reviewRoot := answeredCashReviewRoom(t, nil)

	// サポートプランの契約の支払い。
	tu, _, id := adultAccepted(t, h)
	checkout(t, h, tu, id)
	if got := paymentMethodTypes(f.lastSession()); !sameMethods(got, "card") {
		t.Fatalf("contract checkout methods %v", got)
	}

	// サポートプランの追加相談。
	contractOwner, _, contract := endedCashRoom(t, h)
	cc := consultationDraft(t, h, contractOwner, "/v1/coaching/contracts/"+contract+"/consultations", uuid.NewString(), "契約の相談")
	consultationAction(t, h, contractOwner, cc.ID, "submit", nil)
	form := f.lastSession()
	if form.Get("metadata[consultation_id]") != cc.ID {
		t.Fatalf("contract consultation session %v", form)
	}
	if got := paymentMethodTypes(form); !sameMethods(got, "card") {
		t.Fatalf("contract consultation methods %v", got)
	}

	// 動画添削のあとの追加相談も現在はカードのみ。
	rc := consultationDraft(t, h, reviewOwner, "/v1/coaching/requests/"+reviewRoot+"/consultations", uuid.NewString(), "添削の相談")
	consultationAction(t, h, reviewOwner, rc.ID, "submit", nil)
	form = f.lastSession()
	if form.Get("metadata[consultation_id]") != rc.ID {
		t.Fatalf("review consultation session %v", form)
	}
	if got := paymentMethodTypes(form); !sameMethods(got, "card") {
		t.Fatalf("review consultation methods %v", got)
	}
}
