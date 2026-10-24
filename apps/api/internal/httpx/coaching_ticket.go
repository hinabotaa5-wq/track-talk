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
	"github.com/hinata/track-talk/apps/api/internal/stripe"
	"github.com/jackc/pgx/v5"
)

const ticketFaceYen int32 = 600

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

func (s *Server) ticketEligibility(ctx context.Context, q *db.Queries, p db.Profile) error {
	if !p.CoachingDemo {
		return &ticketRuleError{"tickets_unavailable", "チケットは招待された利用者のみ購入・利用できます"}
	}
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
	eligible := s.ticketEligibility(r.Context(), s.q, p)
	reason := ""
	if eligible != nil {
		if !writeTicketEligibilityReason(eligible, &reason) {
			MapDBError(w, eligible)
			return
		}
	}
	if !s.stripe.Configured() {
		reason = "チケットの販売は準備中です"
	}
	// 残高があれば販売休止中も利用できる。支払い済みチケットをデモ用にはしない。
	JSON(w, 200, map[string]any{"products": products, "balance": balance, "purchases": purchases, "ledger": ledger,
		"can_purchase": eligible == nil && s.stripe.Configured(), "purchase_block_reason": reason, "unit_yen": ticketFaceYen})
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
	if !s.stripe.Configured() {
		Error(w, 409, "tickets_unavailable", "チケットの販売は準備中です")
		return
	}
	var req struct {
		ProductID  string    `json:"product_id"`
		RequestKey uuid.UUID `json:"request_key"`
		ReturnPath string    `json:"return_path"`
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
		if err := s.ticketEligibility(r.Context(), q, p); err != nil {
			return err
		}
		old, err := q.GetTicketPurchaseByKey(r.Context(), db.GetTicketPurchaseByKeyParams{OwnerID: &p.ID, RequestKey: req.RequestKey})
		if err == nil {
			if old.ProductID != req.ProductID || old.ReturnPath != ticketReturnPath(req.ReturnPath) {
				return &ticketRuleError{"purchase_conflict", "同じ購入操作の商品を変更できません。画面を更新してください"}
			}
			purchase = old
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
		if isMinor(*p.BirthMonth, time.Now()) {
			msg, err := s.coachingMonthlyBlock(r.Context(), q, p.ID, uuid.Nil, uuid.Nil, uuid.Nil, product.AmountYen)
			if err != nil {
				return err
			}
			if msg != "" {
				return &ticketRuleError{"monthly_limit", msg}
			}
		}
		purchase, err = q.CreateTicketPurchase(r.Context(), db.CreateTicketPurchaseParams{ID: uuid.New(), OwnerID: &p.ID, RequestKey: req.RequestKey, ProductID: product.ID,
			Quantity: product.Quantity, AmountYen: product.AmountYen, SessionExpiresAt: time.Now().Add(time.Hour), ReturnPath: ticketReturnPath(req.ReturnPath)})
		return err
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
	sess, err := s.stripe.CreateCheckoutSession(r.Context(), stripe.CheckoutParams{Name: fmt.Sprintf("コーチングチケット%d枚", purchase.Quantity), AmountYen: int(purchase.AmountYen),
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
	if err := q.MoveTicketLot(ctx, db.MoveTicketLotParams{ID: lot, AvailableDelta: a, ReservedDelta: r, ConsumedDelta: c, RefundedDelta: f}); err != nil {
		return err
	}
	return q.InsertTicketLedger(ctx, db.InsertTicketLedgerParams{ID: uuid.New(), LotID: lot, OperationKey: key, Kind: kind, AvailableDelta: a, ReservedDelta: r, ConsumedDelta: c, RefundedDelta: f})
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
		if err := q.CreateTicketLot(ctx, db.CreateTicketLotParams{ID: id, OwnerID: p.OwnerID, Quantity: p.Quantity}); err != nil {
			return err
		}
		if err := q.InsertTicketLedger(ctx, db.InsertTicketLedgerParams{ID: uuid.New(), LotID: id, OperationKey: "purchase-" + id.String(), Kind: "purchase", AvailableDelta: p.Quantity}); err != nil {
			return err
		}
		orphan = p.OwnerID == nil
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
func (s *Server) reserveReviewTickets(ctx context.Context, q *db.Queries, p db.Profile, rr db.ReviewRequest) error {
	if err := s.ticketEligibility(ctx, q, p); err != nil {
		return err
	}
	const units int32 = 2
	price := units * ticketFaceYen
	lots, err := q.ListTicketLots(ctx, &p.ID)
	if err != nil {
		return err
	}
	var available int64
	for _, l := range lots {
		available += int64(l.Available)
	}
	if available < int64(units) {
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
		available, consumed := a.Quantity, int32(0)
		if consume {
			available = 0
			consumed = a.Quantity
		}
		if err := moveTickets(ctx, q, a.LotID, kind+"-"+t.ID.String()+"-"+a.LotID.String(), kind, available, -a.Quantity, consumed, 0); err != nil {
			return err
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
	JSON(w, 200, map[string]any{"id": purchase.ID, "status": purchase.Status, "return_path": purchase.ReturnPath})
}
