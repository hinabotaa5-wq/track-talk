package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/push"
	"github.com/hinata/track-talk/apps/api/internal/stripe"
	"github.com/jackc/pgx/v5"
)

// パーソナル指導の支払い（Stripe Checkout）。TrackTalk が売り手として受け取る。
// Stripe の鍵が設定されているときだけ使い、設定されていなければ支払いなしのデモ版として動く。
// コーチが承諾したら72時間以内に払うと、その時から期間が始まる。18歳未満は保護者が同意して払う。
// 始まってから72時間コーチが1通も書かなければ全額を自動で返金する。運営は全額か残りの日数分を返金できる。

// Stripe の支払い画面は、作ってから30分〜24時間で閉じられる。
const (
	checkoutMinLife = 31 * time.Minute
	checkoutMaxLife = 24 * time.Hour
)

// checkoutPaymentMethods は、支払い方法をコードで指定する支払い画面に出す方法。カード（Apple Pay・Google Pay を含む）と
// PayPay だけにする。コンビニ払い・銀行振込は出さない。PayPay は Stripe のダッシュボードで有効にしてから使う。
var checkoutPaymentMethods = []string{"card", "paypay"}

// errStripe は、Stripe とのやりとりがうまくいかなかったこと（つながらない、断られた）。
var errStripe = errors.New("stripe request failed")

func paymentJSON(p db.CoachingPayment) map[string]any {
	return map[string]any{
		"status":       p.Status,
		"amount_yen":   p.AmountYen,
		"refunded_yen": p.RefundedYen,
		"paid_at":      p.PaidAt,
		"refunded_at":  p.RefundedAt,
	}
}

// attachPayment は、契約の JSON に支払いを足す。支払いがなければ null。
func (s *Server) attachPayment(ctx context.Context, c map[string]any, id uuid.UUID) error {
	c["payment"] = nil
	p, err := s.q.GetLatestCoachingPayment(ctx, &id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	c["payment"] = paymentJSON(p)
	return nil
}

// checkoutURL は、支払い画面の URL を返す。まだ使える画面があればそれを、なければ新しく作る。
func (s *Server) checkoutURL(ctx context.Context, ct db.CoachingContract, payer, successURL, cancelURL string) (string, error) {
	if !ct.Minor {
		return s.checkoutURLWith(ctx, s.q, ct, payer, successURL, cancelURL)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)
	if err := q.LockReviewRequester(ctx, ct.RequesterID); err != nil {
		return "", err
	}
	msg, err := s.coachingMonthlyBlock(ctx, q, ct.RequesterID, uuid.Nil, ct.ID, uuid.Nil, ct.PriceYen)
	if err != nil {
		return "", err
	}
	if msg != "" {
		return "", &ticketRuleError{"monthly_limit", msg}
	}
	url, err := s.checkoutURLWith(ctx, q, ct, payer, successURL, cancelURL)
	if err != nil {
		return "", err
	}
	return url, tx.Commit(ctx)
}

func (s *Server) checkoutURLWith(ctx context.Context, q *db.Queries, ct db.CoachingContract, payer, successURL, cancelURL string) (string, error) {
	p, err := q.GetReusableCoachingPayment(ctx, db.GetReusableCoachingPaymentParams{ContractID: &ct.ID, Payer: payer})
	if err == nil {
		return p.CheckoutUrl, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	now := time.Now()
	if ct.CompetitionDate != nil && ct.CompetitionDate.Add(24*time.Hour-9*time.Hour).Before(now.Add(checkoutMinLife)) {
		return "", &ticketRuleError{"support_closed", "大会までの支払い期限を過ぎました"}
	}
	expires := now.Add(checkoutMaxLife)
	if ct.CompetitionDate != nil {
		end := ct.CompetitionDate.Add(24*time.Hour - 9*time.Hour)
		if end.Before(expires) {
			expires = end
		}
	}

	if ct.StartBy != nil && ct.StartBy.Before(expires) {
		expires = *ct.StartBy
	}
	if expires.Before(now.Add(checkoutMinLife)) {
		expires = now.Add(checkoutMinLife)
	}
	id := uuid.New()
	sess, err := s.stripe.CreateCheckoutSession(ctx, stripe.CheckoutParams{
		Name:       fmt.Sprintf("パーソナル指導「%s」（%d日）", ct.PlanTitle, ct.DurationDays),
		AmountYen:  int(ct.PriceYen),
		SuccessURL: successURL,
		CancelURL:  cancelURL,
		Reference:  id.String(),
		Metadata:   map[string]string{"contract_id": ct.ID.String(), "payment_id": id.String()},
		ExpiresAt:  expires,
		// 同じ支払いの行で送り直しても、支払い画面は1つだけ。
		IdempotencyKey: "checkout-" + id.String(),
	})
	if err != nil {
		return "", fmt.Errorf("%w: %w", errStripe, err)
	}
	if sess.ExpiresAt > 0 {
		expires = time.Unix(sess.ExpiresAt, 0)
	}
	if _, err := q.CreateCoachingPayment(ctx, db.CreateCoachingPaymentParams{
		ID: id, ContractID: &ct.ID, Payer: payer, AmountYen: ct.PriceYen,
		CheckoutSessionID: sess.ID, CheckoutUrl: sess.URL, SessionExpiresAt: expires,
	}); err != nil {
		return "", err
	}
	return sess.URL, nil
}

// writeCheckoutError は、支払い画面を作れなかったときの返事。
func writeCheckoutError(w http.ResponseWriter, err error) {
	if writeTicketRuleError(w, err) {
		return
	}
	if errors.Is(err, errStripe) {
		slog.Warn("stripe checkout", "err", err)
		Error(w, http.StatusBadGateway, "payment_unavailable", "支払い画面を開けませんでした。少したってからもう一度お試しください")
		return
	}
	MapDBError(w, err)
}

// checkoutOpen は、支払いを待っている契約か。
func checkoutOpen(w http.ResponseWriter, ct db.CoachingContract) bool {
	if !ct.PaymentRequired {
		Error(w, http.StatusConflict, "conflict", "この申し込みにはお支払いがありません")
		return false
	}
	if ct.Status != "accepted" || ct.StartBy == nil || !ct.StartBy.After(time.Now()) {
		Error(w, http.StatusConflict, "conflict", "この申し込みはお支払いを待っていません")
		return false
	}
	return true
}

// checkoutCoachingContract は、大人の利用者が支払い画面を開く。
func (s *Server) checkoutCoachingContract(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	ct, err := s.q.GetCoachingContract(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && ct.RequesterID != me.ID) {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	if ct.Minor {
		Error(w, http.StatusConflict, "guardian_pays", "18歳未満の人の申し込みは、保護者が同意してお支払いします")
		return
	}
	if !checkoutOpen(w, ct) {
		return
	}
	page := s.cfg.WebBaseURL + "/coaching/personal/" + ct.ID.String()
	url, err := s.checkoutURL(r.Context(), ct, "requester", page+"?paid=1", page)
	if err != nil {
		writeCheckoutError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"checkout_url": url})
}

// guardianCheckoutURL は、保護者の支払い画面。保護者のリンクは Stripe に渡さないよう、戻り先は共通のページにする。
func (s *Server) guardianCheckoutURL(ctx context.Context, ct db.CoachingContract) (string, error) {
	page := s.cfg.WebBaseURL + "/guardian/paid"
	return s.checkoutURL(ctx, ct, "guardian", page, page+"?cancelled=1")
}

// checkoutGuardianContract は、同意を済ませた保護者が支払い画面を開き直す。
func (s *Server) checkoutGuardianContract(w http.ResponseWriter, r *http.Request) {
	ct, ok := s.guardianContract(w, r)
	if !ok {
		return
	}
	if !checkoutOpen(w, ct) {
		return
	}
	consented, err := s.q.HasGuardianContractConsent(r.Context(), ct.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if !consented {
		Error(w, http.StatusConflict, "conflict", "先にこの申し込みに同意してください")
		return
	}
	url, err := s.guardianCheckoutURL(r.Context(), ct)
	if err != nil {
		writeCheckoutError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"checkout_url": url})
}

// stripeWebhook は、Stripe からの知らせを受ける。署名が正しいものだけを読む。
// 2xx を返さないと Stripe は送り直すので、こちらの都合で読めないもの以外は 200 を返す。
func (s *Server) stripeWebhook(w http.ResponseWriter, r *http.Request) {
	secrets := s.webhookSecrets()
	if !s.stripe.Configured() || len(secrets) == 0 {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return
	}
	payload, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "読めませんでした")
		return
	}
	ev, err := verifyWebhookAny(payload, r.Header.Get("Stripe-Signature"), secrets)
	if err != nil {
		Error(w, http.StatusBadRequest, "bad_signature", "署名が正しくありません")
		return
	}
	if !s.cfg.CoachingEnabled {
		// 作成済みの決済への通知だけを処理する。口座更新は専用受信口に分ける。
		switch ev.Type {
		case "checkout.session.completed", "checkout.session.async_payment_succeeded", "charge.dispute.created":
		default:
			Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
			return
		}
	}
	switch ev.Type {
	case "account.updated":
		if err := s.syncCoachAccount(r.Context(), ev); err != nil {
			slog.Error("stripe webhook", "event", ev.ID, "err", err)
			Error(w, http.StatusInternalServerError, "internal_error", "処理できませんでした")
			return
		}
	case "charge.dispute.created":
		s.alertDispute(r.Context(), ev)
	case "checkout.session.completed", "checkout.session.async_payment_succeeded":
		var sess stripe.CheckoutSession
		if err := json.Unmarshal(ev.Data.Object, &sess); err != nil {
			Error(w, http.StatusBadRequest, "validation_error", "読めませんでした")
			return
		}
		complete := s.completeCoachingCheckout
		switch sess.Metadata["kind"] {
		case "review":
			complete = s.completeReviewCheckout
		case "consultation":
			complete = s.completeConsultationCheckout
		case "tickets":
			complete = s.completeTicketCheckout
		case "", "personal":
		default:
			Error(w, 400, "unknown_payment_kind", "支払いの種類が不正です")
			return
		}
		if err := complete(r.Context(), sess); err != nil {
			slog.Error("stripe webhook", "event", ev.ID, "err", err)
			Error(w, http.StatusInternalServerError, "internal_error", "処理できませんでした")
			return
		}
	}
	JSON(w, http.StatusOK, map[string]any{"received": true})
}

// webhookSecrets は、署名を確かめる鍵。Stripe は platform の知らせと Connect（コーチのアカウント）の知らせで
// 送り先ごとに鍵を分ける。公開前の通常決済通知は通常用の鍵だけで検証する。
// 公開済み環境の既存 Connect 登録との互換性のため、公開中は従来どおり両方を受け付ける。
func (s *Server) webhookSecrets() []string {
	var out []string
	keys := []string{s.cfg.StripeWebhookSecret}
	if s.cfg.CoachingEnabled {
		keys = append(keys, s.cfg.StripeConnectWebhookSecret)
	}
	for _, k := range keys {
		if k != "" {
			out = append(out, k)
		}
	}
	return out
}

func verifyWebhookAny(payload []byte, header string, secrets []string) (stripe.Event, error) {
	for _, k := range secrets {
		if ev, err := stripe.VerifyWebhook(payload, header, k, time.Now()); err == nil {
			return ev, nil
		}
	}
	return stripe.Event{}, stripe.ErrBadSignature
}

// syncCoachAccount は、コーチの Stripe アカウントの状態が変わった知らせを、受け取れるかどうかに反映する。
// 知らないアカウントの知らせは読み飛ばす。
func (s *Server) syncCoachAccount(ctx context.Context, ev stripe.Event) error {
	var a stripe.Account
	if err := json.Unmarshal(ev.Data.Object, &a); err != nil || a.ID == "" {
		return nil
	}
	// 知らないアカウントの知らせは読み飛ばす。
	known, err := s.q.CoachStripeAccountExists(ctx, a.ID)
	if err != nil || !known {
		return err
	}
	// 同じ口座の通知が並行して届いたときも、読み直しと保存を一組で直列化する。
	// 読み直しの後だけロックすると、先に読んだ古い状態で上書きしてしまう。
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var coachID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT profile_id FROM coach_profiles WHERE stripe_account_id = $1 FOR UPDATE`, a.ID).Scan(&coachID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	// 知らせは順番どおりに届くとは限らないので、中身ではなく、いまのアカウントを読み直して決める。
	// 読めなければ失敗を返し、Stripe に送り直してもらう。
	current, err := s.stripe.GetAccount(ctx, a.ID)
	if err != nil {
		return err
	}
	a.PayoutsEnabled = current.PayoutsEnabled
	_, err = s.q.WithTx(tx).SetCoachPayoutsEnabledByAccount(ctx, db.SetCoachPayoutsEnabledByAccountParams{PayoutsEnabled: a.PayoutsEnabled, StripeAccountID: &a.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// alertDispute は、チャージバック（利用者がカード会社に異議を申し立てた）を運営に知らせる。中身は載せない。
// 対応は Stripe のダッシュボードで期限までに行うので、読めない知らせでも落とさず記録だけ残す。
func (s *Server) alertDispute(ctx context.Context, ev stripe.Event) {
	var d struct {
		ID            string `json:"id"`
		Amount        int    `json:"amount"`
		Reason        string `json:"reason"`
		PaymentIntent string `json:"payment_intent"`
	}
	_ = json.Unmarshal(ev.Data.Object, &d)
	slog.Error("stripe dispute", "dispute", d.ID, "amount", d.Amount, "reason", d.Reason, "payment_intent", d.PaymentIntent)
	ids, err := s.q.ListAdminProfileIDs(ctx)
	if err != nil {
		slog.Warn("dispute notify", "err", err)
		return
	}
	for _, id := range ids {
		s.notify(ctx, id, notifyImportant, push.Message{
			Title: "支払いに異議が申し立てられました",
			Body:  "Stripe のダッシュボードで、期限までに対応してください",
			URL:   "/admin",
			Tag:   "stripe-dispute-" + d.ID,
		}, 7*24*time.Hour)
	}
}

// completeCoachingCheckout は、支払いが済んだ契約の期間を始める。始められないもの（期限切れ、二重の支払い）は返金する。
func (s *Server) completeCoachingCheckout(ctx context.Context, sess stripe.CheckoutSession) error {
	if sess.PaymentStatus != "paid" || sess.PaymentIntent == "" {
		// 後払いの支払い方法は、async_payment_succeeded が届いてから。
		return nil
	}
	var pay db.CoachingPayment
	var started *db.CoachingContract
	err := s.tx(ctx, func(q *db.Queries) error {
		var err error
		pay, err = q.MarkCoachingPaymentPaid(ctx, db.MarkCoachingPaymentPaidParams{
			CheckoutSessionID: sess.ID, PaymentIntentID: &sess.PaymentIntent,
		})
		if err != nil {
			return err
		}
		if sess.AmountTotal != int(pay.AmountYen) {
			slog.Warn("stripe amount mismatch", "payment", pay.ID, "want", pay.AmountYen, "got", sess.AmountTotal)
		}
		if pay.ContractID == nil {
			return nil
		}
		snapshot, e := q.GetCoachingContract(ctx, *pay.ContractID)
		if e != nil {
			return e
		}
		if snapshot.FulfillmentVersion == 1 && (sess.AmountTotal != int(pay.AmountYen) || sess.Currency != "jpy" || sess.ClientReferenceID != pay.ID.String()) {
			return errors.New("support checkout mismatch")
		}
		ct, err := q.StartPaidCoachingContract(ctx, *pay.ContractID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		started = &ct
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// 知らない支払い画面か、もう受け取った知らせ。
		return nil
	}
	if err != nil {
		return err
	}
	if started == nil {
		if err := s.refundUnstarted(ctx, pay); err != nil {
			// 返金できなかったものは tick が拾い直す。
			slog.Error("refund unstarted payment", "payment", pay.ID, "err", err)
		}
		return nil
	}
	msg := push.Message{
		Title: "パーソナル指導が始まりました",
		Body:  "お支払いが済み、「" + started.PlanTitle + "」の期間が始まりました",
		URL:   "/coaching/personal/" + started.ID.String(),
		Tag:   "contract-" + started.ID.String(),
	}
	s.notify(ctx, started.RequesterID, notifyCoaching, msg, 24*time.Hour)
	s.notify(ctx, started.CoachID, notifyCoaching, msg, 24*time.Hour)
	return nil
}

// refundPayment は、支払いを Stripe で返金して記録する。1つの支払いの返金は1回だけ。
// closeContract なら、同じトランザクションで指導を閉じ、まだ送金に含めていない報酬を消す。返金だけ記録されて
// 指導が期間中のまま残る(途中で落ちた)と、返金済みの支払いは二度と拾われず、無料で続いてしまうので。
func (s *Server) refundPayment(ctx context.Context, pay db.CoachingPayment, amount int32, reason string, closeContract bool) (db.CoachingPayment, error) {
	if pay.PaymentIntentID == nil {
		return pay, errors.New("payment has no payment intent")
	}
	// 同じ支払いの返金が重なっても Stripe に2回送らないよう、支払いの行をロックしたまま送って記録する。
	// まだ返金していないことは、ロックしてから確かめ直す（返し終えていたら ErrNoRows）。
	refunded := pay
	err := s.tx(ctx, func(q *db.Queries) error {
		if _, err := q.LockPaidCoachingPayment(ctx, pay.ID); err != nil {
			return err
		}
		// キーは支払いだけで決める。残りの日数分の額は時間で変わるので、額を入れると、Stripe で返金できたのに
		// 記録の前に落ちたあとで押し直したとき、別の返金としてもう一度返してしまう。同じキーで額が違えば Stripe が断る。
		if _, err := s.stripe.CreateRefund(ctx, *pay.PaymentIntentID, int(amount), "refund-"+pay.ID.String()); err != nil {
			return fmt.Errorf("%w: %w", errStripe, err)
		}
		var err error
		refunded, err = q.RecordCoachingRefund(ctx, db.RecordCoachingRefundParams{ID: pay.ID, RefundedYen: amount, RefundReason: reason})
		if err != nil || !closeContract || pay.ContractID == nil {
			return err
		}
		if err := q.DeleteUnpaidContractEarning(ctx, pay.ContractID); err != nil {
			return err
		}
		if _, err := q.RefundCoachingContract(ctx, *pay.ContractID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		return nil
	})
	if err != nil {
		return pay, err
	}
	return refunded, nil
}

// refundUnstarted は、期間が始まらなかった支払いを全額返金して、払った人の側に知らせる。
func (s *Server) refundUnstarted(ctx context.Context, pay db.CoachingPayment) error {
	if _, err := s.refundPayment(ctx, pay, pay.AmountYen, "not_started", false); err != nil {
		return err
	}
	if pay.ContractID == nil {
		return nil
	}
	ct, err := s.q.GetCoachingContract(ctx, *pay.ContractID)
	if err != nil {
		return nil
	}
	s.notify(ctx, ct.RequesterID, notifyImportant, push.Message{
		Title: "お支払いを返金しました",
		Body:  "「" + ct.PlanTitle + "」は期限を過ぎていたなどの理由で始められなかったため、全額を返金しました",
		URL:   "/coaching/personal/" + ct.ID.String(),
		Tag:   "contract-" + ct.ID.String(),
	}, 24*time.Hour)
	return nil
}

// touchCoachingPayment は、Stripe で返金できなかった支払いを次の tick で後ろに回す。
// 断られ続ける支払いが50件たまっても、ほかの支払いが返金されないままにならないようにする。
func (s *Server) touchCoachingPayment(ctx context.Context, id uuid.UUID) {
	if err := s.q.TouchCoachingPayment(ctx, id); err != nil {
		slog.Error("touch coaching payment", "payment", id, "err", err)
	}
}

// refundCoachingPayments は tick から呼ぶ。始まってから72時間コーチが書いていないものを全額返金して閉じ、
// 始まらなかったのに返金し損ねた支払いも返金する。
func (s *Server) refundCoachingPayments(ctx context.Context) (int, error) {
	if !s.stripe.Configured() {
		return 0, nil
	}
	n := 0
	silent, err := s.q.ListSilentPaidContracts(ctx)
	if err != nil {
		return 0, err
	}
	for _, ct := range silent {
		pay, err := s.q.GetPaidCoachingPayment(ctx, &ct.ID)
		if err != nil {
			continue
		}
		if _, err := s.refundPayment(ctx, pay, pay.AmountYen, "no_coach_reply", true); err != nil {
			slog.Error("refund silent contract", "contract", ct.ID, "err", err)
			s.touchCoachingPayment(ctx, pay.ID)
			continue
		}
		n++
		s.notify(ctx, ct.RequesterID, notifyImportant, push.Message{
			Title: "パーソナル指導を返金しました",
			Body:  "始まってから72時間、コーチから書き込みがなかったため、全額を返金して終了しました",
			URL:   "/coaching/personal/" + ct.ID.String(),
			Tag:   "contract-" + ct.ID.String(),
		}, 24*time.Hour)
		s.notify(ctx, ct.CoachID, notifyImportant, push.Message{
			Title: "パーソナル指導が終了しました",
			Body:  "「" + ct.PlanTitle + "」は始まってから72時間書き込みがなかったため、全額を返金して終了しました",
			URL:   "/coach",
			Tag:   "contract-" + ct.ID.String(),
		}, 24*time.Hour)
	}
	unstarted, err := s.q.ListUnstartedPaidPayments(ctx)
	if err != nil {
		return n, err
	}
	for _, pay := range unstarted {
		if err := s.refundUnstarted(ctx, pay); err != nil {
			slog.Error("refund unstarted payment", "payment", pay.ID, "err", err)
			s.touchCoachingPayment(ctx, pay.ID)
			continue
		}
		n++
	}
	return n, nil
}

// remainingRefund は、残りの日数分の返金額。止めている間は、止めた時点までで数える。
func remainingRefund(ct db.CoachingContract, pay db.CoachingPayment, now time.Time) int32 {
	if ct.StartsAt == nil || ct.EndsAt == nil {
		return 0
	}
	at := now
	if ct.SuspendedAt != nil {
		at = *ct.SuspendedAt
	}
	left := ct.EndsAt.Sub(at)
	total := time.Duration(ct.DurationDays) * 24 * time.Hour
	if left <= 0 || total <= 0 {
		return 0
	}
	if left > total {
		left = total
	}
	// ナノ秒のままかけると桁があふれるので秒で数える。円未満は切り捨て。
	return int32(int64(pay.AmountYen) * int64(left/time.Second) / int64(total/time.Second))
}

// adminRefundCoachingContract は、運営が返金する。kind は full（全額）か remaining（残りの日数分）。
// 期間中のものは返金と一緒に終了する。
func (s *Server) adminRefundCoachingContract(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		Kind string `json:"kind"`
	}
	if err := DecodeJSON(r, &req); err != nil || (req.Kind != "full" && req.Kind != "remaining") {
		Error(w, http.StatusBadRequest, "validation_error", "返金の種類が不正です")
		return
	}
	ct, err := s.q.GetCoachingContract(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	if ct.FulfillmentVersion == 1 {
		s.adminRefundSupport(w, r, ct.ID, req.Kind)
		return
	}
	pay, err := s.q.GetPaidCoachingPayment(r.Context(), &ct.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusConflict, "conflict", "返金できる支払いがありません")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	amount, reason := pay.AmountYen, "admin_full"
	// 期間が終わった指導は、tick がコーチの報酬をもう記録している。送金に含めたあとは取り戻せないので、全額の返金はさせない。
	earning, err := s.q.GetContractEarning(r.Context(), &ct.ID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		MapDBError(w, err)
		return
	}
	if err == nil && earning.PayoutID != nil {
		Error(w, http.StatusConflict, "conflict", "コーチへの報酬を送金に含めたあとなので、ここからは返金できません。Stripe のダッシュボードで対応してください")
		return
	}
	if req.Kind == "remaining" {
		if ct.Status != "active" && ct.Status != "suspended" {
			Error(w, http.StatusConflict, "conflict", "残りの日数分は、期間中のものだけ返金できます")
			return
		}
		amount, reason = remainingRefund(ct, pay, time.Now()), "admin_remaining"
		if amount <= 0 {
			Error(w, http.StatusConflict, "conflict", "残りの日数がありません")
			return
		}
	}
	pay, err = s.refundPayment(r.Context(), pay, amount, reason, true)
	if err != nil {
		if errors.Is(err, errStripe) {
			slog.Warn("stripe refund", "err", err)
			Error(w, http.StatusBadGateway, "payment_unavailable", "Stripe で返金できませんでした。Stripe のダッシュボードで確かめてください")
			return
		}
		MapDBError(w, err)
		return
	}
	// 返金した分のコーチの報酬は refundPayment が外した。まだ記録していなければ、tick が返金後の額で記録する。
	if ended, err := s.q.GetCoachingContract(r.Context(), ct.ID); err == nil {
		ct = ended
	}
	msg := push.Message{
		Title: "パーソナル指導を返金しました",
		Body:  "「" + ct.PlanTitle + "」の " + strconv.Itoa(int(amount)) + "円を返金しました",
		URL:   "/coaching/personal/" + ct.ID.String(),
		Tag:   "contract-" + ct.ID.String(),
	}
	s.notify(r.Context(), ct.RequesterID, notifyImportant, msg, 24*time.Hour)
	msg.URL = "/coach"
	s.notify(r.Context(), ct.CoachID, notifyImportant, msg, 24*time.Hour)
	JSON(w, http.StatusOK, map[string]any{"contract": s.contractJSON(ct), "payment": paymentJSON(pay)})
}
