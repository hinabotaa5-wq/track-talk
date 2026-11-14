package httpx

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/push"
	"github.com/hinata/track-talk/apps/api/internal/stripe"
	"github.com/jackc/pgx/v5"
)

// reviewConsultationPriceYen は、動画添削のあとの追加質問の1回の料金。都度払いのみで、チケットは使えない
// （チケットは添削本体専用）。コーチの取り分は coach_payout.go の reviewSharePercent（70%）で決まる。
const reviewConsultationPriceYen int32 = 500

// 動画添削はチケット1枚で1回（本体 #569）。コーチの報酬は単発と同じく reviewPriceYen の70%（350円）に固定し、
// まとめ買いの割引分は運営が負担する。
const reviewTicketUnits int32 = 1

// ticketValidity は、チケットの有効期限（決済完了から6か月）。SQL の CreateTicketLot と揃える。
const ticketValidityMonths = 6

// ticketWordingVersion は、チケットの購入画面・最終確認・規約に表示している文面の版（本体 #572 v3）。
// 購入の確定時に購入ごとに残し、文面の版と購入を突き合わせられるようにする（証拠化）。
const ticketWordingVersion = "v3"

// チケットの新規販売を止めている間の文言（COACHING_TICKET_SALES_ENABLED）。
const ticketSalesClosedMessage = "チケットの新規販売は終了しました。動画添削・追加質問は申込み時にカードでお支払いください"

type ticketRuleError struct{ code, message string }

func (e *ticketRuleError) Error() string { return e.message }
func writeTicketRuleError(w http.ResponseWriter, err error) bool {
	var e *ticketRuleError
	if !errors.As(err, &e) {
		return false
	}
	Error(w, http.StatusUnprocessableEntity, e.code, e.message)
	return true
}

// 戻り先はコーチング内の決まった画面だけ。外部URLや保護者の秘密トークンを決済に渡さない。
func ticketReturnPath(path string) string {
	switch path {
	case "/coaching/new", "/me/coaching", "/me/coaching/tickets":
		return path
	}
	if strings.HasPrefix(path, "/coaching/requests/") {
		id := strings.TrimPrefix(path, "/coaching/requests/")
		if parsed, err := uuid.Parse(id); err == nil && parsed.String() == id {
			return path
		}
	}
	return "/me/coaching/tickets"
}

// reviewOpenTo は、動画添削（単発・チケット）を申し込めるか。支払いのある環境では誰でも、
// 支払いのない検証用のデモ版では招待コードを入れた人だけ（本体 #569 で招待コードのゲートを正規の購入導線に置き換えた）。
func (s *Server) reviewOpenTo(p db.Profile) bool {
	return s.stripe.Configured() || p.CoachingDemo
}

// ticketEligibility はサポートプランの追加相談に使う既存の年齢・保護者同意確認。
func (s *Server) ticketEligibility(ctx context.Context, q *db.Queries, p db.Profile) error {
	if p.BirthMonth == nil {
		return &ticketRuleError{"birth_month_required", "先に生まれた年月を登録してください"}
	}
	if !isMinor(*p.BirthMonth, time.Now()) {
		return nil
	}
	c, err := q.GetGuardianConsentByMinor(ctx, p.ID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && c.Status != "consented") {
		return &ticketRuleError{"guardian_required", "チケットの購入・利用には保護者の同意が必要です"}
	}
	return err
}

func (s *Server) coachingMonthlyBlock(ctx context.Context, q *db.Queries, owner, review, contract, purchase uuid.UUID, price int32, consultation ...uuid.UUID) (string, error) {
	limit := int32(defaultMinorMonthlyLimit)
	c, err := q.GetGuardianConsentByMinor(ctx, owner)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	if err == nil && c.Status == "consented" {
		if c.MonthlyLimitYen == nil {
			return "", nil
		}
		limit = *c.MonthlyLimitYen
	}
	exclude := uuid.Nil
	if len(consultation) > 0 {
		exclude = consultation[0]
	}
	used, err := q.SumMonthlyCoachingSpend(ctx, db.SumMonthlyCoachingSpendParams{OwnerID: owner, ExcludeReviewID: review, ExcludeContractID: contract, ExcludePurchaseID: purchase, ExcludeConsultationID: exclude})
	if err != nil {
		return "", err
	}
	if used+int64(price) <= int64(limit) {
		return "", nil
	}
	left := int32(max(int64(limit)-used, 0))
	return fmt.Sprintf("今月の上限（%s円）を超えます。今月はあと%s円です。チケット購入と指名・添削のお支払いを合算します", formatYenNumber(limit), formatYenNumber(left)), nil
}

func (s *Server) myTickets(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	products, err := s.q.ListTicketProducts(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	balance, err := s.q.TicketBalance(r.Context(), &p.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	// 残高と一緒に、いちばん早い失効日時とその枚数を出す（マイページ・購入画面に常に出す。本体 #569）。
	var nextExpiry any
	if next, err := s.q.NextTicketExpiry(r.Context(), &p.ID); err == nil {
		nextExpiry = map[string]any{"expires_at": next.ExpiresAt, "available": next.Available}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		MapDBError(w, err)
		return
	}
	byKind, err := s.ticketBalanceByKind(r.Context(), p.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	purchases, err := s.q.ListMyTicketPurchases(r.Context(), &p.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	ledger, err := s.q.ListMyTicketLedger(r.Context(), &p.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	reason := ""
	if !s.stripe.Configured() {
		reason = "チケットの販売は準備中です"
	}
	if !s.cfg.CoachingTicketSales {
		// 販売を止めている間は商品も出さない。動画添削は申込み時の都度払いだけにした。
		products = []db.CoachingTicketProduct{}
		reason = ticketSalesClosedMessage
	}
	// 残高があれば販売休止中も利用できる。支払い済みチケットをデモ用にはしない。
	JSON(w, 200, map[string]any{"products": products, "balance": balance, "next_expiry": nextExpiry, "balance_by_kind": byKind, "purchases": purchases, "ledger": ledger,
		"can_purchase": s.stripe.Configured() && s.cfg.CoachingTicketSales, "sales_open": s.cfg.CoachingTicketSales,
		"purchase_block_reason": reason, "unit_yen": reviewPriceYen, "single_price_yen": reviewPriceYen,
		"review_units": reviewTicketUnits, "validity_months": ticketValidityMonths, "terms_version": ticketWordingVersion})
}

// チケットの種別（本体 #687）。申込みで選んだ種別のロットだけを確保・消費する。
// normal は紹介特典以外のロット（購入したチケット）、reward は紹介特典のロット（無料チケット）。
const (
	ticketKindNormal = "normal"
	ticketKindReward = "reward"
)

func validTicketKind(kind string) bool {
	return kind == "" || kind == ticketKindNormal || kind == ticketKindReward
}

// ticketLotMatchesKind は、ロットが申込みで選んだ種別に当たるか。種別の指定がなければすべて当たる（従来どおり）。
func ticketLotMatchesKind(l db.CoachingTicketLot, kind string) bool {
	switch kind {
	case ticketKindNormal:
		return l.Kind != "referral_reward"
	case ticketKindReward:
		return l.Kind == "referral_reward"
	}
	return true
}

type ticketKindBalance struct {
	Available     int64      `json:"available"`
	NextExpiresAt *time.Time `json:"next_expires_at"`
}

// ticketBalanceByKind は、サマリ API に追加する種別ごとの残高（追加のみ。balance・next_expiry の意味は変えない）。
func (s *Server) ticketBalanceByKind(ctx context.Context, owner uuid.UUID) (map[string]ticketKindBalance, error) {
	rows, err := s.q.TicketBalanceByKind(ctx, &owner)
	if err != nil {
		return nil, err
	}
	out := map[string]ticketKindBalance{ticketKindNormal: {}, ticketKindReward: {}}
	for _, row := range rows {
		b := ticketKindBalance{Available: row.Available}
		if t, ok := row.NextExpiresAt.(time.Time); ok {
			b.NextExpiresAt = &t
		}
		key := ticketKindNormal
		if row.Reward {
			key = ticketKindReward
		}
		out[key] = b
	}
	return out, nil
}

func writeTicketEligibilityReason(err error, reason *string) bool {
	var e *ticketRuleError
	if !errors.As(err, &e) {
		return false
	}
	*reason = e.message
	return true
}

func (s *Server) checkoutTickets(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	if !s.cfg.CoachingTicketSales {
		Error(w, 409, "ticket_sales_closed", ticketSalesClosedMessage)
		return
	}
	if !s.stripe.Configured() {
		Error(w, 409, "tickets_unavailable", "チケットの販売は準備中です")
		return
	}
	var req struct {
		ProductID    string    `json:"product_id"`
		RequestKey   uuid.UUID `json:"request_key"`
		ReturnPath   string    `json:"return_path"`
		ReferralCode string    `json:"referral_code"`
		minorDeclarationReq
	}
	if err := DecodeJSON(r, &req); err != nil || req.RequestKey == uuid.Nil {
		FieldError(w, map[string]string{"product_id": "商品を選び直してください"})
		return
	}
	var purchase db.CoachingTicketPurchase
	err := s.tx(r.Context(), func(q *db.Queries) error {
		if err := q.LockReviewRequester(r.Context(), p.ID); err != nil {
			return err
		}
		// 単発と同じく、全員が購入のたびに注意事項のチェックを行う。
		if err := requirePurchaseNotice(true, req.GuardianPermission); err != nil {
			return err
		}
		old, err := q.GetTicketPurchaseByKey(r.Context(), db.GetTicketPurchaseByKeyParams{OwnerID: &p.ID, RequestKey: req.RequestKey})
		if err == nil {
			if old.ProductID != req.ProductID || old.ReturnPath != ticketReturnPath(req.ReturnPath) {
				return &ticketRuleError{"purchase_conflict", "同じ購入操作の商品を変更できません。画面を更新してください"}
			}
			purchase = old
			if old.Status == "pending" {
				return recordPurchaseNotice(r.Context(), q, p, minorPurchase{refType: "ticket_purchase", refID: old.ID, amount: old.AmountYen}, req.minorDeclarationReq)
			}
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		product, err := q.GetTicketProduct(r.Context(), req.ProductID)
		if errors.Is(err, pgx.ErrNoRows) {
			return &ticketRuleError{"product_unavailable", "この商品は現在購入できません"}
		}
		if err != nil {
			return err
		}
		code, err := usableReferralCode(r.Context(), q, p.ID, req.ReferralCode)
		if err != nil {
			return err
		}
		purchase, err = q.CreateTicketPurchase(r.Context(), db.CreateTicketPurchaseParams{ID: uuid.New(), OwnerID: &p.ID, RequestKey: req.RequestKey, ProductID: product.ID,
			Quantity: product.Quantity, AmountYen: product.AmountYen, ReferralCode: code, SessionExpiresAt: time.Now().Add(time.Hour), ReturnPath: ticketReturnPath(req.ReturnPath)})
		if err != nil {
			return err
		}
		return recordPurchaseNotice(r.Context(), q, p, minorPurchase{refType: "ticket_purchase", refID: purchase.ID, amount: purchase.AmountYen}, req.minorDeclarationReq)
	})
	if err != nil {
		writeCheckoutError(w, err)
		return
	}
	if purchase.Status != "pending" {
		JSON(w, 200, map[string]any{"purchase_id": purchase.ID, "paid": true, "return_path": purchase.ReturnPath})
		return
	}
	if !purchase.SessionExpiresAt.After(time.Now().Add(checkoutMinLife)) {
		Error(w, 409, "ticket_checkout_expired", "購入画面の有効期限が近づいています。新しい購入操作で開き直してください")
		return
	}
	if purchase.CheckoutUrl != nil {
		JSON(w, 200, map[string]any{"purchase_id": purchase.ID, "checkout_url": *purchase.CheckoutUrl})
		return
	}
	// 先に購入記録を確定。同じ操作を再送しても同じ購入・決済画面になり、通知が先着しても受け取れる。
	page := s.cfg.WebBaseURL + "/me/coaching/tickets"
	sess, err := s.stripe.CreateCheckoutSession(r.Context(), stripe.CheckoutParams{Name: fmt.Sprintf("動画添削チケット%d枚", purchase.Quantity), AmountYen: int(purchase.AmountYen),
		SuccessURL: page + "?purchase=" + purchase.ID.String(), CancelURL: page + "?cancelled=1&return_to=" + url.QueryEscape(purchase.ReturnPath), Reference: purchase.ID.String(),
		Metadata: map[string]string{"kind": "tickets", "purchase_id": purchase.ID.String()}, ExpiresAt: purchase.SessionExpiresAt,
		PaymentMethodTypes: []string{"card"}, IdempotencyKey: "ticket-checkout-" + purchase.ID.String()})
	if err != nil {
		writeCheckoutError(w, fmt.Errorf("%w: %w", errStripe, err))
		return
	}
	if err := s.q.SaveTicketCheckout(r.Context(), db.SaveTicketCheckoutParams{ID: purchase.ID, CheckoutSessionID: &sess.ID, CheckoutUrl: &sess.URL}); err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, 200, map[string]any{"purchase_id": purchase.ID, "checkout_url": sess.URL})
}

func moveTickets(ctx context.Context, q *db.Queries, lot uuid.UUID, key, kind string, a, r, c, f int32) error {
	return moveTicketsExpiring(ctx, q, lot, key, kind, a, r, c, f, 0)
}

// moveTicketsExpiring は moveTickets に失効した枚数（e）を足したもの。ロットの行も台帳の行も消さない。
// 台帳の purchase_ref はロットの purchase_id から写す（ロットと購入の分離、000168）。
// 紹介特典のロットには購入がないため、その行の purchase_ref は NULL になる。
func moveTicketsExpiring(ctx context.Context, q *db.Queries, lot uuid.UUID, key, kind string, a, r, c, f, e int32) error {
	if err := q.MoveTicketLot(ctx, db.MoveTicketLotParams{ID: lot, AvailableDelta: a, ReservedDelta: r, ConsumedDelta: c, RefundedDelta: f, ExpiredDelta: e}); err != nil {
		return err
	}
	l, err := q.GetTicketLot(ctx, lot)
	if err != nil {
		return err
	}
	return q.InsertTicketLedger(ctx, db.InsertTicketLedgerParams{ID: uuid.New(), LotID: lot, PurchaseRef: l.PurchaseID, OperationKey: key, Kind: kind, AvailableDelta: a, ReservedDelta: r, ConsumedDelta: c, RefundedDelta: f, ExpiredDelta: e})
}

// expireTickets は tick から呼ぶ。決済完了から6か月を過ぎたロットの未使用分を失効させる（延長・救済はしない）。
// 予約中（添削の返却待ち）の枚数は、返却で消費するか、戻すときに失効に回す（settleReviewTickets）。
func (s *Server) expireTickets(ctx context.Context) (int, error) {
	ids, err := s.q.ListTicketLotsToExpire(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, id := range ids {
		err := s.tx(ctx, func(q *db.Queries) error {
			l, err := q.LockTicketLot(ctx, id)
			if err != nil {
				return err
			}
			if l.Available == 0 || l.ExpiresAt == nil || l.ExpiresAt.After(time.Now()) {
				return nil
			}
			n += int(l.Available)
			return moveTicketsExpiring(ctx, q, l.ID, fmt.Sprintf("expire-%s-%d", l.ID, l.Expired+l.Available), "expire", -l.Available, 0, 0, 0, l.Available)
		})
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

func (s *Server) completeTicketCheckout(ctx context.Context, sess stripe.CheckoutSession) error {
	if sess.PaymentStatus != "paid" || sess.PaymentIntent == "" {
		return nil
	}
	id, err := uuid.Parse(sess.Metadata["purchase_id"])
	if err != nil {
		return fmt.Errorf("ticket checkout missing purchase id")
	}
	var orphan bool
	err = s.tx(ctx, func(q *db.Queries) error {
		p, err := q.LockTicketPurchase(ctx, id)
		if err != nil {
			return err
		}
		orphan = p.OwnerID == nil && p.Status != "refunded"
		if p.Status != "pending" {
			return nil
		}
		if (p.CheckoutSessionID != nil && *p.CheckoutSessionID != sess.ID) || sess.ClientReferenceID != id.String() || sess.AmountTotal != int(p.AmountYen) || sess.Currency != "jpy" {
			return errors.New("ticket checkout does not match purchase")
		}
		p, err = q.MarkTicketPurchasePaid(ctx, db.MarkTicketPurchasePaidParams{ID: id, PaymentIntentID: &sess.PaymentIntent, CheckoutSessionID: &sess.ID})
		if err != nil {
			return err
		}
		version := ticketWordingVersion
		if err := q.UpdateTicketPurchaseWording(ctx, db.UpdateTicketPurchaseWordingParams{ID: id, WordingVersion: &version}); err != nil {
			return err
		}
		// ロットの id は購入の id とは別の固有値にする（000168）。購入との対応は purchase_id。
		lotID := uuid.New()
		if err := q.CreateTicketLot(ctx, db.CreateTicketLotParams{ID: lotID, PurchaseID: &id, OwnerID: p.OwnerID, Quantity: p.Quantity, PaidAt: *p.PaidAt}); err != nil {
			return err
		}
		if err := q.InsertTicketLedger(ctx, db.InsertTicketLedgerParams{ID: uuid.New(), LotID: lotID, PurchaseRef: &id, OperationKey: "purchase-" + id.String(), Kind: "purchase", AvailableDelta: p.Quantity}); err != nil {
			return err
		}
		orphan = p.OwnerID == nil
		if p.ProductID == "six" || p.ProductID == "single" {
			return recordCoachingReferral(ctx, q, p.OwnerID, &p.ID, nil, p.ReferralCode)
		}
		return nil
	})
	if err != nil {
		return err
	}
	// 決済中に退会済みだった場合は利用先がないため、未使用分の返金を確保する。
	if orphan {
		_, err = s.prepareTicketRefund(ctx, id)
	}
	return err
}

// 呼び出し側は利用者のロックを持つ。依頼作成と予約を同じトランザクションにする。
// 1回の添削にチケット1枚。報酬の元になる額（nominal_yen）は単発の料金と同じにし、報酬は350円に固定する。
// 予約で使える枚数から外し（申込み時点で残高が減る）、返却で消費・報酬を計上し、返却されずに終わったら戻す。
// kind（normal／reward）を指定したら、その種別のロットだけから確保する。足りなくても他の種別では代用しない（本体 #687）。
// ロットは失効の早いものから使う（ListTicketLots の並び）。
func (s *Server) reserveReviewTickets(ctx context.Context, q *db.Queries, p db.Profile, rr db.ReviewRequest, kind string) error {
	units := reviewTicketUnits
	price := reviewPriceYen
	all, err := q.ListTicketLots(ctx, &p.ID)
	if err != nil {
		return err
	}
	lots := make([]db.CoachingTicketLot, 0, len(all))
	var available int64
	for _, l := range all {
		if !ticketLotMatchesKind(l, kind) {
			continue
		}
		lots = append(lots, l)
		available += int64(l.Available)
	}
	if available < int64(units) {
		switch kind {
		case ticketKindNormal:
			return &ticketRuleError{"ticket_kind_unavailable", "通常チケットの残りがありません。お支払い方法を選び直してください"}
		case ticketKindReward:
			return &ticketRuleError{"ticket_kind_unavailable", "無料チケットの残りがありません。お支払い方法を選び直してください"}
		}
		return &ticketRuleError{"insufficient_tickets", "チケットが不足しています。購入後にもう一度送信してください"}
	}
	reservation, err := q.CreateTicketReservation(ctx, db.CreateTicketReservationParams{ID: uuid.New(), OwnerID: &p.ID, RequestID: &rr.ID, Quantity: units, NominalYen: price, SharePercent: reviewSharePercent})
	if err != nil {
		return err
	}
	remaining := units
	for _, l := range lots {
		n := min(remaining, l.Available)
		if n == 0 {
			break
		}
		if err := q.CreateTicketAllocation(ctx, db.CreateTicketAllocationParams{ReservationID: reservation.ID, LotID: l.ID, Quantity: n}); err != nil {
			return err
		}
		if err := moveTickets(ctx, q, l.ID, "reserve-"+reservation.ID.String()+"-"+l.ID.String(), "reserve", -n, n, 0, 0); err != nil {
			return err
		}
		remaining -= n
	}
	return nil
}

func settleReviewTickets(ctx context.Context, q *db.Queries, reservation db.CoachingTicketReservation, consume bool) error {
	t, err := q.LockTicketReservation(ctx, reservation.ID)
	if err != nil {
		return err
	}
	if t.Status != "reserved" {
		return nil
	}
	allocations, err := q.ListTicketAllocations(ctx, t.ID)
	if err != nil {
		return err
	}
	kind := "release"
	if consume {
		kind = "consume"
	}
	for _, a := range allocations {
		available, consumed, expired := a.Quantity, int32(0), int32(0)
		if consume {
			available = 0
			consumed = a.Quantity
		} else {
			// 戻す時点でロットの期限を過ぎていたら、使える枚数には戻さず失効に回す。
			l, err := q.LockTicketLot(ctx, a.LotID)
			if err != nil {
				return err
			}
			forfeit := l.ExpiresAt != nil && !l.ExpiresAt.After(time.Now())
			if l.Kind == "referral_reward" {
				refunded, err := q.ReferralRewardHasRefund(ctx, l.ID)
				if err != nil {
					return err
				}
				forfeit = forfeit || refunded
			}
			if forfeit {
				available, expired = 0, a.Quantity
			}
		}
		k := kind
		if expired > 0 {
			k = "expire"
		}
		if err := moveTicketsExpiring(ctx, q, a.LotID, kind+"-"+t.ID.String()+"-"+a.LotID.String(), k, available, -a.Quantity, consumed, 0, expired); err != nil {
			return err
		}
		if !consume && expired > 0 {
			if err := q.CancelReleasedReferralReward(ctx, a.LotID); err != nil {
				return err
			}
		}
	}
	state := "released"
	if consume {
		state = "consumed"
	}
	if err := q.SettleTicketReservation(ctx, db.SettleTicketReservationParams{ID: t.ID, Status: state}); err != nil {
		return err
	}
	if consume {
		return q.RecordTicketReviewEarnings(ctx, t.ID)
	}
	return nil
}

func (s *Server) releaseExpiredTickets(ctx context.Context) (int, error) {
	rows, err := s.q.ListTicketReservationsToRelease(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, t := range rows {
		released := false
		err = s.tx(ctx, func(q *db.Queries) error {
			// 回答処理と同じ順（依頼→予約→ロット）でロックする。
			if t.RequestID != nil {
				rr, e := q.LockTicketReview(ctx, *t.RequestID)
				if e != nil && !errors.Is(e, pgx.ErrNoRows) {
					return e
				}
				if e == nil && rr.Status != "expired" {
					return nil
				}
			}
			current, e := q.LockTicketReservation(ctx, t.ID)
			if e != nil {
				return e
			}
			if current.Status != "reserved" {
				return nil
			}
			if e = settleReviewTickets(ctx, q, current, false); e != nil {
				return e
			}
			released = true
			return nil
		})
		if err != nil {
			return n, err
		}
		if released {
			n++
		}
	}
	return n, nil
}

func (s *Server) attachTicketReservation(ctx context.Context, out map[string]any, id uuid.UUID) error {
	t, err := s.q.GetTicketReservationForReview(ctx, &id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	out["tickets"] = map[string]any{"quantity": t.Quantity, "status": t.Status, "nominal_yen": t.NominalYen}
	return nil
}

func (s *Server) myTicketPurchase(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	purchase, err := s.q.GetTicketPurchase(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (purchase.OwnerID == nil || *purchase.OwnerID != p.ID)) {
		Error(w, 404, "not_found", "見つかりませんでした")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	// 戻り画面は URL（?purchase=）だけで「購入済み」と決めない。支払い待ちの間は Stripe の支払い画面の状態を読み、
	// 完了していれば Webhook と同じ処理で残高を付ける（何度通しても1回分。Webhook を正とする設計は変えない。本体 #569）。
	checkout := ""
	if purchase.Status == "pending" && purchase.CheckoutSessionID != nil && s.stripe.Configured() {
		sess, err := s.stripe.GetCheckoutSession(r.Context(), *purchase.CheckoutSessionID)
		if err != nil {
			writeCheckoutError(w, fmt.Errorf("%w: %w", errStripe, err))
			return
		}
		checkout = sess.Status
		if sess.ID == *purchase.CheckoutSessionID && sess.Status == "complete" {
			if err := s.completeTicketCheckout(r.Context(), sess); err != nil {
				MapDBError(w, err)
				return
			}
			if purchase, err = s.q.GetTicketPurchase(r.Context(), id); err != nil {
				MapDBError(w, err)
				return
			}
		}
	}
	out := map[string]any{"id": purchase.ID, "status": purchase.Status, "return_path": purchase.ReturnPath, "checkout_status": checkout,
		"quantity": purchase.Quantity, "amount_yen": purchase.AmountYen, "paid_at": purchase.PaidAt, "expires_at": nil}
	if purchase.Status != "pending" {
		if l, err := s.q.GetPurchasedTicketLot(r.Context(), &id); err == nil {
			out["expires_at"] = l.ExpiresAt
		}
	}
	JSON(w, 200, out)
}

// remindTicketExpiry は tick から呼ぶ。失効の7日前と30日前に、未使用の枚数と失効日時を本人に知らせる（本体 #569）。
// 7日前を先に見るので、7日前を切ってから見つけたロットには7日前の知らせだけを送る。送る前に印を付け、二重に送らない。
func (s *Server) remindTicketExpiry(ctx context.Context) (int, error) {
	n := 0
	for _, days := range []int32{7, 30} {
		rows, err := s.q.ListTicketLotsToRemind(ctx, days)
		if err != nil {
			return n, err
		}
		for _, l := range rows {
			if _, err := s.q.MarkTicketLotReminded(ctx, db.MarkTicketLotRemindedParams{ID: l.ID, Days: days}); errors.Is(err, pgx.ErrNoRows) {
				continue
			} else if err != nil {
				return n, err
			}
			s.notify(ctx, *l.OwnerID, notifyImportant, ticketExpiryMessage(l.ID, days), 24*time.Hour)
			n++
		}
	}
	return n, nil
}

// ticketExpiryMessage は、チケットの失効が近いことの知らせ。表題1行だけにし、
// 枚数と失効日時はマイページのチケットの画面で見せる（本体 #609）。
func ticketExpiryMessage(lot uuid.UUID, days int32) push.Message {
	return push.Message{
		Title: fmt.Sprintf("チケットの有効期限まであと%d日以内です", days),
		URL:   "/me/coaching/tickets",
		Tag:   fmt.Sprintf("ticket-expiry-%d-%s", days, lot),
	}
}
