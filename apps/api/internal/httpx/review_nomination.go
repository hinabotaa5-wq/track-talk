package httpx

import (
	"context"
	"net/http"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
)

func reviewCashPrice(rr db.ReviewRequest) int32 {
	if rr.UseTickets {
		return 0
	}
	return couponCashPrice(rr.BasePriceYen, rr.CouponDiscountYen)
}

// 指名候補の一覧と、申込み時の指名の確認の両方がここを通る。特集モードでは特集コーチ以外を候補から外す（本体 #985）。
func (s *Server) reviewNominees(ctx context.Context, q *db.Queries, requester uuid.UUID, event string, coach *uuid.UUID) ([]db.ListReviewNomineesRow, error) {
	rows, err := q.ListReviewNominees(ctx, db.ListReviewNomineesParams{
		Payments: s.stripe.Configured(), Tier: "coach", Event: event, RequesterID: requester,
		RequestedCoachID: coach, DayStart: dayStart(time.Now()),
	})
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(rows, func(c db.ListReviewNomineesRow) bool { return s.coachHiddenByFeature(c.ProfileID) }), nil
}

func (s *Server) listReviewNominees(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	event := r.URL.Query().Get("event")
	if !validReviewEvent(event) {
		FieldError(w, map[string]string{"event": "対応している種目を選んでください"})
		return
	}
	rows, err := s.reviewNominees(r.Context(), s.q, p.ID, event, nil)
	if err != nil {
		MapDBError(w, err)
		return
	}
	// プレミアム（本体 #1053）は、プレミアム価格があるコーチだけを、その価格と一緒に返す。
	if r.URL.Query().Get("plan") == planPremium {
		out, err := s.premiumNomineesJSON(r.Context(), rows)
		if err != nil {
			MapDBError(w, err)
			return
		}
		JSON(w, http.StatusOK, map[string]any{"coaches": out})
		return
	}
	ids := make([]uuid.UUID, 0, len(rows))
	for _, c := range rows {
		ids = append(ids, c.ProfileID)
	}
	// 申込価格はコーチのいま有効なスタンダード価格（本体 #1052）。price_yen（全体）は未設定のコーチの価格。
	prices, err := s.effectiveCoachPrices(r.Context(), s.q, ids)
	if err != nil {
		MapDBError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, c := range rows {
		// 選ぶ材料は種目・経験年数・記録・紹介文だけ。添削の件数・評価・売上などの人気の指標は出さない。
		out = append(out, map[string]any{"id": c.ProfileID, "nickname": c.Nickname, "photo_url": s.coachPhotoURL(c.PhotoPublicID),
			"events": c.Events, "years": c.Years, "best_record": c.BestRecord, "achievements": c.Achievements,
			"price_yen": prices[c.ProfileID].Standard})
	}
	JSON(w, http.StatusOK, map[string]any{"coaches": out, "price_yen": reviewPriceYen})
}
