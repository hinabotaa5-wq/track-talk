package httpx

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/push"
	"github.com/jackc/pgx/v5"
)

// コーチの担当種目。アプリ内の種目(appEvents)と同じ。並びは画面に出す順。
var coachEvents = func() []string {
	out := make([]string, 0, len(appEvents))
	for _, event := range appEvents {
		out = append(out, event.Code)
	}
	return out
}()

func validCoachTier(tier string) bool {
	return tier == "coach"
}

func validCoachStatus(status string) bool {
	switch status {
	case "pending", "approved", "rejected", "suspended", "withdrawn":
		return true
	}
	return false
}

// normalizeCoachEvents は、重複を除いて画面と同じ順に並べ直す。知らない値があれば false。
func normalizeCoachEvents(events []string) ([]string, bool) {
	picked := map[string]bool{}
	for _, e := range events {
		picked[e] = true
	}
	out := make([]string, 0, len(picked))
	for _, g := range coachEvents {
		if picked[g] {
			out = append(out, g)
			delete(picked, g)
		}
	}
	return out, len(picked) == 0
}

func coachTextError(label, value string, max int) string {
	n := utf8.RuneCountInString(value)
	if n == 0 {
		return label + "を入力してください"
	}
	if n > max {
		return label + "は" + strconv.Itoa(max) + "文字以内で入力してください"
	}
	return ""
}

// coachPublicTextError は、コーチ一覧で誰でも見る欄を確かめる。coachTextError に加えて、連絡先を書かせない。
func coachPublicTextError(label, value string, max int) string {
	if e := coachTextError(label, value, max); e != "" {
		return e
	}
	if looksLikeContact(value) {
		return "URL・メールアドレス・電話番号・SNS の ID など、連絡先らしきものは書けません"
	}
	return ""
}

func (s *Server) coachProfileJSON(c db.CoachProfile) map[string]any {
	return map[string]any{
		"name":               c.Name,
		"tier":               c.Tier,
		"events":             c.Events,
		"service_types":      nonNilStrings(c.ServiceTypes),
		"best_record":        c.BestRecord,
		"achievements":       c.Achievements,
		"years":              c.Years,
		"sns_urls":           nonNilStrings(c.SnsUrls),
		"status":             c.Status,
		"applied_at":         c.AppliedAt,
		"reviewed_at":        c.ReviewedAt,
		"on_duty":            c.OnDuty,
		"daily_cap":          c.DailyCap,
		"nomination_fee_yen": c.NominationFeeYen,
		"photo_url":          s.coachPhotoURL(c.PhotoPublicID),
		// いまの版のパーソナル指導の誓約をしたか。
		"pledged": pledged(c), "pledged_current": pledgedCurrent(c), "pledge_version": c.PledgeVersion,
		// 報酬の受け取り口座（Stripe）の登録が済み、受付やパーソナル指導の承諾ができるか。
		"payouts_ready": s.payoutsReady(c),
	}
}

// coachPhotoURL は、コーチの顔写真の URL。登録していなければ空。
func (s *Server) coachPhotoURL(publicID *string) string {
	if publicID == nil {
		return ""
	}
	return s.cloud.AvatarURL(*publicID)
}

type coachPhotoReq struct {
	PhotoPublicID string `json:"photo_public_id"`
}

// setCoachPhoto は、応募中か承認済みのコーチの顔写真を差し替える。写真は /v1/uploads/image で先に上げておく。
func (s *Server) setCoachPhoto(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	var req coachPhotoReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	id := strings.TrimSpace(req.PhotoPublicID)
	if e := ImagePublicID(id); e != "" {
		FieldError(w, map[string]string{"photo_public_id": e})
		return
	}
	if !s.uploadIssuedToMe(w, r, p.ID, id, "photo_public_id", "写真") {
		return
	}
	if err := s.cloud.ImageReady(r.Context(), id); err != nil {
		s.purgeRejectedUpload(err, "image", id)
		writeAssetReadyError(w, err, "写真")
		return
	}
	old, err := s.q.SetCoachPhoto(r.Context(), db.SetCoachPhotoParams{PhotoPublicID: id, ProfileID: p.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusConflict, "conflict", "応募してから写真を登録してください")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	// 前の写真が他人の投稿などの写真だったら消さない（他人の写真の ID を登録してから差し替える手を防ぐ）。
	if old != nil && *old != id {
		s.deleteUnusedMedia(r.Context(), postMedia{Images: []string{*old}})
	}
	c, err := s.q.GetCoachProfile(r.Context(), p.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"application": s.coachProfileJSON(c)})
}

// coachMe は自分の応募を返す。まだ応募していなければ application は null。
func (s *Server) coachMe(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	c, err := s.q.GetCoachProfile(r.Context(), p.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		JSON(w, http.StatusOK, map[string]any{"application": nil})
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	rating, err := s.q.GetCoachRatingSummary(r.Context(), p.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{
		"application": s.coachProfileJSON(c),
		"rating": map[string]any{
			"count":   rating.Count,
			"average": rating.Average,
			// 評価が低く、いまは依頼が回ってこない状態か。
			"paused": lowRated(rating.Count, rating.Average),
		},
	})
}

// lowRated は、評価が低くて打診の順番から外れているか。PickCoachForRequest と同じ基準。
func lowRated(count int32, average float64) bool {
	return count >= lowRatingMinCount && average < lowRatingStars
}

type coachApplicationReq struct {
	Name         *string  `json:"name"`
	Tier         string   `json:"tier"`
	Events       []string `json:"events"`
	BestRecord   string   `json:"best_record"`
	Achievements string   `json:"achievements"`
	Years        *int     `json:"years"`
	SnsUrls      []string `json:"sns_urls"`
}

// normalizeSnsUrls は、チームや個人の SNS の URL を整える。空行は除き、重複は1つにする。
// http/https の公開ページ URL だけを通し、それ以外が混ざれば false。
func normalizeSnsUrls(urls []string) ([]string, bool) {
	out := make([]string, 0, len(urls))
	seen := map[string]bool{}
	for _, raw := range urls {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		if len(v) > 300 || len(out) >= 3 || seen[v] {
			if seen[v] {
				continue
			}
			return nil, false
		}
		u, err := url.Parse(v)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil {
			return nil, false
		}
		seen[v] = true
		out = append(out, v)
	}
	return out, true
}

func (s *Server) submitCoachApplication(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	var req coachApplicationReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	name := ""
	if req.Name != nil {
		name = strings.TrimSpace(*req.Name)
	}
	req.BestRecord = strings.TrimSpace(req.BestRecord)
	req.Achievements = strings.TrimSpace(req.Achievements)
	fields := map[string]string{}
	if e := coachPublicTextError("名前", name, 80); e != "" {
		fields["name"] = e
	}
	if !validCoachTier(req.Tier) {
		fields["tier"] = "応募する枠を選んでください"
	}
	events, known := normalizeCoachEvents(req.Events)
	if !known || len(events) == 0 {
		fields["events"] = "担当種目を1つ以上選んでください"
	}
	if e := coachPublicTextError("記録", req.BestRecord, 200); e != "" {
		fields["best_record"] = e
	}
	if e := coachPublicTextError("成績や指導経験", req.Achievements, 500); e != "" {
		fields["achievements"] = e
	}
	snsUrls, snsOK := normalizeSnsUrls(req.SnsUrls)
	if !snsOK {
		fields["sns_urls"] = "SNS・ホームページのURLは3件までで、http://またはhttps://から始まるURLを入力してください"
	} else if len(snsUrls) == 0 {
		fields["sns_urls"] = "SNSまたはホームページのURLを1件以上入力してください"
	}
	if req.Years == nil {
		years := 0
		req.Years = &years
	}
	if *req.Years < 0 || *req.Years > 60 {
		fields["years"] = "競技年数は0〜60年で入力してください"
	}
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	c, err := s.q.SubmitCoachApplication(r.Context(), db.SubmitCoachApplicationParams{
		ProfileID:    p.ID,
		Name:         name,
		Tier:         req.Tier,
		Events:       events,
		BestRecord:   req.BestRecord,
		Achievements: req.Achievements,
		Years:        int16(*req.Years),
		SnsUrls:      snsUrls,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusConflict, "conflict", "審査が済んでいるため、応募内容は変えられません")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	if err := s.q.ReopenCoachLegalIdentity(r.Context(), p.ID); err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"application": s.coachProfileJSON(c)})
}

func (s *Server) listCoachApplications(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "pending"
	}
	if !validCoachStatus(status) {
		FieldError(w, map[string]string{"status": "状態が不正です"})
		return
	}
	page, per, offset := s.queryPage(r)
	total, err := s.q.CountCoachApplications(r.Context(), status)
	if err != nil {
		MapDBError(w, err)
		return
	}
	rows, err := s.q.ListCoachApplications(r.Context(), db.ListCoachApplicationsParams{Status: status, Limit: per, Offset: offset})
	if err != nil {
		MapDBError(w, err)
		return
	}
	type application struct {
		db.ListCoachApplicationsRow
		// 評価が低くて打診の順番から外れているか。
		RatingPaused bool   `json:"rating_paused"`
		PhotoURL     string `json:"photo_url"`
	}
	out := make([]application, 0, len(rows))
	for _, row := range rows {
		out = append(out, application{row, lowRated(row.RatingCount, row.RatingAverage), s.coachPhotoURL(row.PhotoPublicID)})
	}
	JSON(w, http.StatusOK, map[string]any{"applications": out, "meta": meta{Page: int(page), PerPage: int(per), Total: total}})
}

// resetCoachRating は、評価で打診の順番から外れたコーチを戻す。それより前の★は外す判定に数えなくなる。
func (s *Server) resetCoachRating(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	c, err := s.q.ResetCoachRating(r.Context(), id)
	if err != nil {
		MapDBError(w, err)
		return
	}
	// 待ちの依頼があれば、戻したコーチにも回す。
	s.dispatchQueuedReviews(r.Context())
	JSON(w, http.StatusOK, map[string]any{"application": s.coachProfileJSON(c)})
}

// removeCoachPhoto は、運営がコーチの顔写真を外す。写真は Cloudinary からも消す。コーチはまた別の写真を登録できる。
func (s *Server) removeCoachPhoto(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	old, err := s.q.RemoveCoachPhoto(r.Context(), id)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if old != nil {
		s.deleteUnusedMedia(r.Context(), postMedia{Images: []string{*old}})
	}
	w.WriteHeader(http.StatusNoContent)
}

// listCoaches は、依頼する前に見られるコーチ一覧。ログインしなくても見られる。
func (s *Server) listCoaches(w http.ResponseWriter, r *http.Request) {
	rows, err := s.q.ListPublicCoaches(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	plans, err := s.q.ListPublicCoachingPlans(r.Context(), coachPledgeListedVersion)
	if err != nil {
		MapDBError(w, err)
		return
	}
	plansByCoach := map[uuid.UUID][]map[string]any{}
	for _, p := range plans {
		plan := coachingPlanJSON(db.CoachingPlan{
			ID: p.ID, CoachID: p.CoachID, Kind: p.Kind, Title: p.Title, Description: p.Description,
			DurationDays: p.DurationDays, PriceYen: p.PriceYen, Capacity: p.Capacity, Published: p.Published,
			VideoReviews: p.VideoReviews, MenuUpdates: p.MenuUpdates, ReplyFormats: p.ReplyFormats,
			StartOnFirstVideo: p.StartOnFirstVideo,
			CompetitionName:   p.CompetitionName, CompetitionDate: p.CompetitionDate, SupportStartDate: p.SupportStartDate, CallCount: p.CallCount, CallMinutes: p.CallMinutes,
			VenueArea: p.VenueArea, VenueName: p.VenueName, SessionCount: p.SessionCount, SessionMinutes: p.SessionMinutes, TravelFeeYen: p.TravelFeeYen, MaxParticipants: p.MaxParticipants,
			ServiceType: p.ServiceType, AdditionalConsultationPriceYen: p.AdditionalConsultationPriceYen,
		})
		plan["full"] = p.ActiveCount >= int32(p.Capacity)
		plansByCoach[p.CoachID] = append(plansByCoach[p.CoachID], plan)
	}
	out := make([]map[string]any, 0, len(rows))
	for _, c := range rows {
		out = append(out, map[string]any{
			"id":             c.ProfileID,
			"nickname":       c.Nickname,
			"photo_url":      s.coachPhotoURL(c.PhotoPublicID),
			"tier":           c.Tier,
			"events":         c.Events,
			"service_types":  nonNilStrings(c.ServiceTypes),
			"best_record":    c.BestRecord,
			"achievements":   c.Achievements,
			"years":          c.Years,
			"sns_urls":       nonNilStrings(c.SnsUrls),
			"rating_count":   c.RatingCount,
			"rating_average": c.RatingAverage,
			// パーソナル指導のプラン。安い順。
			"plans": nonNilPlans(plansByCoach[c.ProfileID]),
		})
	}
	JSON(w, http.StatusOK, map[string]any{"coaches": out})
}

func nonNilPlans(p []map[string]any) []map[string]any {
	if p == nil {
		return []map[string]any{}
	}
	return p
}

func (s *Server) approveCoachApplication(w http.ResponseWriter, r *http.Request) {
	s.reviewCoachApplication(w, r, "approved")
}

func (s *Server) rejectCoachApplication(w http.ResponseWriter, r *http.Request) {
	s.reviewCoachApplication(w, r, "rejected")
}

func (s *Server) reviewCoachApplication(w http.ResponseWriter, r *http.Request, status string) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	c, err := s.q.ReviewCoachApplication(r.Context(), db.ReviewCoachApplicationParams{Status: status, ProfileID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		current, getErr := s.q.GetCoachProfile(r.Context(), id)
		if errors.Is(getErr, pgx.ErrNoRows) {
			Error(w, http.StatusNotFound, "not_found", "応募が見つかりません")
			return
		}
		if getErr == nil && current.Status == "pending" && current.PhotoPublicID == nil {
			Error(w, http.StatusConflict, "conflict", "顔写真がまだ登録されていないため、承認できません")
			return
		}
		Error(w, http.StatusConflict, "conflict", "この応募は審査済みです")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	if c.Status == "rejected" {
		if err := s.q.EndCoachLegalIdentity(r.Context(), id); err != nil {
			MapDBError(w, err)
			return
		}
	}
	s.notifyCoachReviewed(r.Context(), c)
	JSON(w, http.StatusOK, map[string]any{"application": s.coachProfileJSON(c)})
}

// notifyCoachReviewed は、審査の結果を応募した人に知らせる。
// 届かなくても結果は応募の画面で見られるので、失敗は記録だけする。
func (s *Server) notifyCoachReviewed(ctx context.Context, c db.CoachProfile) {
	msg := push.Message{Title: "コーチの応募が承認されました", Body: "応募の画面から次の準備に進めます", URL: "/coach/apply", Tag: "coach-review"}
	if c.Status == "rejected" {
		msg = push.Message{Title: "コーチの応募の審査が終わりました", Body: "結果を応募の画面で確認してください", URL: "/coach/apply", Tag: "coach-review"}
	}
	s.notify(ctx, c.ProfileID, notifyCoaching, msg, 24*time.Hour)
}
