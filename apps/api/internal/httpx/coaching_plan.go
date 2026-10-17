package httpx

import (
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5"
)

// パーソナル指導の誓約の文面の版。文面を変えたら上げる。上がると、コーチはもう一度誓約するまでプランが一覧に出ない。
const coachPledgeVersion = 1

const (
	maxCoachingPlans = 10
	minPlanPriceYen  = 2000
	// 値段の上限は決めない（ひなたさん決定）。決済で扱える額までにして、桁の打ち間違いだけ防ぐ。
	maxPlanPriceYen = 9999999
	// これを超える値段は、打ち間違いでないかを1回確かめてから保存する。
	highPlanPriceYen = 30000
	maxPlanCapacity  = 50
)

func pledged(c db.CoachProfile) bool {
	return c.PledgeVersion != nil && *c.PledgeVersion == coachPledgeVersion
}

func validPlanDuration(kind string, days int) bool {
	switch kind {
	case "period":
		return days == 14 || days == 30
	case "single":
		return days == 7
	}
	return false
}

func coachingPlanJSON(p db.CoachingPlan) map[string]any {
	return map[string]any{
		"id":            p.ID,
		"kind":          p.Kind,
		"title":         p.Title,
		"description":   p.Description,
		"duration_days": p.DurationDays,
		"price_yen":     p.PriceYen,
		"capacity":      p.Capacity,
		"published":     p.Published,
	}
}

// approvedCoach は、承認済みのコーチ本人かを確かめる。違えば 403 を返して false。
func (s *Server) approvedCoach(w http.ResponseWriter, r *http.Request) (db.CoachProfile, bool) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return db.CoachProfile{}, false
	}
	c, err := s.q.GetCoachProfile(r.Context(), p.ID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && c.Status != "approved") {
		Error(w, http.StatusForbidden, "forbidden", "承認されたコーチだけが使えます")
		return db.CoachProfile{}, false
	}
	if err != nil {
		MapDBError(w, err)
		return db.CoachProfile{}, false
	}
	return c, true
}

// pledgeCoach は、パーソナル指導の誓約を記録する。
func (s *Server) pledgeCoach(w http.ResponseWriter, r *http.Request) {
	coach, ok := s.approvedCoach(w, r)
	if !ok {
		return
	}
	var req struct {
		Agree bool `json:"agree"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	if !req.Agree {
		FieldError(w, map[string]string{"agree": "誓約の内容を確かめて、チェックを入れてください"})
		return
	}
	c, err := s.q.PledgeCoach(r.Context(), db.PledgeCoachParams{PledgeVersion: coachPledgeVersion, ProfileID: coach.ProfileID})
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusForbidden, "forbidden", "承認されたコーチだけが使えます")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"application": s.coachProfileJSON(c)})
}

type coachingPlanReq struct {
	Kind             string `json:"kind"`
	Title            string `json:"title"`
	Description      string `json:"description"`
	DurationDays     *int   `json:"duration_days"`
	PriceYen         *int   `json:"price_yen"`
	Capacity         *int   `json:"capacity"`
	Published        *bool  `json:"published"`
	ConfirmHighPrice bool   `json:"confirm_high_price"`
}

// validate は入力を確かめる。prevPrice は直す前の値段（新しく作るときは 0）で、値段を変えないなら高い値段の確認はいらない。
func (req *coachingPlanReq) validate(prevPrice int) map[string]string {
	req.Title = strings.TrimSpace(req.Title)
	req.Description = strings.TrimSpace(req.Description)
	fields := map[string]string{}
	if req.Kind != "period" && req.Kind != "single" {
		fields["kind"] = "プランの種類を選んでください"
	} else if req.DurationDays == nil || !validPlanDuration(req.Kind, *req.DurationDays) {
		if req.Kind == "single" {
			fields["duration_days"] = "単発は7日です"
		} else {
			fields["duration_days"] = "期間は2週間か1か月を選んでください"
		}
	}
	if e := coachTextError("プラン名", req.Title, 60); e != "" {
		fields["title"] = e
	}
	if e := coachTextError("内容", req.Description, 1000); e != "" {
		fields["description"] = e
	}
	switch {
	case req.PriceYen == nil || *req.PriceYen < minPlanPriceYen:
		fields["price_yen"] = "値段は2,000円以上にしてください"
	case *req.PriceYen > maxPlanPriceYen:
		fields["price_yen"] = "値段が大きすぎます。桁を確かめてください"
	case *req.PriceYen > highPlanPriceYen && *req.PriceYen != prevPrice && !req.ConfirmHighPrice:
		fields["price_yen"] = "30,000円を超える値段です。打ち間違いでないか確かめてください"
	}
	if req.Capacity == nil || *req.Capacity < 1 || *req.Capacity > maxPlanCapacity {
		fields["capacity"] = "同時に受ける人数は1〜50人にしてください"
	}
	return fields
}

func (s *Server) listMyCoachingPlans(w http.ResponseWriter, r *http.Request) {
	c, ok := s.approvedCoach(w, r)
	if !ok {
		return
	}
	plans, err := s.q.ListMyCoachingPlans(r.Context(), c.ProfileID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(plans))
	for _, p := range plans {
		out = append(out, coachingPlanJSON(p))
	}
	JSON(w, http.StatusOK, map[string]any{"plans": out})
}

func (s *Server) createCoachingPlan(w http.ResponseWriter, r *http.Request) {
	c, ok := s.approvedCoach(w, r)
	if !ok {
		return
	}
	if !pledged(c) {
		Error(w, http.StatusConflict, "pledge_required", "先にパーソナル指導の誓約をしてください")
		return
	}
	var req coachingPlanReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	if fields := req.validate(0); len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	n, err := s.q.CountCoachingPlans(r.Context(), c.ProfileID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if n >= maxCoachingPlans {
		Error(w, http.StatusConflict, "conflict", "プランは10個までです。使わないプランを直して使ってください")
		return
	}
	published := true
	if req.Published != nil {
		published = *req.Published
	}
	p, err := s.q.CreateCoachingPlan(r.Context(), db.CreateCoachingPlanParams{
		ID:           uuid.New(),
		CoachID:      c.ProfileID,
		Kind:         req.Kind,
		Title:        req.Title,
		Description:  req.Description,
		DurationDays: int16(*req.DurationDays),
		PriceYen:     int32(*req.PriceYen),
		Capacity:     int16(*req.Capacity),
		Published:    published,
	})
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusCreated, map[string]any{"plan": coachingPlanJSON(p)})
}

func (s *Server) updateCoachingPlan(w http.ResponseWriter, r *http.Request) {
	c, ok := s.approvedCoach(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	current, err := s.q.GetCoachingPlan(r.Context(), db.GetCoachingPlanParams{ID: id, CoachID: c.ProfileID})
	if err != nil {
		MapDBError(w, err)
		return
	}
	var req coachingPlanReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	if fields := req.validate(int(current.PriceYen)); len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	published := current.Published
	if req.Published != nil {
		published = *req.Published
	}
	if published && !current.Published && !pledged(c) {
		Error(w, http.StatusConflict, "pledge_required", "先にパーソナル指導の誓約をしてください")
		return
	}
	p, err := s.q.UpdateCoachingPlan(r.Context(), db.UpdateCoachingPlanParams{
		ID:           id,
		CoachID:      c.ProfileID,
		Kind:         req.Kind,
		Title:        req.Title,
		Description:  req.Description,
		DurationDays: int16(*req.DurationDays),
		PriceYen:     int32(*req.PriceYen),
		Capacity:     int16(*req.Capacity),
		Published:    published,
	})
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"plan": coachingPlanJSON(p)})
}
