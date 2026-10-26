package httpx

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5"
)

// パーソナル指導の誓約の文面の版。文面を変えたら上げる。上がると、コーチはもう一度誓約するまでプランが一覧に出ない。
const coachPledgeVersion = 2

// 再同意期間中も、前の版で誓約したコーチの既存プランは一覧に出す。
const coachPledgeListedVersion = 1

const (
	maxCoachingPlans = 10
	minPlanPriceYen  = 2000
	// 値段の上限は決めない（ひなたさん決定）。決済で扱える額までにして、桁の打ち間違いだけ防ぐ。
	maxPlanPriceYen = 9999999
	// これを超える値段は、打ち間違いでないかを1回確かめてから保存する。
	highPlanPriceYen = 30000
	maxPlanCapacity  = 50
	// お試しプランの値段の上限。月額プランの前に気軽に試せる値段にする。
	maxTrialPriceYen = 3000
	// 動画への返答の回数の上限。0 は回数を決めない（期間中いつでも）。
	maxVideoReviews = 30
)

// プランの「何が返ってくるか」で選べる値。DB の CHECK と合わせる。
var (
	planMenuUpdates  = []string{"none", "once", "weekly"}
	planReplyFormats = []string{"text", "video"}
)

func pledged(c db.CoachProfile) bool {
	return c.PledgeVersion != nil && *c.PledgeVersion >= coachPledgeListedVersion
}
func pledgedCurrent(c db.CoachProfile) bool {
	return c.PledgeVersion != nil && *c.PledgeVersion == coachPledgeVersion
}

func validPlanDuration(kind string, days int) bool {
	switch kind {
	case "period":
		return days == 14 || days == 30
	case "single", "trial":
		return days == 7
	}
	return false
}

func coachingPlanJSON(p db.CoachingPlan) map[string]any {
	return map[string]any{
		"id":               p.ID,
		"competition_name": p.CompetitionName, "competition_date": dateString(p.CompetitionDate), "support_start_date": dateString(p.SupportStartDate), "call_count": p.CallCount, "call_minutes": p.CallMinutes, "venue_area": p.VenueArea, "venue_name": p.VenueName, "session_count": p.SessionCount, "session_minutes": p.SessionMinutes, "travel_fee_yen": p.TravelFeeYen, "max_participants": p.MaxParticipants,
		"kind":                              p.Kind,
		"service_type":                      p.ServiceType,
		"additional_consultation_price_yen": p.AdditionalConsultationPriceYen,
		"title":                             p.Title,
		"description":                       p.Description,
		"duration_days":                     p.DurationDays,
		"price_yen":                         p.PriceYen,
		"capacity":                          p.Capacity,
		"published":                         p.Published,
		// 何が返ってくるか。null か空なら、コーチが書いていない。
		"video_reviews": p.VideoReviews,
		"menu_updates":  p.MenuUpdates,
		"reply_formats": nonNilStrings(p.ReplyFormats),
		// 期間を、最初の動画が届いた日から数えるか。
		"start_on_first_video": p.StartOnFirstVideo,
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
	if !s.requireStaffMFA(w, r) {
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
	supportPlanReq
	inPersonPlanReq
	ServiceType *string `json:"service_type"`
	// 未送信は以前の価格を維持し、明示的な null は設定を解除する。
	AdditionalConsultationPriceYen json.RawMessage `json:"additional_consultation_price_yen"`
	additionalPrice                *int32

	Kind             string `json:"kind"`
	Title            string `json:"title"`
	Description      string `json:"description"`
	DurationDays     *int   `json:"duration_days"`
	PriceYen         *int   `json:"price_yen"`
	Capacity         *int   `json:"capacity"`
	Published        *bool  `json:"published"`
	ConfirmHighPrice bool   `json:"confirm_high_price"`
	// 何が返ってくるか。どれも書かなくてよい。
	VideoReviews *int     `json:"video_reviews"`
	MenuUpdates  *string  `json:"menu_updates"`
	ReplyFormats []string `json:"reply_formats"`
	// 期間を、最初の動画が届いた日から数えるか。
	StartOnFirstVideo bool `json:"start_on_first_video"`
}

// includes は、「何が返ってくるか」を DB に入れる形にする。validate のあとで呼ぶ。
func (req *coachingPlanReq) includes() (*int16, *string, []string) {
	var reviews *int16
	if req.VideoReviews != nil {
		n := int16(*req.VideoReviews)
		reviews = &n
	}
	// 選んだ順ではなく、決まった順で持つ。
	formats := []string{}
	for _, f := range planReplyFormats {
		if slices.Contains(req.ReplyFormats, f) {
			formats = append(formats, f)
		}
	}
	return reviews, req.MenuUpdates, formats
}

// validate は入力を確かめる。prevPrice は直す前の値段（新しく作るときは 0）で、値段を変えないなら高い値段の確認はいらない。
func (req *coachingPlanReq) validate(prevPrice int) map[string]string {
	req.Title = strings.TrimSpace(req.Title)
	req.Description = strings.TrimSpace(req.Description)
	fields := map[string]string{}
	if req.ServiceType != nil && !slices.Contains(coachingServiceTypes, *req.ServiceType) {
		fields["service_type"] = "担当するサービスを選んでください"
	}
	if len(req.AdditionalConsultationPriceYen) > 0 {
		var price *int
		if err := json.Unmarshal(req.AdditionalConsultationPriceYen, &price); err != nil || (price != nil && (*price < 500 || *price > maxPlanPriceYen)) {
			fields["additional_consultation_price_yen"] = "追加相談の値段は500円以上で入力してください"
		} else if price != nil {
			n := int32(*price)
			req.additionalPrice = &n
		}
	}
	if req.Kind != "period" && req.Kind != "single" && req.Kind != "trial" {
		fields["kind"] = "プランの種類を選んでください"
	} else if req.DurationDays == nil || !(validPlanDuration(req.Kind, *req.DurationDays) || req.ServiceType != nil && *req.ServiceType == "competition_support" && req.Kind == "period" && *req.DurationDays >= 1 && *req.DurationDays <= 90) {
		switch req.Kind {
		case "single":
			fields["duration_days"] = "単発は7日です"
		case "trial":
			fields["duration_days"] = "お試しは7日です"
		default:
			fields["duration_days"] = "期間は2週間か1か月を選んでください"
		}
	}
	// プランはコーチ一覧で誰でも見るので、連絡先は書けない。
	if e := coachPublicTextError("プラン名", req.Title, 60); e != "" {
		fields["title"] = e
	}
	if e := coachPublicTextError("内容", req.Description, 1000); e != "" {
		fields["description"] = e
	}
	switch {
	case req.PriceYen == nil || *req.PriceYen < minPlanPriceYen:
		fields["price_yen"] = "値段は2,000円以上にしてください"
	case req.Kind == "trial" && *req.PriceYen > maxTrialPriceYen:
		fields["price_yen"] = "お試しの値段は2,000〜3,000円にしてください"
	case *req.PriceYen > maxPlanPriceYen:
		fields["price_yen"] = "値段が大きすぎます。桁を確かめてください"
	case *req.PriceYen > highPlanPriceYen && *req.PriceYen != prevPrice && !req.ConfirmHighPrice:
		fields["price_yen"] = "30,000円を超える値段です。打ち間違いでないか確かめてください"
	}
	if req.Capacity == nil || *req.Capacity < 1 || *req.Capacity > maxPlanCapacity {
		fields["capacity"] = "同時に受ける人数は1〜50人にしてください"
	}
	if req.VideoReviews != nil && (*req.VideoReviews < 0 || *req.VideoReviews > maxVideoReviews) {
		fields["video_reviews"] = "動画への返答の回数は30回までにしてください"
	}
	if req.MenuUpdates != nil && !slices.Contains(planMenuUpdates, *req.MenuUpdates) {
		fields["menu_updates"] = "メニューの作り方を選び直してください"
	}
	for _, f := range req.ReplyFormats {
		if !slices.Contains(planReplyFormats, f) {
			fields["reply_formats"] = "返し方を選び直してください"
		}
	}
	return fields
}

// noOtherTrialPlan は、このコーチにほかのお試しプランがないかを確かめる。あれば 409 を返して false。
func (s *Server) noOtherTrialPlan(w http.ResponseWriter, r *http.Request, q *db.Queries, coachID, exclude uuid.UUID) bool {
	n, err := q.CountOtherTrialPlans(r.Context(), db.CountOtherTrialPlansParams{CoachID: coachID, ExcludeID: exclude})
	if err != nil {
		MapDBError(w, err)
		return false
	}
	if n > 0 {
		Error(w, http.StatusConflict, "conflict", "お試しプランは1つまでです。いまのお試しプランを直して使ってください")
		return false
	}
	return true
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
	fields := req.supportFields(db.CoachingPlan{}, req.Published == nil || *req.Published)
	for k, v := range req.inPersonFields(db.CoachingPlan{}, req.Published == nil || *req.Published) {
		fields[k] = v
	}
	for k, v := range req.validate(0) {
		fields[k] = v
	}
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := s.q.WithTx(tx)
	if err := q.LockCoachingPlans(r.Context(), c.ProfileID); err != nil {
		MapDBError(w, err)
		return
	}
	if !s.planServiceAllowed(w, r, q, c.ProfileID, req.ServiceType, req.Published == nil || *req.Published) {
		return
	}
	n, err := q.CountCoachingPlans(r.Context(), c.ProfileID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if n >= maxCoachingPlans {
		Error(w, http.StatusConflict, "conflict", "プランは10個までです。使わないプランを直して使ってください")
		return
	}
	if req.Kind == "trial" && !s.noOtherTrialPlan(w, r, q, c.ProfileID, uuid.Nil) {
		return
	}
	published := true
	if req.Published != nil {
		published = *req.Published
	}
	reviews, menu, formats := req.includes()
	p, err := q.CreateCoachingPlan(r.Context(), db.CreateCoachingPlanParams{
		ID:                             uuid.New(),
		CoachID:                        c.ProfileID,
		Kind:                           req.Kind,
		Title:                          req.Title,
		Description:                    req.Description,
		DurationDays:                   int16(*req.DurationDays),
		PriceYen:                       int32(*req.PriceYen),
		Capacity:                       int16(*req.Capacity),
		Published:                      published,
		VideoReviews:                   reviews,
		MenuUpdates:                    menu,
		ReplyFormats:                   formats,
		StartOnFirstVideo:              req.StartOnFirstVideo,
		ServiceType:                    req.ServiceType,
		AdditionalConsultationPriceYen: req.additionalPrice,
	})
	if err != nil {
		MapDBError(w, err)
		return
	}
	p, ok = s.saveSupportPlan(w, r, q, req, p)
	if !ok {
		return
	}
	p, ok = s.saveInPersonPlan(w, r, q, req, p)
	if !ok {
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
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

	// 担当サービスの変更・プランの作成と直列化する。
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := s.q.WithTx(tx)
	if err := q.LockCoachingPlans(r.Context(), c.ProfileID); err != nil {
		MapDBError(w, err)
		return
	}
	// 省略された設定は、ロック取得後の最新値を維持する。
	current, err = q.GetCoachingPlan(r.Context(), db.GetCoachingPlanParams{ID: id, CoachID: c.ProfileID})
	if err != nil {
		MapDBError(w, err)
		return
	}
	if req.ServiceType == nil {
		req.ServiceType = current.ServiceType
	}
	if len(req.AdditionalConsultationPriceYen) == 0 {
		req.additionalPrice = current.AdditionalConsultationPriceYen
	}
	published := current.Published
	if req.Published != nil {
		published = *req.Published
	}
	fields := req.supportFields(current, published)
	for k, v := range req.inPersonFields(current, published) {
		fields[k] = v
	}
	for k, v := range req.validate(int(current.PriceYen)) {
		fields[k] = v
	}
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	if published && !current.Published && !pledged(c) {
		Error(w, http.StatusConflict, "pledge_required", "先にパーソナル指導の誓約をしてください")
		return
	}
	if !s.planServiceAllowed(w, r, q, c.ProfileID, req.ServiceType, published) {
		return
	}
	if req.Kind == "trial" && current.Kind != "trial" {
		if !s.noOtherTrialPlan(w, r, q, c.ProfileID, id) {
			return
		}
	}
	reviews, menu, formats := req.includes()
	p, err := q.UpdateCoachingPlan(r.Context(), db.UpdateCoachingPlanParams{
		ID:                             id,
		CoachID:                        c.ProfileID,
		Kind:                           req.Kind,
		Title:                          req.Title,
		Description:                    req.Description,
		DurationDays:                   int16(*req.DurationDays),
		PriceYen:                       int32(*req.PriceYen),
		Capacity:                       int16(*req.Capacity),
		Published:                      published,
		VideoReviews:                   reviews,
		MenuUpdates:                    menu,
		ReplyFormats:                   formats,
		StartOnFirstVideo:              req.StartOnFirstVideo,
		ServiceType:                    req.ServiceType,
		AdditionalConsultationPriceYen: req.additionalPrice,
	})
	if err != nil {
		MapDBError(w, err)
		return
	}
	p, ok = s.saveSupportPlan(w, r, q, req, p)
	if !ok {
		return
	}
	p, ok = s.saveInPersonPlan(w, r, q, req, p)
	if !ok {
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"plan": coachingPlanJSON(p)})
}
