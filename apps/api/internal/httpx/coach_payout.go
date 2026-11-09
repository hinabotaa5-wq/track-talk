package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/jst"
	"github.com/hinata/track-talk/apps/api/internal/push"
	"github.com/hinata/track-talk/apps/api/internal/stripe"

	"github.com/jackc/pgx/v5"
)

// コーチへの報酬。支払いは TrackTalk が受け取り、返却や期間の終わりごとに取り分を記録して、
// 月に1回、運営が前の月までの分をまとめて Stripe Connect でコーチのアカウントへ送金する。
const (
	// 添削の代金のうちコーチの取り分（%）。
	reviewSharePercent = 70
	// パーソナル指導の手数料（%）。
	personalFeePercent = 20
	// 1回に送金する最低額（円）。送金のたびに Stripe の費用（月200円と入金ごとの250円+0.25%）がかかるので、
	// 届かない分は翌月に繰り越す。繰り越しは1回までで、翌月は届かなくても送る。
	minPayoutYen = 3000
)

// payoutsReady は、コーチが報酬を受け取れる状態か。Stripe の鍵がないデモ版では、いつでも受け取れる扱いにする。
func (s *Server) payoutsReady(c db.CoachProfile) bool {
	return !s.stripe.Configured() || c.PayoutsEnabled
}

const payoutsNotReadyMessage = "報酬の受け取り口座を登録してから使えます（コーチの画面の「報酬」から登録できます）"

var errCoachStripeAttemptExpired = errors.New("unresolved Stripe account creation is outside the retry window")

// startCoachOnboarding は、コーチの受け取り用のアカウントを作り（まだなければ）、口座登録の画面の URL を返す。
func (s *Server) startCoachOnboarding(w http.ResponseWriter, r *http.Request) {
	coach, ok := s.approvedCoach(w, r)
	if !ok {
		return
	}
	if !s.stripe.Configured() {
		Error(w, http.StatusConflict, "payments_disabled", "いまは支払いのないデモ版なので、口座の登録はいりません")
		return
	}
	account := ""
	if coach.StripeAccountID != nil {
		account = *coach.StripeAccountID
	} else {
		var err error
		account, err = s.createCoachStripeAccount(r.Context(), coach.ProfileID)
		if errors.Is(err, errCoachStripeAttemptExpired) {
			slog.Warn("stripe account creation needs reconciliation", "profile_id", coach.ProfileID)
			Error(w, http.StatusConflict, "stripe_account_pending", "以前の口座登録の結果を確認する必要があります。運営にお問い合わせください")
			return
		}
		if err != nil {
			writeCheckoutError(w, err)
			return
		}
	}
	page := s.cfg.WebBaseURL + "/coach"
	link, err := s.stripe.CreateAccountLink(r.Context(), account, page+"?stripe=refresh", page+"?stripe=return")
	if err != nil {
		writeCheckoutError(w, fmt.Errorf("%w: %w", errStripe, err))
		return
	}
	JSON(w, http.StatusOK, map[string]any{"url": link})
}

func (s *Server) createCoachStripeAccount(ctx context.Context, profileID uuid.UUID) (string, error) {
	// Stripe に送る前に試行を確定する。デプロイ前の試行も同じキーで再送できる。
	err := s.q.StartCoachStripeAccountAttempt(ctx, db.StartCoachStripeAccountAttemptParams{
		ProfileID: profileID, IdempotencyKey: "account-" + profileID.String(),
	})
	if err != nil {
		return "", err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	q := s.q.WithTx(tx)
	// 口座の保存とキーの更新を直列化し、同時リクエストによる二重作成を防ぐ。
	attempt, err := q.LockCoachStripeAccountAttempt(ctx, profileID)
	if err != nil {
		return "", err
	}
	if attempt.StripeAccountID != nil {
		return *attempt.StripeAccountID, nil
	}
	if attempt.StartedAt == nil {
		// ロックを待つ間に別のリクエストが失敗し、次のキーに切り替えた。
		return "", fmt.Errorf("%w: Stripe account creation was rejected by a concurrent request", errStripe)
	}
	// Stripe がキーを削除できる24時間までに、HTTP の再送を含めて終わらせる。
	if time.Since(*attempt.StartedAt) >= 23*time.Hour+58*time.Minute {
		return "", errCoachStripeAttemptExpired
	}

	a, err := s.stripe.CreateAccount(ctx, map[string]string{"profile_id": profileID.String()}, attempt.IdempotencyKey)
	if err != nil {
		// Stripe が口座作成を明示的に拒否した場合だけ、次の試行に新しいキーを割り当てる。
		// 通信断・5xx・べき等キーの競合では、すでに作成された可能性がある。
		var apiErr *stripe.APIError
		if errors.As(err, &apiErr) && apiErr.Status == http.StatusBadRequest &&
			apiErr.Type == "invalid_request_error" && apiErr.Code != "idempotency_key_in_use" {
			if rotateErr := q.RotateCoachStripeAccountAttempt(ctx, db.RotateCoachStripeAccountAttemptParams{
				ProfileID: profileID, IdempotencyKey: "account-" + uuid.NewString(),
			}); rotateErr != nil {
				return "", rotateErr
			}
			if commitErr := tx.Commit(ctx); commitErr != nil {
				return "", commitErr
			}
		}
		return "", fmt.Errorf("%w: %w", errStripe, err)
	}
	if a.ID == "" {
		return "", fmt.Errorf("%w: missing Stripe account ID", errStripe)
	}
	coach, err := q.SetCoachStripeAccount(ctx, db.SetCoachStripeAccountParams{StripeAccountID: &a.ID, ProfileID: profileID})
	if err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return *coach.StripeAccountID, nil
}

// refreshPayoutsEnabled は、口座登録が済んだかを Stripe に聞いて記録する。
func (s *Server) refreshPayoutsEnabled(ctx context.Context, c db.CoachProfile) db.CoachProfile {
	if !s.stripe.Configured() || c.StripeAccountID == nil || c.PayoutsEnabled {
		return c
	}
	a, err := s.stripe.GetAccount(ctx, *c.StripeAccountID)
	if err != nil {
		slog.Warn("stripe get account", "err", err)
		return c
	}
	if !a.PayoutsEnabled {
		return c
	}
	updated, err := s.q.SetCoachPayoutsEnabled(ctx, db.SetCoachPayoutsEnabledParams{PayoutsEnabled: true, ProfileID: c.ProfileID})
	if err != nil {
		slog.Warn("set payouts enabled", "err", err)
		return c
	}
	return updated
}

func payoutJSON(p db.CoachPayout) map[string]any {
	return map[string]any{
		"id": p.ID, "month": p.Month.Format("2006-01"), "amount_yen": p.AmountYen,
		"status": p.Status, "paid_at": p.PaidAt,
	}
}

// coachEarnings は、コーチの画面の「報酬」。口座登録の状態、今月と未送金の報酬、送金の記録。
func (s *Server) coachEarnings(w http.ResponseWriter, r *http.Request) {
	profile, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	coach, err := s.q.GetCoachProfile(ctx, profile.ID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && coach.Status != "approved" && coach.Status != "suspended" && coach.Status != "ended") {
		Error(w, http.StatusForbidden, "forbidden", "承認されたコーチだけが使えます")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	if coach.Status == "approved" {
		coach = s.refreshPayoutsEnabled(ctx, coach)
	}
	date, yen, err := s.previewCoachPayout(ctx, coach)
	if err != nil {
		MapDBError(w, err)
		return
	}
	summary, err := s.q.GetCoachEarningsSummary(r.Context(), &coach.ProfileID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	payouts, err := s.q.ListCoachPayouts(r.Context(), &coach.ProfileID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	earnings, err := s.q.ListRecentCoachEarnings(r.Context(), &coach.ProfileID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	outPayouts := make([]map[string]any, 0, len(payouts))
	for _, p := range payouts {
		outPayouts = append(outPayouts, payoutJSON(p))
	}
	outEarnings := make([]map[string]any, 0, len(earnings))
	for _, e := range earnings {
		outEarnings = append(outEarnings, map[string]any{
			"id": e.ID, "source": e.Source, "gross_yen": e.GrossYen, "share_percent": e.SharePercent,
			"amount_yen": e.AmountYen, "paid": e.PayoutID != nil, "created_at": e.CreatedAt, "confirmed_at": e.ConfirmedAt,
		})
	}
	JSON(w, http.StatusOK, map[string]any{
		// payments が false のあいだは支払いなしのデモ版。
		"payments":         s.stripe.Configured(),
		"stripe_linked":    coach.StripeAccountID != nil,
		"payouts_enabled":  coach.PayoutsEnabled,
		"review_share":     reviewSharePercent,
		"personal_fee":     personalFeePercent,
		"min_payout_yen":   minPayoutYen,
		"this_month_yen":   summary.ThisMonthYen,
		"unpaid_yen":       summary.UnpaidYen,
		"next_payout_date": date,
		"next_payout_yen":  yen,
		"payouts":          outPayouts,
		"earnings":         outEarnings,
	})
}

// previewCoachPayout uses a consistent snapshot for the offset plan and payout query.
func (s *Server) previewCoachPayout(ctx context.Context, coach db.CoachProfile) (*string, *int32, error) {
	if !s.stripe.Configured() || coach.Status == "suspended" || !coach.PayoutsEnabled || coach.StripeAccountID == nil {
		return nil, nil, nil
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(context.Background())

	plan, err := planCoachRefundOffsets(ctx, tx, &coach.ProfileID, false)
	if err != nil {
		return nil, nil, err
	}
	offsets := make(map[string]int)
	for _, offset := range plan {
		offsets[offset.earningID.String()] += offset.amount
	}
	projected, err := json.Marshal(offsets)
	if err != nil {
		return nil, nil, err
	}
	rows, err := db.New(tx).ListPayableCoachEarnings(ctx, db.ListPayableCoachEarningsParams{
		CoachID: &coach.ProfileID, MinYen: minPayoutYen, IncludeDeferred: true, ProjectedOffsets: projected,
	})
	if err != nil || len(rows) == 0 {
		return nil, nil, err
	}
	row := rows[0]
	date := nextCoachPayoutDate(row.EvaluatedAt, row.PayableNow && !row.BatchAlreadyPaid)
	return &date, &row.AmountYen, nil
}

func nextCoachPayoutDate(now time.Time, payableThisMonth bool) string {
	now = now.In(jst.Zone)
	month := now.Month()
	if !payableThisMonth || now.Day() > stripe.PayoutMonthlyAnchor {
		month++
	}
	return time.Date(now.Year(), month, stripe.PayoutMonthlyAnchor, 0, 0, 0, 0, jst.Zone).Format(time.DateOnly)
}

// recordCoachEarnings は、返却した添削と、終わったパーソナル指導の報酬を記録する（tick から）。
func (s *Server) recordCoachEarnings(ctx context.Context) (int, error) {
	var n int
	err := s.refundTx(ctx, func(tx pgx.Tx) error { var err error; n, err = recordCoachEarningsWith(ctx, db.New(tx)); return err })
	return n, err
}

// recordCoachEarningsWith は recordCoachEarnings を q(トランザクションの中など)で行う。
func recordCoachEarningsWith(ctx context.Context, q *db.Queries) (int, error) {
	if err := q.LockCoachPayouts(ctx); err != nil {
		return 0, err
	}
	reviews, err := q.InsertReviewEarnings(ctx, reviewSharePercent)
	if err != nil {
		return 0, err
	}
	personal, err := q.InsertPersonalEarnings(ctx, personalFeePercent)
	if err != nil {
		return len(reviews), err
	}
	// 無料相談から成立した大会までプラン：期間中は月次の仮払い、終わったら日割りで確定して差額を精算する。
	advances, err := recordSupportAdvances(ctx, q, time.Now())
	if err != nil {
		return len(reviews) + len(personal), err
	}
	finals, err := finalizeSupportEarnings(ctx, q)
	if err != nil {
		return len(reviews) + len(personal) + advances, err
	}
	return len(reviews) + len(personal) + advances + finals, nil
}

type payoutRunResult struct {
	// 新しくまとめた送金と、送金できたもの・失敗したもの（失敗は次に押したときにやり直す）。
	Created int `json:"created"`
	Paid    int `json:"paid"`
	Failed  int `json:"failed"`
}

// runCoachPayouts は、前の月までの報酬をコーチごとにまとめ、まだ送っていない送金を Stripe で送る。
// 合計が minPayoutYen に届かないコーチの分は、1回だけ送金にまとめずに残しておく。
func (s *Server) runCoachPayouts(ctx context.Context) (payoutRunResult, error) {
	var out payoutRunResult
	// 送金する前に、記録し損ねた報酬を拾う。
	if _, err := s.recordCoachEarnings(ctx); err != nil {
		return out, err
	}
	err := s.refundTx(ctx, func(tx pgx.Tx) error {
		q := db.New(tx)
		if err := applyCoachRefundOffsets(ctx, tx); err != nil {
			return err
		}
		rows, err := q.ListPayableCoachEarnings(ctx, db.ListPayableCoachEarningsParams{MinYen: minPayoutYen})
		if err != nil {
			return err
		}
		for _, row := range rows {
			if row.AmountYen <= 0 {
				continue
			}
			p, err := q.UpsertCoachPayout(ctx, db.UpsertCoachPayoutParams{ID: uuid.New(), CoachID: &row.CoachID, AmountYen: row.AmountYen})
			if errors.Is(err, pgx.ErrNoRows) {
				// その月の送金はもう済んでいる。残りは次の月にまとめる。
				continue
			}
			if err != nil {
				return err
			}
			if _, err := q.AttachEarningsToPayout(ctx, db.AttachEarningsToPayoutParams{PayoutID: &p.ID, CoachID: &row.CoachID}); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE coach_refund_applications a SET payout_id=$1 FROM coach_earnings e WHERE e.id=a.earning_id AND e.payout_id=$1 AND a.payout_id IS NULL AND NOT EXISTS(SELECT 1 FROM coach_refund_application_reversals v WHERE v.application_id=a.id)`, p.ID); err != nil {
				return err
			}
			out.Created++
		}
		return nil
	})
	if err != nil {
		return out, err
	}
	var notices []db.ListUnpaidCoachPayoutsRow
	err = s.refundTx(ctx, func(tx pgx.Tx) error {
		q := db.New(tx)
		unpaid, err := q.ListUnpaidCoachPayouts(ctx)
		if err != nil {
			return err
		}
		for _, p := range unpaid {
			group := "payout-" + p.ID.String()
			// 前の送金が Stripe 側では成功していた（通信エラーや DB の更新失敗で failed/pending のまま残った）ら、
			// 送り直さずにその送金で済ませる。Idempotency-Key は約24時間で切れるので、それだけでは二重送金を防げない。
			sent, err := s.stripe.FindTransfers(ctx, group)
			if err != nil {
				slog.Warn("stripe find transfers", "payout", p.ID, "err", err)
				msg := err.Error()
				if dbErr := q.MarkCoachPayoutFailed(ctx, db.MarkCoachPayoutFailedParams{ID: p.ID, Failure: &msg}); dbErr != nil {
					return dbErr
				}
				out.Failed++
				continue
			}
			if len(sent) > 0 {
				if sent[0].Amount != int(p.AmountYen) {
					// 金額の違う送金がすでにある。差額をどうするかは運営が確かめる。
					msg := fmt.Sprintf("金額の違う送金 %s（%d円）がすでにあります", sent[0].ID, sent[0].Amount)
					if dbErr := q.MarkCoachPayoutFailed(ctx, db.MarkCoachPayoutFailedParams{ID: p.ID, Failure: &msg}); dbErr != nil {
						return dbErr
					}
					out.Failed++
					continue
				}
				if _, err := q.MarkCoachPayoutPaid(ctx, db.MarkCoachPayoutPaidParams{ID: p.ID, StripeTransferID: &sent[0].ID}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
					return err
				}
				out.Paid++
				continue
			}
			t, err := s.stripe.CreateTransfer(ctx, p.StripeAccountID, int(p.AmountYen), group,
				map[string]string{"payout_id": p.ID.String(), "month": p.Month.Format("2006-01")},
				fmt.Sprintf("payout-%s-%d", p.ID, p.AmountYen))
			if err != nil {
				slog.Warn("stripe transfer", "payout", p.ID, "err", err)
				msg := err.Error()
				if dbErr := q.MarkCoachPayoutFailed(ctx, db.MarkCoachPayoutFailedParams{ID: p.ID, Failure: &msg}); dbErr != nil {
					return dbErr
				}
				out.Failed++
				continue
			}
			if _, err := q.MarkCoachPayoutPaid(ctx, db.MarkCoachPayoutPaidParams{ID: p.ID, StripeTransferID: &t.ID}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			out.Paid++
			notices = append(notices, p)
		}
		return nil
	})
	if err != nil {
		return out, err
	}
	for _, p := range notices {
		if p.CoachID != nil {
			s.notify(ctx, *p.CoachID, notifyImportant, push.Message{Title: "報酬を送金しました", Body: fmt.Sprintf("%s分までの報酬 %d円を Stripe のアカウントに送金しました", p.Month.Format("2006年1月"), p.AmountYen), URL: "/coach", Tag: "payout-" + p.ID.String()}, 24*time.Hour)
		}
	}
	return out, nil
}

// adminCoachPayouts は、運営の送金の画面。コーチごとの口座登録と未送金の報酬、これまでの送金。
func (s *Server) adminCoachPayouts(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	coaches, err := s.q.AdminListPayoutCoaches(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	payouts, err := s.q.AdminListCoachPayouts(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	if coaches == nil {
		coaches = []db.AdminListPayoutCoachesRow{}
	}
	outPayouts := make([]map[string]any, 0, len(payouts))
	for _, p := range payouts {
		outPayouts = append(outPayouts, map[string]any{
			"id": p.ID, "coach_id": p.CoachID, "nickname": p.Nickname, "month": p.Month.Format("2006-01"),
			"amount_yen": p.AmountYen, "status": p.Status, "failure": p.Failure, "paid_at": p.PaidAt, "created_at": p.CreatedAt,
		})
	}
	JSON(w, http.StatusOK, map[string]any{"payments": s.stripe.Configured(), "coaches": coaches, "payouts": outPayouts})
}

// adminRunCoachPayouts は、運営が月に1回押して、前の月までの報酬を送金する。失敗した送金もやり直す。
func (s *Server) adminRunCoachPayouts(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if !s.stripe.Configured() {
		Error(w, http.StatusConflict, "payments_disabled", "いまは支払いのないデモ版なので、送金はありません")
		return
	}
	out, err := s.runCoachPayouts(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, out)
}
