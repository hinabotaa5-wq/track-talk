package httpx

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/push"
	"github.com/jackc/pgx/v5"
)

// コーチの価格申請制（本体 #1052、運営者決定 2026-11-25／2026-11-26）。
// コーチはプラン（スタンダード／プレミアム）ごとに価格を申請し、運営が承認する（却下はしない方針。承認は確認・記録の場）。
// 承認しても即時には切り替えない。承認の時点でそのコーチが対応中だった依頼がすべて終わってから有効になる
// （ApplyDueCoachPrices）。切り替えまでの新規申込みは旧価格で受け付け、進行中の依頼は作成時の base_price_yen のまま完走する。
// 金額は円のみ。海外コーチも円で申請する（公開ページの $ 表示は変えない）。

const (
	coachPlanStandard = "standard"
	coachPlanPremium  = "premium"
	// 申請できる下限（円）。上限は設けない。
	coachPriceMinYen int32 = 500
	// 入力の桁あふれを防ぐだけの技術的な上限（int32 に収まる範囲）。運営の方針としての上限ではない。
	coachPriceInputMaxYen int64 = 2_000_000_000
)

func validCoachPlan(plan string) bool { return plan == coachPlanStandard || plan == coachPlanPremium }

// coachPriceSet は、コーチのいま有効な価格。Standard は未設定なら reviewPriceYen（1,000円）。Premium は未設定なら nil。
type coachPriceSet struct {
	Standard    int32
	StandardSet bool
	Premium     *int32
}

// coachPriceError は、申請金額の入力チェック。下限500円のみ（上限なし）。
func coachPriceError(v *int64) string {
	if v == nil {
		return "金額を入力してください"
	}
	if *v < int64(coachPriceMinYen) {
		return "価格は500円以上で入力してください"
	}
	if *v > coachPriceInputMaxYen {
		return "金額が大きすぎます"
	}
	return ""
}

// applyDueCoachPrices は、切り替えの条件を満たした承認済みの申請を有効にし、コーチに知らせる。
// coachIDs が空なら全コーチ（tick）。同じ申請は1回しか切り替わらないので、知らせも1回だけ届く。
func (s *Server) applyDueCoachPrices(ctx context.Context, coachIDs ...uuid.UUID) (int, error) {
	if coachIDs == nil {
		coachIDs = []uuid.UUID{}
	}
	rows, err := s.q.ApplyDueCoachPrices(ctx, coachIDs)
	if err != nil {
		return 0, err
	}
	for _, row := range rows {
		s.notifyCoachPriceApplied(ctx, row.CoachID)
	}
	return len(rows), nil
}

// effectiveCoachPrices は、コーチたちのいま有効な価格を返す。読む前に、切り替えの条件を満たした申請を有効にする。
// 切り替えに失敗しても、いまの有効価格で答える（表示や申込みを止めない。tick が拾い直す）。
func (s *Server) effectiveCoachPrices(ctx context.Context, q *db.Queries, coachIDs []uuid.UUID) (map[uuid.UUID]coachPriceSet, error) {
	out := make(map[uuid.UUID]coachPriceSet, len(coachIDs))
	if len(coachIDs) == 0 {
		return out, nil
	}
	if q == s.q {
		if _, err := s.applyDueCoachPrices(ctx, coachIDs...); err != nil {
			slog.Warn("apply due coach prices", "err", err)
		}
	}
	for _, id := range coachIDs {
		out[id] = coachPriceSet{Standard: reviewPriceYen}
	}
	rows, err := q.ListCoachPrices(ctx, coachIDs)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		set := out[row.CoachID]
		switch row.Plan {
		case coachPlanStandard:
			set.Standard, set.StandardSet = row.PriceYen, true
		case coachPlanPremium:
			price := row.PriceYen
			set.Premium = &price
		}
		out[row.CoachID] = set
	}
	return out, nil
}

// coachStandardPriceYen は、指名したコーチのスタンダードの申込価格。コーチが決まっていなければ reviewPriceYen。
func (s *Server) coachStandardPriceYen(ctx context.Context, q *db.Queries, coachID *uuid.UUID) (int32, error) {
	if coachID == nil {
		return reviewPriceYen, nil
	}
	prices, err := s.effectiveCoachPrices(ctx, q, []uuid.UUID{*coachID})
	if err != nil {
		return 0, err
	}
	return prices[*coachID].Standard, nil
}

// coachPremiumPriceYen は、コーチのプレミアムの有効価格。未設定（まだ承認・切り替えされていない）なら nil。
// プレミアムの申込み（本体 #1053）はこの値を使う。
func (s *Server) coachPremiumPriceYen(ctx context.Context, q *db.Queries, coachID uuid.UUID) (*int32, error) {
	prices, err := s.effectiveCoachPrices(ctx, q, []uuid.UUID{coachID})
	if err != nil {
		return nil, err
	}
	return prices[coachID].Premium, nil
}

func coachPriceApplicationJSON(a db.CoachPriceApplication) map[string]any {
	return map[string]any{
		"id": a.ID, "plan": a.Plan, "price_yen": a.PriceYen, "source": a.Source, "status": a.Status,
		"created_at": a.CreatedAt, "updated_at": a.UpdatedAt, "approved_at": a.ApprovedAt, "applied_at": a.AppliedAt,
	}
}

// requireApprovedCoach は、承認されたコーチ本人だけを通す。
func (s *Server) requireApprovedCoach(w http.ResponseWriter, r *http.Request) (db.Profile, bool) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return db.Profile{}, false
	}
	c, err := s.q.GetCoachProfile(r.Context(), p.ID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && c.Status != "approved") {
		Error(w, http.StatusForbidden, "forbidden", "承認されたコーチだけが価格を申請できます")
		return db.Profile{}, false
	}
	if err != nil {
		MapDBError(w, err)
		return db.Profile{}, false
	}
	return p, true
}

// myCoachPrices は、コーチ本人の価格（プランごとの有効価格と、承認待ち・切り替え待ちの申請）。
func (s *Server) myCoachPrices(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireApprovedCoach(w, r)
	if !ok {
		return
	}
	s.writeMyCoachPrices(w, r, p.ID)
}

func (s *Server) writeMyCoachPrices(w http.ResponseWriter, r *http.Request, coachID uuid.UUID) {
	prices, err := s.effectiveCoachPrices(r.Context(), s.q, []uuid.UUID{coachID})
	if err != nil {
		MapDBError(w, err)
		return
	}
	apps, err := s.q.ListCoachPriceApplicationsFor(r.Context(), coachID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	set := prices[coachID]
	plans := make([]map[string]any, 0, 2)
	for _, plan := range []string{coachPlanStandard, coachPlanPremium} {
		entry := map[string]any{"plan": plan, "pending": nil, "approved": nil}
		if plan == coachPlanStandard {
			entry["price_yen"] = set.Standard
			// 未設定のコーチは共通の価格（reviewPriceYen）で受け付けている。
			entry["is_default"] = !set.StandardSet
		} else {
			entry["price_yen"] = set.Premium
			entry["is_default"] = false
		}
		for _, a := range apps {
			if a.Plan != plan {
				continue
			}
			switch a.Status {
			case "pending":
				entry["pending"] = coachPriceApplicationJSON(a)
			case "approved":
				entry["approved"] = coachPriceApplicationJSON(a)
			}
		}
		plans = append(plans, entry)
	}
	JSON(w, http.StatusOK, map[string]any{"plans": plans, "min_price_yen": coachPriceMinYen, "default_price_yen": reviewPriceYen})
}

type coachPriceReq struct {
	PriceYen *int64 `json:"price_yen"`
}

// applyCoachPrice は、プランの価格を申請する。承認待ちがあれば金額を書き換える（上書き申請）。
// 承認されたあとの申請はコーチからは変えられない（新しい申請を出すことはできる。頻度の制限はない）。
func (s *Server) applyCoachPrice(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireApprovedCoach(w, r)
	if !ok {
		return
	}
	plan := chi.URLParam(r, "plan")
	if !validCoachPlan(plan) {
		Error(w, http.StatusNotFound, "not_found", "プランが見つかりません")
		return
	}
	var req coachPriceReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	if e := coachPriceError(req.PriceYen); e != "" {
		FieldError(w, map[string]string{"price_yen": e})
		return
	}
	if _, err := s.q.UpsertPendingCoachPrice(r.Context(), db.UpsertPendingCoachPriceParams{
		ID: uuid.New(), CoachID: p.ID, Plan: plan, PriceYen: int32(*req.PriceYen), Source: "price",
	}); err != nil {
		MapDBError(w, err)
		return
	}
	s.writeMyCoachPrices(w, r, p.ID)
}

// withdrawCoachPrice は、承認待ちの申請を取り下げる。承認済みのものは取り下げられない。
func (s *Server) withdrawCoachPrice(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireApprovedCoach(w, r)
	if !ok {
		return
	}
	plan := chi.URLParam(r, "plan")
	if !validCoachPlan(plan) {
		Error(w, http.StatusNotFound, "not_found", "プランが見つかりません")
		return
	}
	_, err := s.q.WithdrawPendingCoachPrice(r.Context(), db.WithdrawPendingCoachPriceParams{CoachID: p.ID, Plan: plan})
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusConflict, "conflict", "取り下げられる申請がありません（承認済みの申請は取り下げられません）")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	s.writeMyCoachPrices(w, r, p.ID)
}

// recordCoachApplicationPrices は、コーチ応募フォームの希望価格を承認待ちの申請として残す。
// 未入力のプランは何もしない（応募そのものは希望価格なしでもできる）。
func recordCoachApplicationPrices(ctx context.Context, q *db.Queries, coachID uuid.UUID, wanted map[string]*int64) error {
	for _, plan := range []string{coachPlanStandard, coachPlanPremium} {
		v := wanted[plan]
		if v == nil {
			continue
		}
		if _, err := q.UpsertPendingCoachPrice(ctx, db.UpsertPendingCoachPriceParams{
			ID: uuid.New(), CoachID: coachID, Plan: plan, PriceYen: int32(*v), Source: "coach_apply",
		}); err != nil {
			return err
		}
	}
	return nil
}

// adminCoachPrices は、運営の価格申請の一覧（承認待ちを先に）。
func (s *Server) adminCoachPrices(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	// 切り替え待ちのうち条件を満たしたものを先に有効にしてから並べる。
	if _, err := s.applyDueCoachPrices(r.Context()); err != nil {
		slog.Warn("apply due coach prices", "err", err)
	}
	rows, err := s.q.ListAdminCoachPriceApplications(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	ids := make([]uuid.UUID, 0, len(rows))
	seen := map[uuid.UUID]bool{}
	for _, row := range rows {
		if !seen[row.CoachID] {
			seen[row.CoachID] = true
			ids = append(ids, row.CoachID)
		}
	}
	current, err := s.effectiveCoachPrices(r.Context(), s.q, ids)
	if err != nil {
		MapDBError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		set := current[row.CoachID]
		var currentPrice any = set.Standard
		if row.Plan == coachPlanPremium {
			currentPrice = set.Premium
		}
		out = append(out, map[string]any{
			"id": row.ID, "coach_id": row.CoachID, "coach_name": row.CoachName, "coach_status": row.CoachStatus,
			"coach_country_code": row.CoachCountryCode, "plan": row.Plan, "price_yen": row.PriceYen,
			"current_price_yen": currentPrice, "source": row.Source, "status": row.Status,
			"created_at": row.CreatedAt, "updated_at": row.UpdatedAt,
			"approved_at": row.ApprovedAt, "approved_by_nickname": row.ApprovedByNickname, "applied_at": row.AppliedAt,
		})
	}
	JSON(w, http.StatusOK, map[string]any{"applications": out})
}

// adminApproveCoachPrice は、申請を承認する（承認した人・日時を残す）。即時には切り替えず、
// 承認の時点で対応中の依頼がなければ、このあとすぐに有効になる。
func (s *Server) adminApproveCoachPrice(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	var approved db.CoachPriceApplication
	err := s.tx(ctx, func(q *db.Queries) error {
		a, err := q.GetCoachPriceApplication(ctx, id)
		if err != nil {
			return err
		}
		if err := q.LockCoachPrices(ctx, a.CoachID); err != nil {
			return err
		}
		// 切り替え前の古い承認は、新しい承認で置き換える。
		if a.Status == "pending" {
			if err := q.SupersedeApprovedCoachPrice(ctx, db.SupersedeApprovedCoachPriceParams{CoachID: a.CoachID, Plan: a.Plan}); err != nil {
				return err
			}
		}
		approved, err = q.ApproveCoachPrice(ctx, db.ApproveCoachPriceParams{ID: id, ApprovedBy: &admin.ID})
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		if _, getErr := s.q.GetCoachPriceApplication(ctx, id); errors.Is(getErr, pgx.ErrNoRows) {
			Error(w, http.StatusNotFound, "not_found", "申請が見つかりません")
			return
		}
		Error(w, http.StatusConflict, "conflict", "この申請は承認待ちではありません（取り下げ済みか、承認済みです）")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	s.notifyCoachPriceApproved(ctx, approved.CoachID)
	if _, err := s.applyDueCoachPrices(ctx, approved.CoachID); err != nil {
		slog.Warn("apply due coach prices", "err", err)
	}
	if latest, err := s.q.GetCoachPriceApplication(ctx, approved.ID); err == nil {
		approved = latest
	}
	JSON(w, http.StatusOK, map[string]any{"application": coachPriceApplicationJSON(approved)})
}

func (s *Server) notifyCoachPriceApproved(ctx context.Context, coachID uuid.UUID) {
	s.notify(ctx, coachID, notifyCoaching, push.Message{
		Title: "価格の申請が承認されました",
		Body:  "対応中の依頼がすべて終わると、新しい価格に切り替わります",
		URL:   "/coach/price",
		Tag:   "coach-price",
	}, 24*time.Hour)
}

func (s *Server) notifyCoachPriceApplied(ctx context.Context, coachID uuid.UUID) {
	s.notify(ctx, coachID, notifyCoaching, push.Message{
		Title: "新しい価格に切り替わりました",
		Body:  "これからの申込みは新しい価格で受け付けます",
		URL:   "/coach/price",
		Tag:   "coach-price",
	}, 24*time.Hour)
}

// parseCoachApplicationPrice は、応募フォームの希望価格（任意）を確かめる。空欄（null）は未入力。
func parseCoachApplicationPrice(fields map[string]string, key string, v *int64) *int64 {
	if v == nil {
		return nil
	}
	if e := coachPriceError(v); e != "" {
		fields[key] = strings.Replace(e, "価格は", "希望価格は", 1)
		return nil
	}
	return v
}
