package httpx

import "testing"

// splitPostExpiryRefund の直接 unit test（B U-u1）。
// 期待挙動の根拠は正本の決まり：1枚500円（PREMISES §1 L18、reviewPriceYen）、
// Stripe へは決済額−Stripe返済済額まで・超えた分は Stripe の外で返す（PREMISES §1 L22、運営者決定 2026-11-12）。
// 間接の HTTP 級テスト（coaching_ticket_test.go）が踏む境界（3000=2500+500、
// 全額 Stripe、他返金後残額 417+83）と同じ値を直接の単位でも固定する。
func TestSplitPostExpiryRefund(t *testing.T) {
	for _, tc := range []struct {
		name                                 string
		requested, chargeYen, stripeRefunded int32
		wantStripe, wantManual               int32
	}{
		// 6枚 3000円のうち Stripe に残る 2500円まで・残り500円は Stripe 外（間接 L413 と同値）。
		{"within-charge", 3000, 2500, 0, 2500, 500},
		// 要求額が決済額に収まるときは全額 Stripe（間接 L617 と同値）。
		{"all-stripe", 2500, 2500, 0, 2500, 0},
		// 通常返金で 2083円を Stripe へ返し済みなら残り417円だけ Stripe・83円は Stripe 外（間接 L641 と同値）。
		{"after-other-refund", 500, 2500, 2083, 417, 83},
		// 要求額ちょうど＝残額ちょうどは全額 Stripe・Stripe 外0。
		{"exact", 417, 2500, 2083, 417, 0},
		// 境界±1：残額+1円の要求は1円だけ Stripe 外、残額−1円は全額 Stripe。
		{"one-over", 418, 2500, 2083, 417, 1},
		{"one-under", 416, 2500, 2083, 416, 0},
		// Stripe の残り0（6枚目。間接 L617 の2回目と同値）は全額 Stripe 外。
		{"room-exhausted", 500, 2500, 2500, 0, 500},
		// 返済済みが決済額を超えていても残りは0止め（負にしない）。
		{"over-refunded-clamps", 500, 2500, 2600, 0, 500},
		// 要求0は両方0。
		{"zero-requested", 0, 2500, 0, 0, 0},
		// 決済額0（Stripe で取っていない分）は全額 Stripe 外。
		{"zero-charge", 500, 0, 0, 0, 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stripePart, manualPart := splitPostExpiryRefund(tc.requested, tc.chargeYen, tc.stripeRefunded)
			if stripePart != tc.wantStripe || manualPart != tc.wantManual {
				t.Fatalf("splitPostExpiryRefund(%d, %d, %d) = (%d, %d), want (%d, %d)",
					tc.requested, tc.chargeYen, tc.stripeRefunded, stripePart, manualPart, tc.wantStripe, tc.wantManual)
			}
			if stripePart+manualPart != tc.requested {
				t.Fatalf("parts do not add up to requested: %d + %d != %d", stripePart, manualPart, tc.requested)
			}
		})
	}
}

// 定義のコメント「複数回に分けても累計で同じ計算になる」の直接確認。
// 5枚→1枚の順（間接 L617 と同じ流れ）で Stripe 累計は決済額 2500円に一致する。
func TestSplitPostExpiryRefundSequentialMatchesCharge(t *testing.T) {
	const chargeYen = 2500
	s1, m1 := splitPostExpiryRefund(2500, chargeYen, 0)
	s2, m2 := splitPostExpiryRefund(500, chargeYen, s1)
	if s1 != 2500 || m1 != 0 || s2 != 0 || m2 != 500 {
		t.Fatalf("sequential split = (%d, %d) then (%d, %d)", s1, m1, s2, m2)
	}
	if s1+s2 != chargeYen {
		t.Fatalf("stripe total %d != charge %d", s1+s2, chargeYen)
	}
}
