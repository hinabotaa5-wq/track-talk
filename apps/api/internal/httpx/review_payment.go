package httpx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/push"
	"github.com/hinata/track-talk/apps/api/internal/stripe"
	"github.com/jackc/pgx/v5"
)

// フォーム添削の支払い（Stripe Checkout）。Stripe の鍵があるときだけ使い、なければ支払いなしのデモ版。
// 送ると支払い画面に進み、支払いが届いたら選んだコーチに依頼が届く。
// カード（Apple Pay・Google Pay を含む）は仮押さえして、返却で確定、目安を過ぎたら取り消す。
// PayPay は仮押さえができないので、その場で払って、目安を過ぎたら返金する。

// 動画添削の料金。どのコーチを選んでも同じ（コーチごとの価格差はつくらない）。報酬はこの70%。
const reviewPriceYen int32 = 1500

// 保護者が同意していない18歳未満の、保護者が払う契約の上限（design-review.md「18歳未満の支払い」）。
// 本人が自分で払う動画添削・追加相談は、保護者の同意がないと払えない（minorSelfPayBlock）。
const defaultMinorMonthlyLimit = 3000

func validReviewPayMethod(m string) bool { return m == "card" || m == "paypay" }

func reviewPaymentJSON(p db.ReviewPayment) map[string]any {
	return map[string]any{
		"method":      p.Method,
		"amount_yen":  p.AmountYen,
		"status":      p.Status,
		"captured_at": p.CapturedAt,
		"canceled_at": p.CanceledAt,
		"refunded_at": p.RefundedAt,
	}
}

// attachReviewPayment は、依頼の JSON に支払いを足す。支払いがなければ null。
func (s *Server) attachReviewPayment(ctx context.Context, out map[string]any, id uuid.UUID) error {
	if err := s.attachTicketReservation(ctx, out, id); err != nil {
		return err
	}
	out["payment"] = nil
	p, err := s.q.GetLatestReviewPayment(ctx, &id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	out["payment"] = reviewPaymentJSON(p)
	return nil
}

// reviewCheckoutURL は、依頼の支払い画面の URL を返す。まだ使える画面があればそれを、なければ新しく作る。
func (s *Server) reviewCheckoutURL(ctx context.Context, q *db.Queries, rr db.ReviewRequest, method string) (string, error) {
	p, err := q.GetReusableReviewPayment(ctx, db.GetReusableReviewPaymentParams{RequestID: &rr.ID, Method: method})
	if err == nil {
		return p.CheckoutUrl, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	price := reviewCashPrice(rr)
	id := uuid.New()
	page := s.cfg.WebBaseURL + "/coaching/requests/" + rr.ID.String()
	name := "動画添削"
	params := stripe.CheckoutParams{
		Name:           name,
		AmountYen:      int(price),
		SuccessURL:     page + "?paid=1",
		CancelURL:      page,
		Reference:      id.String(),
		Metadata:       map[string]string{"kind": "review", "request_id": rr.ID.String(), "payment_id": id.String()},
		ExpiresAt:      time.Now().Add(checkoutMinLife),
		IdempotencyKey: "checkout-" + id.String(),
	}
	if method == "card" {
		// Apple Pay・Google Pay はカードとして出る。
		params.PaymentMethodTypes = []string{"card"}
		params.CaptureManual = true
	} else {
		params.PaymentMethodTypes = []string{"paypay"}
	}
	sess, err := s.stripe.CreateCheckoutSession(ctx, params)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errStripe, err)
	}
	expires := params.ExpiresAt
	if sess.ExpiresAt > 0 {
		expires = time.Unix(sess.ExpiresAt, 0)
	}
	if _, err := q.CreateReviewPayment(ctx, db.CreateReviewPaymentParams{
		ID: id, RequestID: &rr.ID, PayerID: &rr.RequesterID, Method: method, AmountYen: price,
		CheckoutSessionID: sess.ID, CheckoutUrl: sess.URL, SessionExpiresAt: expires,
	}); err != nil {
		return "", err
	}
	return sess.URL, nil
}

// checkoutReviewRequest は、支払いがまだの依頼の支払い画面を開き直す。
func (s *Server) checkoutReviewRequest(w http.ResponseWriter, r *http.Request) {
	rr, ok := s.requesterReview(w, r)
	if !ok {
		return
	}
	var req struct {
		Method string `json:"method"`
	}
	if err := DecodeJSON(r, &req); err != nil || !validReviewPayMethod(req.Method) {
		FieldError(w, map[string]string{"method": "支払い方法を選んでください"})
		return
	}
	if rr.Status != "awaiting_payment" || !rr.DueAt.After(time.Now().Add(checkoutMinLife)) {
		Error(w, http.StatusConflict, "conflict", "この依頼はお支払いを待っていません")
		return
	}
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	s.writeReviewCheckout(w, r, rr, p, req.Method, http.StatusOK)
}

// writeReviewCheckout は、月の上限を確かめてから支払い画面を作り、依頼と一緒に返す。
func (s *Server) writeReviewCheckout(w http.ResponseWriter, r *http.Request, rr db.ReviewRequest, p db.Profile, method string, status int) {
	url, block, err := s.reviewCheckoutFor(r.Context(), rr, p, method)
	if err != nil {
		writeCheckoutError(w, err)
		return
	}
	if block != nil {
		s.writeMinorBlock(w, r, block)
		return
	}
	out := s.reviewRequestJSON(rr)
	out["checkout_url"] = url
	JSON(w, status, out)
}

// reviewCheckoutFor は、支払い画面の URL を返す。18歳未満は、保護者の有効な同意と月の上限を確かめ、
// 払えないときは理由を返す。同時に開かれても上限を超えないよう、確かめてから支払いの行を作るまでを
// 本人ごとのロックの中で行う（支払い画面を開いている分も今月の合計に数える）。
func (s *Server) reviewCheckoutFor(ctx context.Context, rr db.ReviewRequest, p db.Profile, method string) (string, *minorBlockError, error) {
	if p.BirthMonth == nil || !isMinor(*p.BirthMonth, time.Now()) {
		url, err := s.reviewCheckoutURL(ctx, s.q, rr, method)
		return url, nil, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", nil, err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)
	if err := q.LockReviewRequester(ctx, p.ID); err != nil {
		return "", nil, err
	}
	block, err := s.minorSelfPayBlock(ctx, q, p.ID, minorPurchase{refType: "review", coachName: rr.RequestedCoachName, price: reviewCashPrice(rr)}, rr.ID, uuid.Nil)
	if err != nil || block != nil {
		return "", block, err
	}
	// 支払いの行もロックを持つ接続で作る。別の接続を待つと、同時に来たときに接続が尽きて止まる。
	url, err := s.reviewCheckoutURL(ctx, q, rr, method)
	if err != nil {
		return "", nil, err
	}
	return url, nil, tx.Commit(ctx)
}

// completeReviewCheckout は、添削の支払い（カードは仮押さえ）が届いたら依頼を割り当ての待ち行列に入れる。
// 入れられないもの（目安を過ぎた、二重の支払い）は取り消すか返金する。
func (s *Server) completeReviewCheckout(ctx context.Context, sess stripe.CheckoutSession) error {
	if sess.PaymentIntent == "" {
		return nil
	}
	pay, err := s.q.GetReviewPaymentBySession(ctx, sess.ID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && pay.Status != "pending") {
		return nil
	}
	if err != nil {
		return err
	}
	status := "captured"
	if pay.Method == "card" {
		// 仮押さえのときは payment_status が unpaid のまま届くので、支払いそのものの状態を確かめる。
		pi, err := s.stripe.GetPaymentIntent(ctx, sess.PaymentIntent)
		if err != nil {
			return err
		}
		switch pi.Status {
		case "requires_capture":
			status = "authorized"
		case "succeeded":
		default:
			return nil
		}
	} else if sess.PaymentStatus != "paid" {
		return nil
	}
	var started *db.ReviewRequest
	var notice *db.GuardianNotice
	err = s.tx(ctx, func(q *db.Queries) error {
		var err error
		pay, err = q.MarkReviewPaymentPaid(ctx, db.MarkReviewPaymentPaidParams{
			Status: status, PaymentIntentID: &sess.PaymentIntent, CheckoutSessionID: sess.ID,
		})
		if err != nil {
			return err
		}
		if pay.RequestID == nil {
			return nil
		}
		rr, err := q.StartPaidReviewRequest(ctx, *pay.RequestID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		started = &rr
		// 18歳未満が自分で払ったときは、保護者への購入の通知を支払いと一緒に予約する。
		notice, err = queueGuardianPurchaseNotice(ctx, q, rr.RequesterID, "review", rr.ID, pay.AmountYen, rr.RequestedCoachName)
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if started == nil {
		if err := s.releaseReviewPayment(ctx, pay); err != nil {
			// 取り消せなかったものは tick が拾い直す。
			slog.Error("release unstarted review payment", "payment", pay.ID, "err", err)
		}
		return nil
	}
	if notice != nil {
		s.sendGuardianNotice(ctx, *notice)
	}
	if _, err := s.offerReview(ctx, started.ID); err != nil {
		slog.Warn("review offer after payment", "err", err)
	}
	return nil
}

// releaseReviewPayment は、仮押さえを取り消すか、確定済み（PayPay）なら全額を返金する。
func (s *Server) releaseReviewPayment(ctx context.Context, pay db.ReviewPayment) error {
	if pay.PaymentIntentID == nil {
		return errors.New("payment has no payment intent")
	}
	var err error
	if pay.Status == "authorized" {
		_, err = s.stripe.CancelPaymentIntent(ctx, *pay.PaymentIntentID, "cancel-"+pay.ID.String())
	} else {
		_, err = s.stripe.CreateRefund(ctx, *pay.PaymentIntentID, int(pay.AmountYen), "refund-"+pay.ID.String())
	}
	if err != nil {
		return fmt.Errorf("%w: %w", errStripe, err)
	}
	_, err = s.q.MarkReviewPaymentReleased(ctx, pay.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	return err
}

// captureReviewPayment は、返却された依頼の仮押さえを確定する。
func (s *Server) captureReviewPayment(ctx context.Context, pay db.ReviewPayment) error {
	if pay.PaymentIntentID == nil {
		return errors.New("payment has no payment intent")
	}
	if _, err := s.stripe.CapturePaymentIntent(ctx, *pay.PaymentIntentID, "capture-"+pay.ID.String()); err != nil {
		return fmt.Errorf("%w: %w", errStripe, err)
	}
	_, err := s.q.MarkReviewPaymentCaptured(ctx, pay.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	return err
}

// captureReturnedReview は、返答を出したあとに呼ぶ。確定できなかったものは tick が拾い直す。
func (s *Server) captureReturnedReview(ctx context.Context, requestID uuid.UUID) {
	if !s.stripe.Configured() {
		return
	}
	pay, err := s.q.GetLatestReviewPayment(ctx, &requestID)
	if err != nil || pay.Status != "authorized" {
		return
	}
	if err := s.captureReviewPayment(ctx, pay); err != nil {
		slog.Error("capture review payment", "payment", pay.ID, "err", err)
	}
}

// touchReviewPayment は、Stripe で処理できなかった支払いを次の tick で後ろに回す。
// 断られ続ける支払いが50件たまっても、ほかの支払いが処理されないままにならないようにする。
func (s *Server) touchReviewPayment(ctx context.Context, id uuid.UUID) {
	if err := s.q.TouchReviewPayment(ctx, id); err != nil {
		slog.Error("touch review payment", "payment", id, "err", err)
	}
}

// settleReviewPayments は tick から呼ぶ。返却されたものを確定し、目安を過ぎたもの・始められなかったものを
// 取り消すか返金し、払わないまま支払い画面が閉じた依頼を消す。
func (s *Server) settleReviewPayments(ctx context.Context) (released int, err error) {
	if !s.stripe.Configured() {
		return 0, nil
	}
	capture, err := s.q.ListReviewPaymentsToCapture(ctx)
	if err != nil {
		return 0, err
	}
	for _, pay := range capture {
		if err := s.captureReviewPayment(ctx, pay); err != nil {
			slog.Error("capture review payment", "payment", pay.ID, "err", err)
			s.touchReviewPayment(ctx, pay.ID)
		}
	}
	release, err := s.q.ListReviewPaymentsToRelease(ctx)
	if err != nil {
		return 0, err
	}
	for _, pay := range release {
		if err := s.releaseReviewPayment(ctx, pay); err != nil {
			slog.Error("release review payment", "payment", pay.ID, "err", err)
			s.touchReviewPayment(ctx, pay.ID)
			continue
		}
		released++
		if pay.PayerID != nil && pay.Status == "captured" {
			s.notify(ctx, *pay.PayerID, notifyImportant, push.Message{
				Title: "フォーム添削の代金を返金しました",
				Body:  "添削が成立しなかったため、お支払い額を全額返金しました",
				URL:   "/me/coaching/requests",
				Tag:   "review-refund-" + pay.ID.String(),
			}, 24*time.Hour)
		}
	}
	if _, err := s.q.DeleteUnpaidReviewRequests(ctx); err != nil {
		return released, err
	}
	if _, err := s.releaseExpiredTickets(ctx); err != nil {
		return released, err
	}
	return released, nil
}
