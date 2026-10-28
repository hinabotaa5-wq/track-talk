package httpx

import (
	"net/http"
	"slices"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
)

var coachingServiceTypes = []string{"video_review", "competition_support", "in_person"}

func normalizeCoachingServices(values []string) ([]string, bool) {
	out := []string{}
	for _, v := range values {
		if !slices.Contains(coachingServiceTypes, v) {
			return nil, false
		}
	}
	for _, v := range coachingServiceTypes {
		if slices.Contains(values, v) {
			out = append(out, v)
		}
	}
	return out, true
}

// サービス設定とプランの保存は同じロックを使い、未対応プランの公開を防ぐ。
func (s *Server) updateCoachServices(w http.ResponseWriter, r *http.Request) {
	coach, ok := s.approvedCoach(w, r)
	if !ok {
		return
	}
	var req struct {
		ServiceTypes *[]string `json:"service_types"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	if req.ServiceTypes == nil {
		FieldError(w, map[string]string{"service_types": "担当するサービスを選んでください"})
		return
	}
	services, known := normalizeCoachingServices(*req.ServiceTypes)
	if !known {
		FieldError(w, map[string]string{"service_types": "担当するサービスを選び直してください"})
		return
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := s.q.WithTx(tx)
	if err := q.LockCoachingPlans(r.Context(), coach.ProfileID); err != nil {
		MapDBError(w, err)
		return
	}
	blocked, err := q.HasPublishedPlanOutsideServices(r.Context(), db.HasPublishedPlanOutsideServicesParams{CoachID: coach.ProfileID, ServiceTypes: services})
	if err != nil {
		MapDBError(w, err)
		return
	}
	if blocked {
		Error(w, http.StatusConflict, "published_service_in_use", "担当を外す前に、そのサービスのプランを非公開にしてください")
		return
	}
	updated, err := q.UpdateCoachServices(r.Context(), db.UpdateCoachServicesParams{ProfileID: coach.ProfileID, ServiceTypes: services})
	if err != nil {
		MapDBError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"application": s.coachProfileJSON(updated)})
}

// 後続フェーズで提供管理が整うまでは大会・対面を販売しない。既存の分類なしプランは維持する。
func (s *Server) planServiceAllowed(w http.ResponseWriter, r *http.Request, q *db.Queries, coachID uuid.UUID, service *string, published bool) bool {
	if published && service != nil && *service == "video_review" {
		FieldError(w, map[string]string{"service_type": "動画添削は動画添削プランの指名設定へ移りました。コーチ画面で指名料を設定してください"})
		return false
	}
	if service == nil {
		return true
	}
	coach, err := q.GetCoachProfile(r.Context(), coachID)
	if err != nil {
		MapDBError(w, err)
		return false
	}
	if coach.Status != "approved" {
		Error(w, http.StatusForbidden, "forbidden", "承認されたコーチだけが使えます")
		return false
	}
	if !slices.Contains(coach.ServiceTypes, *service) {
		FieldError(w, map[string]string{"service_type": "先に担当するサービスとして登録してください"})
		return false
	}
	if published && *service == "in_person" && !pledgedCurrent(coach) {
		FieldError(w, map[string]string{"published": "対面指導を含む新しい誓約をしてから公開してください"})
		return false
	}
	return true
}
