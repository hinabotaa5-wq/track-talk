package httpx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/stripe"
	"github.com/jackc/pgx/v5"
)

// フォーム添削の支払い（Stripe Checkout）。Stripe の鍵があるときだけ使い、なければ支払いなしのデモ版。
// 送ると支払い画面に進み、支払いが届いたら選んだコーチに依頼が届く。
// カード（Apple Pay・Google Pay を含む）は仮押さえして、返却で確定、目安を過ぎたら取り消す。
// PayPay は仮押さえができないので、その場で払って、目安を過ぎたら返金する（今は受け付けていない。reviewPaymentMethods）。

// 動画添削の料金（単発1回）。どのコーチを選んでも同じ（コーチごとの価格差はつくらない）。
// 報酬はこの70%（350円）。チケット（6枚2,500円）で払った回も報酬は同じ350円（本体 #569）。
// 変えるときは migrations の review_requests.base_price_yen の既定値と、画面の lib/review-pricing.ts も揃える。
const reviewPriceYen int32 = 500

// 保護者が同意していない18歳未満の、保護者が払う契約の上限（design-review.md「18歳未満の支払い」）。
// 本人が自分で払う動画添削・追加相談は、この上限ではなく購入のたびの宣言で確かめる（minor_purchase.go）。
const defaultMinorMonthlyLimit = 3000

// validReviewPayMethod は、動画添削の申込みで受け付ける支払い方法か。今使えない方法（PayPay）は断る。
func validReviewPayMethod(m string) bool { return slices.Contains(reviewPaymentMethods, m) }

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
// minor は、18歳未満の本人が自分で払う支払いか。支払いの行に印を付け、返金を後から集計できるようにする。
func (s *Server) reviewCheckoutURL(ctx context.Context, q *db.Queries, rr db.ReviewRequest, method string, code *string, minor bool) (string, error) {
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
		CheckoutSessionID: sess.ID, CheckoutUrl: sess.URL, SessionExpiresAt: expires, Minor: minor, ReferralCode: code,
	}); err != nil {
		return "", err
	}
	return sess.URL, nil
}

// checkoutReviewRequest は、支払いがまだの依頼の支払い画面を開き直す。
// 18歳未満は、開き直すときも宣言（guardian_permission）が要る。
func (s *Server) checkoutReviewRequest(w http.ResponseWriter, r *http.Request) {
	rr, ok := s.requesterReview(w, r)
	if !ok {
		return
	}
	var req struct {
		Method       string `json:"method"`
		ReferralCode string `json:"referral_code"`
		minorDeclarationReq
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
	url, err := s.reviewCheckoutFor(r.Context(), rr, p, req.Method, req.ReferralCode, req.minorDeclarationReq)
	if err != nil {
		writeCheckoutError(w, err)
		return
	}
	out := s.reviewRequestJSON(rr)
	out["checkout_url"] = url
	JSON(w, http.StatusOK, out)
}

// reviewCheckoutFor は、全員の購入前チェック（decl）を確かめ、支払い画面の URL を返す。
// 支払い画面を作るのと同じトランザクションで宣言を残す。保護者同意の有無と月の上限では止めない。
// 本人・依頼のロック内で記録し、年齢による分岐は置かない。
func (s *Server) reviewCheckoutFor(ctx context.Context, rr db.ReviewRequest, p db.Profile, method, referralCode string, decl minorDeclarationReq) (string, error) {
	if err := requirePurchaseNotice(true, decl.GuardianPermission); err != nil {
		return "", err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)
	// 1つの申込みに支払い画面は1つ（本体 #569）。同時に開き直しても、依頼の行のロックで順に通し、
	// 2つ目は1つ目が作った画面をそのまま返す（GetReusableReviewPayment）。
	if _, err := q.LockTicketReview(ctx, rr.ID); err != nil {
		return "", err
	}
	if err := q.LockReviewRequester(ctx, p.ID); err != nil {
		return "", err
	}
	coach := rr.RequestedCoachID
	if coach == nil {
		coach = rr.CoachID
	}
	if err := recordPurchaseNotice(ctx, q, p, minorPurchase{refType: "review", refID: rr.ID, amount: reviewCashPrice(rr), coachID: coach}, decl); err != nil {
		return "", err
	}
	// 支払いの行もロックを持つ接続で作る。別の接続を待つと、同時に来たときに接続が尽きて止まる。
	code, err := usableReferralCode(ctx, q, p.ID, referralCode)
	if err != nil {
		return "", err
	}
	url, err := s.reviewCheckoutURL(ctx, q, rr, method, code, false)
	if err != nil {
		return "", err
	}
	return url, tx.Commit(ctx)
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
	err = s.tx(ctx, func(q *db.Queries) error {
		if err := q.LockReviewAssignment(ctx); err != nil {
			return err
		}
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
		rr, err = s.assignReview(ctx, q, rr)
		if err != nil {
			return err
		}
		started = &rr
		if status == "captured" && pay.AmountYen == reviewPriceYen {
			return recordCoachingReferral(ctx, q, pay.PayerID, nil, &pay.ID, pay.ReferralCode)
		}
		// 単発カードは決済完了（仮押さえ）の時点で紹介IDを発行し、カウントは capture で行う（本体 #686）。
		// 紹介IDを入れた購入は capture で数えられ得るので、ここでは起点にしない（数えられなければ capture で発行）。
		// この起点の周は capture まで付与を保留し、仮押さえが解放されたら DB のトリガーで無効になる。
		if status == "authorized" && pay.AmountYen == reviewPriceYen && pay.PayerID != nil && pay.ReferralCode == nil {
			return q.IssueCoachingReferralOrigin(ctx, db.IssueCoachingReferralOriginParams{Owner: *pay.PayerID, PaymentID: &pay.ID})
		}
		return nil
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
	s.sendPendingOutbox(ctx, pushOutboxBatch)
	return nil
}

// releaseReviewPayment は、仮押さえを取り消すか、確定済み（PayPay）なら全額を返金する。
func (s *Server) releaseReviewPayment(ctx context.Context, pay db.ReviewPayment) error {
	var notify bool
	err := s.refundTx(ctx, func(tx pgx.Tx) error {
		q := db.New(tx)
		var err error
		pay, err = q.GetReviewPaymentForRefund(ctx, pay.ID)
		if err != nil {
			return err
		}
		if pay.Status != "authorized" && pay.Status != "captured" {
			return nil
		}
		if pay.PaymentIntentID == nil {
			return errors.New("payment has no payment intent")
		}
		kind := ""
		var pendingRefundID *string
		if pay.Status == "authorized" {
			var canceled stripe.PaymentIntent
			canceled, err = s.stripe.CancelPaymentIntent(ctx, *pay.PaymentIntentID, "cancel-"+pay.ID.String())
			if canceled.Status == "canceled" && canceled.ID == *pay.PaymentIntentID {
				kind = "review_cancel"
			}
		} else {
			var refund stripe.Refund
			refund, err = s.stripe.CreateRefund(ctx, *pay.PaymentIntentID, int(pay.AmountYen), "refund-"+pay.ID.String())
			if refund.ID != "" && refund.Amount == int(pay.AmountYen) {
				switch refund.Status {
				case "succeeded":
					kind = "review_refund"
				case "pending", "requires_action":
					kind, pendingRefundID = "review_refund", &refund.ID
				}
			}
		}
		if err != nil {
			return fmt.Errorf("%w: %w", errStripe, err)
		}
		_, err = q.MarkReviewPaymentReleased(ctx, pay.ID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil || kind == "" || pay.PayerID == nil {
			return err
		}
		if kind == "review_cancel" && pay.RequestID != nil {
			// 期限切れ系の取り消しは期限切れ通知の1通に統合したため、別通知は出さない（本体 #573）。
			// 実際の返金（review_refund）は資金の移動があるため従来どおり知らせる。
			if rr, err := q.GetReviewRequest(ctx, *pay.RequestID); err == nil && rr.Status == "expired" {
				return nil
			}
		}
		notify = pendingRefundID == nil
		return q.EnqueueReviewRefundNotice(ctx, db.EnqueueReviewRefundNoticeParams{
			OperationID: pay.ID, PaymentID: pay.ID, Kind: kind, PendingStripeRefundID: pendingRefundID,
		})
	})
	if err == nil && notify {
		s.deliverRefundCompletionNotice(ctx, pay.ID)
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
	return s.tx(ctx, func(q *db.Queries) error {
		captured, err := q.MarkReviewPaymentCaptured(ctx, pay.ID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil || captured.AmountYen != reviewPriceYen {
			return err
		}
		return recordCoachingReferral(ctx, q, captured.PayerID, nil, &captured.ID, captured.ReferralCode)
	})
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
	if err := s.settleCapturedRefunds(ctx); err != nil {
		return 0, err
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
	}
	if _, err := s.q.DeleteUnpaidReviewRequests(ctx); err != nil {
		return released, err
	}
	if _, err := s.releaseExpiredTickets(ctx); err != nil {
		return released, err
	}
	return released, nil
}

// syncReviewPayment は、支払い画面から戻ったとき（成功・キャンセルとも）に呼ぶ。戻った URL ではなく、
// Stripe の支払い画面の今の状態を読み、完了していれば Webhook と同じ処理（completeReviewCheckout）を通す。
// Webhook を正とする設計は変えない。同じ処理は何度通しても1回分しか効かないので、Webhook が先でも後でもよい（本体 #569）。
func (s *Server) syncReviewPayment(w http.ResponseWriter, r *http.Request) {
	rr, ok := s.requesterReview(w, r)
	if !ok {
		return
	}
	checkout := ""
	if s.stripe.Configured() && rr.Status == "awaiting_payment" {
		pending, err := s.q.ListPendingReviewPayments(r.Context(), &rr.ID)
		if err != nil {
			MapDBError(w, err)
			return
		}
		for _, pay := range pending {
			sess, err := s.stripe.GetCheckoutSession(r.Context(), pay.CheckoutSessionID)
			if err != nil {
				writeCheckoutError(w, fmt.Errorf("%w: %w", errStripe, err))
				return
			}
			if checkout == "" || sess.Status == "complete" {
				checkout = sess.Status
			}
			if sess.ID == pay.CheckoutSessionID && sess.Status == "complete" {
				if err := s.completeReviewCheckout(r.Context(), sess); err != nil {
					MapDBError(w, err)
					return
				}
			}
		}
		if fresh, err := s.q.GetReviewRequest(r.Context(), rr.ID); err == nil {
			rr = fresh
		}
	}
	out := s.reviewRequestJSON(rr)
	if err := s.attachReviewPayment(r.Context(), out, rr.ID); err != nil {
		MapDBError(w, err)
		return
	}
	out["checkout_status"] = checkout
	JSON(w, http.StatusOK, out)
}
