package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/jst"
	"github.com/hinata/track-talk/apps/api/internal/push"
	"github.com/hinata/track-talk/apps/api/internal/stripe"
	"github.com/jackc/pgx/v5"
)

// 1ヶ月プラン（無料相談の提案から成立した、動画への返答が無制限の契約）の途中終了とコーチ報酬の精算。
//
//   - 解約料・違約金・手数料は取らない。未経過分は「契約価格 ÷ 期間日数 × 未経過日数」（円未満切り捨て）で全額返金する。
//     期間日数は契約期間（1ヶ月）の日数で、#488 で確定した数え方（開始日の JST 日付・両端を含む）を流用する。
//   - コーチ報酬は契約終了時に、経過日数の日割り（経過分の価格の75%）で確定する。月次送金はそれまでの「確定前の仮払い」。
//   - コーチ都合（帰責事由あり）は選手へ全額返金する。コーチは、返金額の75%を上限に仮払いを返す（没収ではなく返還・精算）。
//     返金が終わるまで、その契約のコーチ分は送金しない（留保）。
//   - やむを得ない事情・選手の都合は、通常の精算（経過分は支払い、未経過分は返金）。
//   - 大会の中止・延期での終了は、1ヶ月プランでは前提が消滅したため無効化する（下の supportEndCancelled は残すが、
//     新規の終了理由としては受け付けない。既存データの解釈は変えない）。

// 終わり方。supportEndCancelled は無効化済み（受け付けない）のため、新規には使わない。削除はしない。
const (
	supportEndPlayer      = "player"
	supportEndCoachFault  = "coach_fault"
	supportEndUnavoidable = "unavoidable"
	supportEndCancelled   = "competition_cancelled"
)

// supportConsultContract は、無料相談の提案から成立した、動画への返答が無制限の大会までプランか。
func supportConsultContract(ct db.CoachingContract) bool {
	return ct.FulfillmentVersion == 1 && supportUnlimited(ct) && ct.ServiceType != nil && *ct.ServiceType == "competition_support"
}

// supportDay は、日本時間の日付（その日の 0 時、UTC で持つ）。日数の差を時差なしに数えるため。
func supportDay(t time.Time) time.Time {
	j := t.In(jst.Zone)
	return time.Date(j.Year(), j.Month(), j.Day(), 0, 0, 0, 0, time.UTC)
}

// calendarDate は、DB の date 列（UTC の 0 時で読める）をそのまま日付として扱う。
func calendarDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// supportDays は、期間の日数（実際の開始日〜目標大会の日、両端を含む）と、asOf の日までに経過した日数（asOf の日も経過に数える）。
// 始まっていなければ経過0日。
func supportDays(ct db.CoachingContract, asOf time.Time) (elapsed, total int64) {
	if ct.StartsAt == nil || ct.CompetitionDate == nil {
		return 0, 0
	}
	start, end := supportDay(*ct.StartsAt), calendarDate(*ct.CompetitionDate)
	total = int64(end.Sub(start).Hours()/24) + 1
	if total <= 0 {
		return 0, 0
	}
	elapsed = int64(asOf.Sub(start).Hours()/24) + 1
	return min(max(elapsed, 0), total), total
}

// supportUnprovidedAsOf は、asOf の日（経過扱い）の時点の未提供分。始まっていなければ全額、期間が終わっていれば0円。
func supportUnprovidedAsOf(ct db.CoachingContract, asOf time.Time) int32 {
	elapsed, total := supportDays(ct, asOf)
	if total == 0 {
		if ct.StartsAt == nil {
			return ct.PriceYen
		}
		return 0
	}
	return int32(int64(ct.PriceYen) * (total - elapsed) / total)
}

// supportAsOf は、日割りの基準の日。途中終了の日（大会の中止・延期は主催者の発表の日）があればその日、無ければ今日。
func supportAsOf(ct db.CoachingContract, now time.Time) time.Time {
	if ct.SupportEndOn != nil {
		return calendarDate(*ct.SupportEndOn)
	}
	return supportDay(now)
}

// supportCoachShare は、利用者が払った額（返金を引いたもの）のうちコーチの取り分（運営手数料25%を引く、円未満切り捨て）。
func supportCoachShare(gross int32) int32 {
	if gross <= 0 {
		return 0
	}
	return int32(int64(gross) * int64(100-personalFeePercent) / 100)
}

// supportEarnedAsOf は、asOf の日までの経過日数で確定するコーチ報酬。
func supportEarnedAsOf(ct db.CoachingContract, asOf time.Time) int32 {
	return supportCoachShare(ct.PriceYen - supportUnprovidedAsOf(ct, asOf))
}

// supportCoachEarned は、契約終了時に確定するコーチ報酬。
//   - コーチ都合: 0円（選手へ全額返金。仮払いは返還・精算する）
//   - 選手の都合・やむを得ない事情: 終了の日までの経過日数の日割り（返金額によらない）
//   - 大会の中止延期: 新規の終了理由としては受け付けないが、既存データの解釈は変えない（同じく経過日数の日割り）
//   - それ以外（期間を終えた、運営の全額返金など）: 払われた額（返金を引いたもの）の75%
func supportCoachEarned(ct db.CoachingContract, paid, refunded int32) int32 {
	cause := ""
	if ct.SupportEndCause != nil {
		cause = *ct.SupportEndCause
	}
	switch cause {
	case supportEndCoachFault:
		return 0
	case supportEndPlayer, supportEndUnavoidable, supportEndCancelled:
		return supportEarnedAsOf(ct, supportAsOf(ct, time.Now()))
	}
	return supportCoachShare(paid - refunded)
}

// supportSettle は、確定した報酬と仮払いから、新しく記録する報酬（final）と返してもらう額（clawback）を決める。
// まだ送金していない仮払いは消して差し引き（dropUnpaid）、送金済みの仮払いが確定額を超える分だけを返還とする。
// 返還は、選手への返金額のコーチ報酬相当分（75%）が上限。それを超える分は求めない。
func supportSettle(earned, paidAdvances, unpaidAdvances, refunded int32) (final, clawback int32, dropUnpaid bool) {
	if earned >= paidAdvances+unpaidAdvances {
		return earned - paidAdvances - unpaidAdvances, 0, false
	}
	if earned >= paidAdvances {
		return earned - paidAdvances, 0, true
	}
	return 0, min(paidAdvances-earned, supportCoachShare(refunded)), true
}

// supportAdvanceThrough は、月次の仮払いの締めの日（前の月の末日、日本時間）と、その日の終わりの時刻。
func supportAdvanceThrough(now time.Time) (through time.Time, end time.Time) {
	j := now.In(jst.Zone)
	monthStart := time.Date(j.Year(), j.Month(), 1, 0, 0, 0, 0, jst.Zone)
	last := monthStart.AddDate(0, 0, -1)
	return time.Date(last.Year(), last.Month(), last.Day(), 0, 0, 0, 0, time.UTC), monthStart
}

// recordSupportAdvances は、期間中の契約の、前の月の末日までの経過日数ぶんの報酬を仮払いとして記録する。
// 同じ月に何度呼んでも、累計との差額だけを記録するので重ならない。
func recordSupportAdvances(ctx context.Context, q *db.Queries, now time.Time) (int, error) {
	through, end := supportAdvanceThrough(now)
	rows, err := q.ListSupportAdvanceCandidates(ctx, end)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, r := range rows {
		ct := db.CoachingContract{PriceYen: r.PriceYen, StartsAt: r.StartsAt, CompetitionDate: r.CompetitionDate}
		elapsed, total := supportDays(ct, through)
		if elapsed <= 0 {
			continue
		}
		amount := supportEarnedAsOf(ct, through) - r.AdvancedYen
		if amount <= 0 {
			continue
		}
		coach := r.CoachID
		e, err := q.InsertSupportAdvanceEarning(ctx, db.InsertSupportAdvanceEarningParams{
			ID: uuid.New(), CoachID: &coach, AmountYen: amount, ConfirmedAt: end.Add(-time.Second),
		})
		if err != nil {
			return n, err
		}
		if err := q.InsertSupportAdvance(ctx, db.InsertSupportAdvanceParams{
			EarningID: e.ID, ContractID: r.ID, ThroughDate: through, ElapsedDays: int32(elapsed), TotalDays: int32(total), AmountYen: amount,
		}); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// finalizeSupportEarnings は、返金と精算が終わった契約のコーチ報酬を確定する（止めているコーチ都合の契約は除く）。
func finalizeSupportEarnings(ctx context.Context, q *db.Queries) (int, error) {
	rows, err := q.ListSupportFinalizations(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, ct := range rows {
		if _, err := finalizeSupportContract(ctx, q, ct); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// finalizeSupportContract は、1つの契約のコーチ報酬を確定し、仮払いとの差額を記録するか、返還を返金相殺の台帳に載せる。
func finalizeSupportContract(ctx context.Context, q *db.Queries, ct db.CoachingContract) (db.SupportCoachSettlement, error) {
	paid, err := q.GetSupportPaid(ctx, &ct.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		paid = db.GetSupportPaidRow{}
	} else if err != nil {
		return db.SupportCoachSettlement{}, err
	}
	totals, err := q.SupportAdvanceTotals(ctx, ct.ID)
	if err != nil {
		return db.SupportCoachSettlement{}, err
	}
	earned := supportCoachEarned(ct, paid.AmountYen, paid.RefundedYen)
	final, clawback, dropUnpaid := supportSettle(earned, totals.PaidYen, totals.TotalYen-totals.PaidYen, paid.RefundedYen)
	if dropUnpaid {
		if err := q.DeleteUnpaidSupportAdvances(ctx, ct.ID); err != nil {
			return db.SupportCoachSettlement{}, err
		}
	}
	confirmed := time.Now()
	if ct.EndsAt != nil && ct.EndsAt.Before(confirmed) {
		confirmed = *ct.EndsAt
	}
	if paid.RefundedAt != nil && paid.RefundedAt.After(confirmed) {
		confirmed = *paid.RefundedAt
	}
	coach := ct.CoachID
	if final > 0 {
		if err := q.InsertSupportFinalEarning(ctx, db.InsertSupportFinalEarningParams{
			ID: uuid.New(), CoachID: &coach, ContractID: &ct.ID, GrossYen: max(paid.AmountYen-paid.RefundedYen, 1),
			SharePercent: int16(100 - personalFeePercent), AmountYen: final, ConfirmedAt: confirmed,
		}); err != nil {
			return db.SupportCoachSettlement{}, err
		}
	}
	if clawback > 0 {
		if err := q.InsertSupportClawback(ctx, db.InsertSupportClawbackParams{ID: uuid.New(), ContractID: &ct.ID, CoachID: &coach, AmountYen: clawback}); err != nil {
			return db.SupportCoachSettlement{}, err
		}
	}
	advances := totals.PaidYen
	if !dropUnpaid {
		advances = totals.TotalYen
	}
	return q.InsertSupportSettlement(ctx, db.InsertSupportSettlementParams{
		ContractID: ct.ID, EarnedYen: earned, AdvancesYen: advances, FinalYen: final, ClawbackYen: clawback,
	})
}

// supportEndReq は、運営が大会までプランを途中で終えるときの入力。
type supportEndReq struct {
	Cause       string `json:"cause"`
	Reason      string `json:"reason"`
	AnnouncedOn string `json:"announced_on"`
}

// adminEndSupportContract は、運営が大会までプランを途中で終え、精算する。
// コーチ都合・やむを得ない事情の判定は、理由をコーチに書面（この記録）で示し、コーチ都合には異議申立てを1回受ける。
func (s *Server) adminEndSupportContract(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var req supportEndReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "入力が不正です")
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	switch req.Cause {
	case supportEndPlayer, supportEndCoachFault, supportEndUnavoidable:
	case supportEndCancelled:
		// 大会の中止・延期での終了は、1ヶ月プランでは前提が消滅したため受け付けない（無効化。定数・列・既存の解釈は残す）。
		Error(w, http.StatusBadRequest, "validation_error", "大会の中止・延期による終了は1ヶ月プランでは扱いません")
		return
	default:
		Error(w, http.StatusBadRequest, "validation_error", "終わり方が不正です")
		return
	}
	needsReason := req.Cause == supportEndCoachFault || req.Cause == supportEndUnavoidable
	if needsReason && (req.Reason == "" || len([]rune(req.Reason)) > 2000) {
		Error(w, http.StatusBadRequest, "validation_error", "判定の理由を2000字以内で書いてください")
		return
	}
	endOn := supportDay(time.Now())
	if req.Cause == supportEndCancelled {
		d, err := time.Parse(time.DateOnly, req.AnnouncedOn)
		if err != nil || d.After(endOn) {
			Error(w, http.StatusBadRequest, "validation_error", "主催者が中止・延期を正式に発表した日（今日まで）を入れてください")
			return
		}
		endOn = d
	}
	err := s.tx(r.Context(), func(q *db.Queries) error {
		if err := q.LockCoachPayouts(r.Context()); err != nil {
			return err
		}
		ct, err := q.LockConsultationContract(r.Context(), id)
		if err != nil {
			return err
		}
		if !supportConsultContract(ct) {
			return &ticketRuleError{"invalid_support", "無料相談から成立した大会までプランを選んでください"}
		}
		if ct.SupportEndCause != nil || ct.FulfillmentState != "open" || (ct.Status != "active" && ct.Status != "suspended") {
			return &ticketRuleError{"conflict", "期間中の契約だけを途中で終えられます"}
		}
		if _, err := q.SetSupportEnd(r.Context(), db.SetSupportEndParams{ID: ct.ID, Cause: &req.Cause, EndOn: &endOn, Hold: req.Cause == supportEndCoachFault}); err != nil {
			return err
		}
		if req.Cause == supportEndCoachFault {
			if err := q.DeleteUnpaidSupportAdvances(r.Context(), ct.ID); err != nil {
				return err
			}
		}
		if needsReason {
			if _, err := q.InsertFaultDetermination(r.Context(), db.InsertFaultDeterminationParams{ContractID: ct.ID, Decision: req.Cause, Reason: req.Reason, DecidedBy: &admin.ID}); err != nil {
				return err
			}
		}
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return
	}
	if err != nil {
		writeCheckoutError(w, err)
		return
	}
	if err := s.prepareSupportSettlement(r.Context(), id, "end"); err != nil {
		writeCheckoutError(w, err)
		return
	}
	if err := s.processSupportRefund(r.Context(), id); err != nil {
		slog.Error("support end refund", "contract", id, "err", err)
	}
	ct, err := s.q.GetCoachingContract(r.Context(), id)
	if err != nil {
		MapDBError(w, err)
		return
	}
	msg := push.Message{Title: "大会までプランが終了しました", Body: "「" + ct.PlanTitle + "」が終了しました。返金と精算の内容を確認してください", URL: "/coaching/personal/" + ct.ID.String(), Tag: "contract-" + ct.ID.String()}
	s.notify(r.Context(), ct.RequesterID, notifyImportant, msg, 24*time.Hour)
	if needsReason {
		msg.Body = "「" + ct.PlanTitle + "」の終了について、運営の判定と理由を確認してください"
	}
	msg.URL = "/coach/support-contracts/" + ct.ID.String()
	s.notify(r.Context(), ct.CoachID, notifyImportant, msg, 24*time.Hour)
	out, err := s.supportSettlementJSON(r.Context(), ct)
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, out)
}

// adminReviewSupportFault は、コーチの異議申立てを再審査する（1回）。
func (s *Server) adminReviewSupportFault(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	if err := DecodeJSON(r, &req); err != nil || (req.Decision != supportEndCoachFault && req.Decision != supportEndUnavoidable) {
		Error(w, http.StatusBadRequest, "validation_error", "再審査の結果が不正です")
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" || len([]rune(req.Reason)) > 2000 {
		Error(w, http.StatusBadRequest, "validation_error", "再審査の理由を2000字以内で書いてください")
		return
	}
	d, err := s.q.ReviewFaultDetermination(r.Context(), db.ReviewFaultDeterminationParams{ContractID: id, Decision: &req.Decision, Reason: &req.Reason, ReviewedBy: &admin.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusConflict, "conflict", "再審査できる異議申立てがありません")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	ct, err := s.q.GetCoachingContract(r.Context(), id)
	if err != nil {
		MapDBError(w, err)
		return
	}
	s.notify(r.Context(), ct.CoachID, notifyImportant, push.Message{Title: "異議申立ての再審査が終わりました", Body: "「" + ct.PlanTitle + "」の再審査の結果と理由を確認してください", URL: "/coach/support-contracts/" + ct.ID.String(), Tag: "contract-" + ct.ID.String()}, 24*time.Hour)
	JSON(w, http.StatusOK, map[string]any{"determination": determinationJSON(d)})
}

// adminFinalizeSupportFault は、コーチ都合で止めていた契約のコーチ分を精算する。
// 選手への返金が終わり、異議申立てが無いか再審査が終わっていることが条件。期限で自動に確定はしない（運営の操作で確定する）。
// 再審査でやむを得ない事情となった場合は、通常の精算（終了の日までの経過分を支払う）に切り替える。
func (s *Server) adminFinalizeSupportFault(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var out db.CoachingContract
	err := s.tx(r.Context(), func(q *db.Queries) error {
		if err := q.LockCoachPayouts(r.Context()); err != nil {
			return err
		}
		ct, err := q.LockConsultationContract(r.Context(), id)
		if err != nil {
			return err
		}
		if !ct.SupportCoachHold || ct.FulfillmentState != "settled" {
			return &ticketRuleError{"conflict", "選手への返金が終わった、コーチ都合の契約だけを精算できます"}
		}
		d, err := q.GetFaultDetermination(r.Context(), ct.ID)
		if err != nil {
			return err
		}
		if d.ObjectedAt != nil && d.ReviewedAt == nil {
			return &ticketRuleError{"conflict", "異議申立ての再審査が終わっていません"}
		}
		cause := d.Decision
		if d.ReviewDecision != nil {
			cause = *d.ReviewDecision
		}
		if ct, err = q.ReleaseSupportHold(r.Context(), db.ReleaseSupportHoldParams{ID: ct.ID, Cause: &cause}); err != nil {
			return err
		}
		if _, err := finalizeSupportContract(r.Context(), q, ct); err != nil {
			return err
		}
		out = ct
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return
	}
	if err != nil {
		writeCheckoutError(w, err)
		return
	}
	res, err := s.supportSettlementJSON(r.Context(), out)
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, res)
}

// objectSupportFault は、コーチがコーチ都合の判定に異議を申し立てる（1回だけ）。
func (s *Server) objectSupportFault(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		Objection string `json:"objection"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "入力が不正です")
		return
	}
	req.Objection = strings.TrimSpace(req.Objection)
	if req.Objection == "" || len([]rune(req.Objection)) > 2000 {
		Error(w, http.StatusBadRequest, "validation_error", "異議の内容を2000字以内で書いてください")
		return
	}
	ct, err := s.q.GetCoachingContract(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && ct.CoachID != me.ID) {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	d, err := s.q.ObjectFaultDetermination(r.Context(), db.ObjectFaultDeterminationParams{ContractID: id, Objection: &req.Objection})
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusConflict, "conflict", "異議申立てはできません（1回まで・精算の確定前のコーチ都合の判定のみ）")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"determination": determinationJSON(d)})
}

// getCoachSupportTerms は、コーチが自分の契約の取引条件の明示（フリーランス法第3条）と、判定・精算を見る。
func (s *Server) getCoachSupportTerms(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	ct, err := s.q.GetCoachingContract(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (ct.CoachID != me.ID || !supportConsultContract(ct))) {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	out, err := s.supportSettlementJSON(r.Context(), ct)
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, out)
}

func determinationJSON(d db.SupportFaultDetermination) map[string]any {
	return map[string]any{
		"decision": d.Decision, "reason": d.Reason, "notified_at": d.NotifiedAt,
		"objection": d.Objection, "objected_at": d.ObjectedAt,
		"review_decision": d.ReviewDecision, "review_reason": d.ReviewReason, "reviewed_at": d.ReviewedAt,
	}
}

// supportSettlementJSON は、契約の終わり方・仮払い・確定・判定・取引条件の明示をまとめる。
func (s *Server) supportSettlementJSON(ctx context.Context, ct db.CoachingContract) (map[string]any, error) {
	out := map[string]any{"contract": s.contractJSON(ct), "end_cause": ct.SupportEndCause, "end_on": ct.SupportEndOn, "coach_hold": ct.SupportCoachHold}
	totals, err := s.q.SupportAdvanceTotals(ctx, ct.ID)
	if err != nil {
		return nil, err
	}
	out["advances"] = map[string]any{"total_yen": totals.TotalYen, "paid_yen": totals.PaidYen}
	if st, err := s.q.GetSupportSettlement(ctx, ct.ID); err == nil {
		out["settlement"] = map[string]any{"earned_yen": st.EarnedYen, "advances_yen": st.AdvancesYen, "final_yen": st.FinalYen, "clawback_yen": st.ClawbackYen, "settled_at": st.CreatedAt}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if d, err := s.q.GetFaultDetermination(ctx, ct.ID); err == nil {
		out["determination"] = determinationJSON(d)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if st, err := s.q.GetContractStatement(ctx, ct.ID); err == nil {
		var body map[string]any
		_ = json.Unmarshal(st.Body, &body)
		out["statement"] = map[string]any{"issued_at": st.IssuedAt, "body": body}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	return out, nil
}

// supportPayoutDayText は、仮払いと確定分の支払期日。
func supportPayoutDayText() string {
	a := stripe.PayoutMonthlyAnchor
	return fmt.Sprintf("前の月の末日までの仮払い・確定分を、翌月%d日（日本時間）にまとめて送金します。ただし、各締切時点の支払額（税込）が%s円未満の場合は翌月以降に繰り越し、累積した支払額が%s円以上となった締切月の翌月%d日に送金します。なお、コーチとの取引が終了した場合は、金額にかかわらず、終了月の翌月%d日に未払額の全額を送金します",
		a, formatYenNumber(minPayoutYen), formatYenNumber(minPayoutYen), a, a)
}

// issueSupportContractStatement は、契約ごとの取引条件をコーチへ電磁的に明示する（フリーランス法第3条に対応する形）。
func issueSupportContractStatement(ctx context.Context, q *db.Queries, ct db.CoachingContract) error {
	if !supportConsultContract(ct) {
		return nil
	}
	competition := ""
	if ct.CompetitionName != nil {
		competition = *ct.CompetitionName
	}
	period := ""
	if ct.SupportStartDate != nil && ct.CompetitionDate != nil {
		period = ct.SupportStartDate.Format("2006年1月2日") + "〜" + ct.CompetitionDate.Format("2006年1月2日") + "（1ヶ月・30日間）"
	}
	desc := ""
	if ct.PlanDescription != nil {
		desc = *ct.PlanDescription
	}
	body := map[string]any{
		"title":           "取引条件の明示（1ヶ月プラン）",
		"contract_id":     ct.ID.String(),
		"contracted_on":   supportDay(time.Now()).Format(time.DateOnly),
		"principal":       "TrackTalk 運営者（本サービスの運営者）",
		"service":         "1ヶ月プランのサポート（目標: " + competition + "）。" + desc + " 返答は24時間以内を目安とします。",
		"period":          period,
		"price_yen":       ct.PriceYen,
		"fee_percent":     personalFeePercent,
		"coach_share_yen": supportCoachShare(ct.PriceYen),
		"pay_calculation": "報酬は契約終了時に、経過日数の日割り（契約価格 ÷ 期間日数 × 経過日数、円未満切り捨て）の75%で確定します。期間日数は契約期間（1ヶ月）の日数（実際の開始日から両端を含む30日）です。",
		"advance":         "期間中は、毎月末日までの経過日数ぶんを確定前の仮払いとして送金します。確定額との差額は契約終了後に精算します。",
		"payment_due":     supportPayoutDayText(),
		"early_end":       "途中で終わった場合も解約料は取りません。選手へは未経過分を日割りで全額返金し、報酬は終了日までの経過分で確定します。",
		"coach_fault":     "コーチの責めに帰すべき事由で終わった場合は選手へ全額返金し、コーチは返金額の75%を上限に受け取った仮払いを返還します（それを超える賠償は求めません）。返金が終わるまで、この契約の報酬の送金を留めます。",
		"unavoidable":     "本人の入院・死亡、災害による業務不能、法令による業務停止など、やむを得ない事情の場合は通常の精算とします。判定の理由は書面（電磁的記録）で通知し、異議申立て（再審査）を1回受け付けます。",
		"payment_method":  "Stripe Connect の受け取り口座への送金",
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	coach := ct.CoachID
	return q.InsertContractStatement(ctx, db.InsertContractStatementParams{ContractID: ct.ID, CoachID: &coach, Body: raw})
}
