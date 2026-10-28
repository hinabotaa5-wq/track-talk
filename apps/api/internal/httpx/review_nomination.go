package httpx

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
)

func reviewCashPrice(rr db.ReviewRequest) int32 {
	if rr.UseTickets {
		return 0
	}
	return rr.BasePriceYen
}

func (s *Server) reviewNominees(ctx context.Context, q *db.Queries, requester uuid.UUID, event string, coach *uuid.UUID) ([]db.ListReviewNomineesRow, error) {
	return q.ListReviewNominees(ctx, db.ListReviewNomineesParams{
		Payments: s.stripe.Configured(), Tier: "coach", Event: event, RequesterID: requester,
		RequestedCoachID: coach, MaxActive: maxActiveReviews, DayStart: dayStart(time.Now()),
		MinRatings: lowRatingMinCount, LowStars: lowRatingStars,
	})
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
	out := make([]map[string]any, 0, len(rows))
	for _, c := range rows {
		// 選ぶ材料は種目・経験年数・記録・紹介文だけ。添削の件数・評価・売上などの人気の指標は出さない。
		out = append(out, map[string]any{"id": c.ProfileID, "nickname": c.Nickname, "photo_url": s.coachPhotoURL(c.PhotoPublicID),
			"events": c.Events, "years": c.Years, "best_record": c.BestRecord, "achievements": c.Achievements})
	}
	JSON(w, http.StatusOK, map[string]any{"coaches": out, "price_yen": reviewPriceYen})
}
