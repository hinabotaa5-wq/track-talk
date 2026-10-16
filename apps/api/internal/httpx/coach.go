package httpx

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/push"
	"github.com/jackc/pgx/v5"
)

// コーチの担当種目。並びは画面に出す順。コードは練習相手募集の種目に合わせている。
var coachEvents = []string{
	"100m",
	"200m",
	"400m",
	"800m",
	"1500m",
	"5000m",
	"10000m",
	"110mh",
	"100mh",
	"400mh",
	"3000msc",
	"high-jump",
	"pole-vault",
	"long-jump",
	"triple-jump",
	"shot-put",
	"discus",
	"hammer",
	"javelin",
	"race-walk",
	"relay",
}

func validCoachTier(tier string) bool {
	return tier == "coach" || tier == "senpai"
}

func validCoachStatus(status string) bool {
	switch status {
	case "pending", "approved", "rejected", "suspended":
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

func (s *Server) coachProfileJSON(c db.CoachProfile) map[string]any {
	return map[string]any{
		"tier":         c.Tier,
		"events":       c.Events,
		"best_record":  c.BestRecord,
		"achievements": c.Achievements,
		"years":        c.Years,
		"status":       c.Status,
		"applied_at":   c.AppliedAt,
		"reviewed_at":  c.ReviewedAt,
		"on_duty":      c.OnDuty,
		"daily_cap":    c.DailyCap,
		"photo_url":    s.coachPhotoURL(c.PhotoPublicID),
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
	if err := s.cloud.ImageReady(r.Context(), id); err != nil {
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
	if old != nil && *old != id {
		if err := s.cloud.Delete(r.Context(), "image", []string{*old}); err != nil {
			slog.Warn("delete old coach photo", "err", err)
		}
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
	Tier         string   `json:"tier"`
	Events       []string `json:"events"`
	BestRecord   string   `json:"best_record"`
	Achievements string   `json:"achievements"`
	Years        *int     `json:"years"`
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
	req.BestRecord = strings.TrimSpace(req.BestRecord)
	req.Achievements = strings.TrimSpace(req.Achievements)
	fields := map[string]string{}
	if !validCoachTier(req.Tier) {
		fields["tier"] = "応募する枠を選んでください"
	}
	events, known := normalizeCoachEvents(req.Events)
	if !known || len(events) == 0 {
		fields["events"] = "担当種目を1つ以上選んでください"
	}
	if e := coachTextError("記録", req.BestRecord, 200); e != "" {
		fields["best_record"] = e
	}
	if e := coachTextError("成績", req.Achievements, 500); e != "" {
		fields["achievements"] = e
	}
	if req.Years == nil || *req.Years < 0 || *req.Years > 60 {
		fields["years"] = "競技年数は0〜60年で入力してください"
	}
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	c, err := s.q.SubmitCoachApplication(r.Context(), db.SubmitCoachApplicationParams{
		ProfileID:    p.ID,
		Tier:         req.Tier,
		Events:       events,
		BestRecord:   req.BestRecord,
		Achievements: req.Achievements,
		Years:        int16(*req.Years),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusConflict, "conflict", "審査が済んでいるため、応募内容は変えられません")
		return
	}
	if err != nil {
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
		if err := s.cloud.Delete(r.Context(), "image", []string{*old}); err != nil {
			slog.Warn("delete removed coach photo", "err", err)
		}
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
	out := make([]map[string]any, 0, len(rows))
	for _, c := range rows {
		out = append(out, map[string]any{
			"id":             c.ProfileID,
			"nickname":       c.Nickname,
			"photo_url":      s.coachPhotoURL(c.PhotoPublicID),
			"tier":           c.Tier,
			"events":         c.Events,
			"best_record":    c.BestRecord,
			"achievements":   c.Achievements,
			"years":          c.Years,
			"rating_count":   c.RatingCount,
			"rating_average": c.RatingAverage,
		})
	}
	JSON(w, http.StatusOK, map[string]any{"coaches": out})
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
		if _, getErr := s.q.GetCoachProfile(r.Context(), id); errors.Is(getErr, pgx.ErrNoRows) {
			Error(w, http.StatusNotFound, "not_found", "応募が見つかりません")
			return
		}
		Error(w, http.StatusConflict, "conflict", "この応募は審査済みです")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	s.notifyCoachReviewed(r.Context(), c)
	JSON(w, http.StatusOK, map[string]any{"application": s.coachProfileJSON(c)})
}

// notifyCoachReviewed は、審査の結果を応募した人の端末に知らせる。
// 届かなくても結果は応募の画面で見られるので、失敗は記録だけする。
func (s *Server) notifyCoachReviewed(ctx context.Context, c db.CoachProfile) {
	if !s.push.Configured() {
		return
	}
	msg := push.Message{Title: "コーチの応募が承認されました", Body: "応募の画面から次の準備に進めます", URL: "/coach/apply", Tag: "coach-review"}
	if c.Status == "rejected" {
		msg = push.Message{Title: "コーチの応募の審査が終わりました", Body: "結果を応募の画面で確認してください", URL: "/coach/apply", Tag: "coach-review"}
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if _, err := s.sendPush(ctx, c.ProfileID, msg, 24*time.Hour); err != nil {
		slog.Warn("coach review push", "err", err)
	}
}
