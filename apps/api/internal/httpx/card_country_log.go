package httpx

import (
	"log/slog"

	"github.com/hinata/track-talk/apps/api/internal/stripe"
)

// カード発行国が取れなかった支払いの種類。ログの payment_kind に入れ、種類ごとの件数を数えられるようにする。
const (
	cardKindReview       = "review"       // 動画添削（単発）
	cardKindTicket       = "ticket"       // チケット購入
	cardKindConsultation = "consultation" // 追加質問の都度払い
	cardKindSupport      = "support"      // 継続プランの契約決済
)

// logUnknownCardCountry は、カード発行国が取れないまま支払いを通したときに構造化ログを出す。
// 通す挙動は変えない。発行国が分かっているときは何も出さない。
// 載せるのは支払いの種類と内部の支払い ID だけで、カード番号・氏名などは含めない。
func logUnknownCardCountry(kind, paymentRef string, pi stripe.PaymentIntent) {
	if pi.CardCountry != "" {
		return
	}
	slog.Warn("card country unknown", "event", "card_country_unknown", "payment_kind", kind, "ref", paymentRef)
}
