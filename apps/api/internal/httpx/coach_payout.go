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
	"github.com/jackc/pgx/v5"
)

// コーチへの報酬。支払いは TrackTalk が受け取り、返却や期間の終わりごとに取り分を記録して、
// 月に1回、運営が前の月までの分をまとめて Stripe Connect でコーチのアカウントへ送金する。
const (
	// 添削の代金のうちコーチの取り分（%）。
	reviewSharePercent = 70
	// パーソナル指導の手数料（%）。最初の数人のコーチは初年10%。
	personalFeePercent        = 20
	personalReducedFeePercent = 10
)

// payoutsReady は、コーチが報酬を受け取れる状態か。Stripe の鍵がないデモ版では、いつでも受け取れる扱いにする。
func (s *Server) payoutsReady(c db.CoachProfile) bool {
	return !s.stripe.Configured() || c.PayoutsEnabled
}

const payoutsNotReadyMessage = "報酬の受け取り口座を登録してから使えます（コーチの画面の「報酬」から登録できます）"

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
		// 2回押しても、Stripe は同じアカウントを返す。
		a, err := s.stripe.CreateAccount(r.Context(), map[string]string{"profile_id": coach.ProfileID.String()}, "account-"+coach.ProfileID.String())
		if err != nil {
			writeCheckoutError(w, fmt.Errorf("%w: %w", errStripe, err))
			return
		}
		c, err := s.q.SetCoachStripeAccount(r.Context(), db.SetCoachStripeAccountParams{StripeAccountID: &a.ID, ProfileID: coach.ProfileID})
		if err != nil {
			MapDBError(w, err)
			return
		}
		account = *c.StripeAccountID
	}
	page := s.cfg.WebBaseURL + "/coach"
	link, err := s.stripe.CreateAccountLink(r.Context(), account, page+"?stripe=refresh", page+"?stripe=return")
	if err != nil {
		writeCheckoutError(w, fmt.Errorf("%w: %w", errStripe, err))
		return
	}
	JSON(w, http.StatusOK, map[string]any{"url": link})
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
	coach, ok := s.approvedCoach(w, r)
	if !ok {
		return
	}
	coach = s.refreshPayoutsEnabled(r.Context(), coach)
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
			"amount_yen": e.AmountYen, "paid": e.PayoutID != nil, "created_at": e.CreatedAt,
		})
	}
	JSON(w, http.StatusOK, map[string]any{
		// payments が false のあいだは支払いなしのデモ版。
		"payments":          s.stripe.Configured(),
		"stripe_linked":     coach.StripeAccountID != nil,
		"payouts_enabled":   coach.PayoutsEnabled,
		"reduced_fee_until": coach.ReducedFeeUntil,
		"review_share":      reviewSharePercent,
		"personal_fee":      personalFeePercent,
		"this_month_yen":    summary.ThisMonthYen,
		"unpaid_yen":        summary.UnpaidYen,
		"payouts":           outPayouts,
		"earnings":          outEarnings,
	})
}

// recordCoachEarnings は、返却した添削と、終わったパーソナル指導の報酬を記録する（tick から）。
func (s *Server) recordCoachEarnings(ctx context.Context) (int, error) {
	return recordCoachEarningsWith(ctx, s.q)
}

// recordCoachEarningsWith は recordCoachEarnings を q(トランザクションの中など)で行う。
func recordCoachEarningsWith(ctx context.Context, q *db.Queries) (int, error) {
	reviews, err := q.InsertReviewEarnings(ctx, reviewSharePercent)
	if err != nil {
		return 0, err
	}
	personal, err := q.InsertPersonalEarnings(ctx, db.InsertPersonalEarningsParams{
		FeePercent: personalFeePercent, ReducedFeePercent: personalReducedFeePercent,
	})
	if err != nil {
		return len(reviews), err
	}
	return len(reviews) + len(personal), nil
}

type payoutRunResult struct {
	// 新しくまとめた送金と、送金できたもの・失敗したもの（失敗は次に押したときにやり直す）。
	Created int `json:"created"`
	Paid    int `json:"paid"`
	Failed  int `json:"failed"`
}

// runCoachPayouts は、前の月までの報酬をコーチごとにまとめ、まだ送っていない送金を Stripe で送る。
func (s *Server) runCoachPayouts(ctx context.Context) (payoutRunResult, error) {
	var out payoutRunResult
	// 送金する前に、記録し損ねた報酬を拾う。
	if _, err := s.recordCoachEarnings(ctx); err != nil {
		return out, err
	}
	err := s.tx(ctx, func(q *db.Queries) error {
		if err := q.LockCoachPayouts(ctx); err != nil {
			return err
		}
		rows, err := q.ListPayableCoachEarnings(ctx)
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
			out.Created++
		}
		return nil
	})
	if err != nil {
		return out, err
	}
	unpaid, err := s.q.ListUnpaidCoachPayouts(ctx)
	if err != nil {
		return out, err
	}
	for _, p := range unpaid {
		group := "payout-" + p.ID.String()
		// 前の送金が Stripe 側では成功していた（通信エラーや DB の更新失敗で failed/pending のまま残った）ら、
		// 送り直さずにその送金で済ませる。Idempotency-Key は約24時間で切れるので、それだけでは二重送金を防げない。
		sent, err := s.stripe.FindTransfers(ctx, group)
		if err != nil {
			slog.Warn("stripe find transfers", "payout", p.ID, "err", err)
			msg := err.Error()
			if dbErr := s.q.MarkCoachPayoutFailed(ctx, db.MarkCoachPayoutFailedParams{ID: p.ID, Failure: &msg}); dbErr != nil {
				return out, dbErr
			}
			out.Failed++
			continue
		}
		if len(sent) > 0 {
			if sent[0].Amount != int(p.AmountYen) {
				// 金額の違う送金がすでにある。差額をどうするかは運営が確かめる。
				msg := fmt.Sprintf("金額の違う送金 %s（%d円）がすでにあります", sent[0].ID, sent[0].Amount)
				if dbErr := s.q.MarkCoachPayoutFailed(ctx, db.MarkCoachPayoutFailedParams{ID: p.ID, Failure: &msg}); dbErr != nil {
					return out, dbErr
				}
				out.Failed++
				continue
			}
			if _, err := s.q.MarkCoachPayoutPaid(ctx, db.MarkCoachPayoutPaidParams{ID: p.ID, StripeTransferID: &sent[0].ID}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return out, err
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
			if dbErr := s.q.MarkCoachPayoutFailed(ctx, db.MarkCoachPayoutFailedParams{ID: p.ID, Failure: &msg}); dbErr != nil {
				return out, dbErr
			}
			out.Failed++
			continue
		}
		if _, err := s.q.MarkCoachPayoutPaid(ctx, db.MarkCoachPayoutPaidParams{ID: p.ID, StripeTransferID: &t.ID}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return out, err
		}
		out.Paid++
		if p.CoachID != nil {
			s.notify(ctx, *p.CoachID, notifyImportant, push.Message{
				Title: "報酬を送金しました",
				Body:  fmt.Sprintf("%s分までの報酬 %d円を Stripe のアカウントに送金しました", p.Month.Format("2006年1月"), p.AmountYen),
				URL:   "/coach",
				Tag:   "payout-" + p.ID.String(),
			}, 24*time.Hour)
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

// adminSetReducedFee は、運営がコーチに「初年10%」を付ける（外す）。付けた時から1年のあいだに始まったパーソナル指導が10%。
func (s *Server) adminSetReducedFee(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	c, err := s.q.SetCoachReducedFee(r.Context(), db.SetCoachReducedFeeParams{Enabled: req.Enabled, ProfileID: id})
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"reduced_fee_until": c.ReducedFeeUntil})
}
