package httpx

import "testing"

// 本体 #1031 C2・C7：支払額は価格 − ¥500（0円未満にしない）。0円なら支払い画面を飛ばす。
func TestCouponCashPriceAndPaymentSkip(t *testing.T) {
	cases := []struct {
		price, discount, cash int32
		pay                   bool
	}{
		{1000, couponDiscount(1000, referralCouponYen), 500, true},
		{500, couponDiscount(500, referralCouponYen), 0, false},
		{300, couponDiscount(300, referralCouponYen), 0, false},
		{1000, 0, 1000, true},
	}
	for _, c := range cases {
		if c.discount > c.price {
			t.Fatalf("discount %d exceeds price %d", c.discount, c.price)
		}
		got := couponCashPrice(c.price, c.discount)
		if got != c.cash {
			t.Errorf("couponCashPrice(%d, %d) = %d, want %d", c.price, c.discount, got, c.cash)
		}
		if reviewNeedsPayment(true, got) != c.pay {
			t.Errorf("reviewNeedsPayment(true, %d) = %v, want %v", got, !c.pay, c.pay)
		}
	}
	if reviewNeedsPayment(false, 500) {
		t.Error("payment required without Stripe")
	}
}
