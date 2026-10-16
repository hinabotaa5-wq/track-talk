package httpx

import (
	"crypto/rand"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/cloudinary"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// おまかせ添削の依頼（決済なしのデモ版）。
// 決済ができるまでは、運営が配った招待コードを入れた人（profiles.coaching_demo）だけが依頼できる。

// 見てほしい点。並びは画面に出す順。
var reviewFocusOptions = []string{"start", "acceleration", "top-speed", "takeoff", "arm", "contact", "rhythm", "other"}

const (
	defaultDailyCap = 5
	maxDailyCap     = 20
)

// 招待コードに使う文字。読み違えやすい 0/O と 1/I は外す。
const demoCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func validCoachEvent(event string) bool {
	for _, e := range coachEvents {
		if e == event {
			return true
		}
	}
	return false
}

// normalizeFocus は、重複を除いて画面と同じ順に並べ直す。知らない値があれば false。
func normalizeFocus(focus []string) ([]string, bool) {
	picked := map[string]bool{}
	for _, f := range focus {
		picked[f] = true
	}
	out := make([]string, 0, len(picked))
	for _, f := range reviewFocusOptions {
		if picked[f] {
			out = append(out, f)
			delete(picked, f)
		}
	}
	return out, len(picked) == 0
}

// reviewDueAt は、受付中のコーチがいないときの返却の目安。設計書の既定どおり、送った翌日の12時（日本時間）にする。
func reviewDueAt(now time.Time) time.Time {
	local := now.In(tokyo)
	return time.Date(local.Year(), local.Month(), local.Day()+1, 12, 0, 0, 0, tokyo)
}

// reviewWaiting は、まだ返却を待っている状態か。
func reviewWaiting(status string) bool {
	return status == "queued" || status == "offered" || status == "in_review"
}

func newDemoCode() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, len(buf))
	for i, b := range buf {
		out[i] = demoCodeAlphabet[int(b)%len(demoCodeAlphabet)]
	}
	return string(out), nil
}

func (s *Server) reviewRequestJSON(rr db.ReviewRequest) map[string]any {
	return map[string]any{
		"id":            rr.ID,
		"tier":          rr.Tier,
		"event":         rr.Event,
		"focus":         rr.Focus,
		"status":        rr.Status,
		"due_at":        rr.DueAt,
		"created_at":    rr.CreatedAt,
		"video_url":     s.cloud.CoachingPlaybackURL(rr.VideoPublicID),
		"thumbnail_url": s.cloud.CoachingThumbnailURL(rr.VideoPublicID),
	}
}

type coachDutyReq struct {
	OnDuty   *bool `json:"on_duty"`
	DailyCap *int  `json:"daily_cap"`
}

// updateCoachDuty は、承認済みのコーチの「受付中」と1日の上限本数を変える。
// 依頼の打診は Web プッシュで届くので、通知を受け取れる端末がないと受付中にはできない。
func (s *Server) updateCoachDuty(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	var req coachDutyReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	current, err := s.q.GetCoachProfile(r.Context(), p.ID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && current.Status != "approved") {
		Error(w, http.StatusForbidden, "forbidden", "承認されたコーチだけが受付を設定できます")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	onDuty := current.OnDuty
	if req.OnDuty != nil {
		onDuty = *req.OnDuty
	}
	dailyCap := int(current.DailyCap)
	if req.DailyCap != nil {
		dailyCap = *req.DailyCap
	}
	if dailyCap < 1 || dailyCap > maxDailyCap {
		FieldError(w, map[string]string{"daily_cap": "1日の上限は1〜20本で選んでください"})
		return
	}
	if onDuty && !current.OnDuty {
		devices, err := s.q.CountPushSubscriptions(r.Context(), p.ID)
		if err != nil {
			MapDBError(w, err)
			return
		}
		if devices == 0 {
			Error(w, http.StatusUnprocessableEntity, "validation_error", "依頼は通知で届くので、先にこの端末で通知を受け取れるようにしてください")
			return
		}
	}
	c, err := s.q.UpdateCoachDuty(r.Context(), db.UpdateCoachDutyParams{OnDuty: onDuty, DailyCap: int16(dailyCap), ProfileID: p.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusForbidden, "forbidden", "承認されたコーチだけが受付を設定できます")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	// 受付を始めたり上限を上げたりしたら、待っている依頼をすぐ回す。
	if c.OnDuty && (!current.OnDuty || c.DailyCap > current.DailyCap) {
		s.dispatchQueuedReviews(r.Context())
	}
	JSON(w, http.StatusOK, map[string]any{"application": coachProfileJSON(c)})
}

type demoCodeReq struct {
	Code string `json:"code"`
}

// joinCoachingDemo は、運営が配った招待コードでデモの参加者になる。
func (s *Server) joinCoachingDemo(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	var req demoCodeReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	if p.CoachingDemo {
		JSON(w, http.StatusOK, map[string]any{"demo": true})
		return
	}
	code := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(req.Code), "-", ""))
	if len(code) != 8 {
		FieldError(w, map[string]string{"code": "招待コードは8文字です"})
		return
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := s.q.WithTx(tx)
	if _, err := q.UseDemoCode(r.Context(), db.UseDemoCodeParams{Code: code, UsedBy: &p.ID}); errors.Is(err, pgx.ErrNoRows) {
		FieldError(w, map[string]string{"code": "この招待コードは使えません"})
		return
	} else if err != nil {
		MapDBError(w, err)
		return
	}
	if err := q.SetCoachingDemo(r.Context(), p.ID); err != nil {
		MapDBError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"demo": true})
}

type reviewRequestReq struct {
	Tier          string   `json:"tier"`
	Event         string   `json:"event"`
	Focus         []string `json:"focus"`
	VideoPublicID string   `json:"video_public_id"`
}

func (s *Server) createReviewRequest(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	if !p.CoachingDemo {
		Error(w, http.StatusForbidden, "forbidden", "フォーム添削は準備中です")
		return
	}
	var req reviewRequestReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	fields := map[string]string{}
	if !validCoachTier(req.Tier) {
		fields["tier"] = "枠を選んでください"
	}
	if !validCoachEvent(req.Event) {
		fields["event"] = "種目を選んでください"
	}
	focus, known := normalizeFocus(req.Focus)
	if !known || len(focus) == 0 {
		fields["focus"] = "見てほしい点を1つ以上選んでください"
	}
	if !cloudinary.ValidCoachingPublicID(req.VideoPublicID) {
		fields["video_public_id"] = "動画を選んでください"
	}
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	if p.BirthMonth == nil {
		Error(w, http.StatusUnprocessableEntity, "birth_month_required", "先に生まれた年月を登録してください")
		return
	}
	if isMinor(*p.BirthMonth, time.Now()) {
		if code, message := s.minorRequestBlock(r, p.ID); code != "" {
			Error(w, http.StatusUnprocessableEntity, code, message)
			return
		}
	}
	if err := s.cloud.CoachingVideoReady(r.Context(), req.VideoPublicID); err != nil {
		writeAssetReadyError(w, err, "動画")
		return
	}
	// 目安は送る画面に出したものと同じ計算で決め、依頼に保存する。これを過ぎたら expired になる。
	due, _, err := s.estimateReviewDue(r.Context(), p.ID, req.Tier, req.Event, time.Now())
	if err != nil {
		MapDBError(w, err)
		return
	}
	rr, err := s.q.CreateReviewRequest(r.Context(), db.CreateReviewRequestParams{
		ID:            uuid.New(),
		RequesterID:   p.ID,
		Tier:          req.Tier,
		Event:         req.Event,
		Focus:         focus,
		VideoPublicID: req.VideoPublicID,
		DueAt:         due,
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		Error(w, http.StatusConflict, "conflict", "この動画はもう送っています")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	if offered, err := s.offerReview(r.Context(), rr.ID); err != nil {
		slog.Warn("review offer after create", "err", err)
	} else if offered {
		if fresh, err := s.q.GetReviewRequest(r.Context(), rr.ID); err == nil {
			rr = fresh
		}
	}
	JSON(w, http.StatusCreated, s.reviewRequestJSON(rr))
}

// minorRequestBlock は、18歳未満が依頼を送れない理由を返す。送れるなら空。
// 保護者の同意を待たずに1本目は送れるが、同意が済むまで2本目は送れない。同意しないと言われたら送れない。
func (s *Server) minorRequestBlock(r *http.Request, minorID uuid.UUID) (string, string) {
	consent, err := s.q.GetGuardianConsentByMinor(r.Context(), minorID)
	status := "none"
	if err == nil {
		status = consent.Status
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return "internal_error", "サーバーエラーが発生しました"
	}
	switch status {
	case "consented":
		return "", ""
	case "declined", "revoked":
		return "guardian_declined", "保護者の同意がないため、フォーム添削は使えません"
	}
	sent, err := s.q.CountReviewRequestsByRequester(r.Context(), minorID)
	if err != nil {
		return "internal_error", "サーバーエラーが発生しました"
	}
	if sent > 0 {
		return "guardian_pending", "2本目からは保護者の同意が必要です。保護者にリンクを送ってください"
	}
	return "", ""
}

func (s *Server) listMyReviewRequests(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	page, per, offset := s.queryPage(r)
	total, err := s.q.CountMyReviewRequests(r.Context(), p.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	rows, err := s.q.ListMyReviewRequests(r.Context(), db.ListMyReviewRequestsParams{RequesterID: p.ID, Limit: per, Offset: offset})
	if err != nil {
		MapDBError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, rr := range rows {
		out = append(out, s.reviewRequestJSON(rr))
	}
	JSON(w, http.StatusOK, map[string]any{"requests": out, "meta": meta{Page: int(page), PerPage: int(per), Total: total}})
}

// getReviewRequest は、依頼した本人にだけ依頼を見せる（他の人には無いものとして返す）。
func (s *Server) getReviewRequest(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	rr, err := s.q.GetReviewRequest(r.Context(), id)
	if err == nil && rr.RequesterID != p.ID {
		err = pgx.ErrNoRows
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	// tick が遅れていても、目安を過ぎた依頼は開いた時点で片付ける。
	if reviewWaiting(rr.Status) && !time.Now().Before(rr.DueAt) {
		if _, err := s.runCoachingTick(r.Context()); err != nil {
			slog.Warn("review tick on read", "err", err)
		} else if fresh, err := s.q.GetReviewRequest(r.Context(), id); err == nil {
			rr = fresh
		}
	}
	JSON(w, http.StatusOK, s.reviewRequestJSON(rr))
}

func (s *Server) createDemoCode(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	// 重なる確率はほぼないが、重なったら作り直す。
	for range 3 {
		code, err := newDemoCode()
		if err != nil {
			Error(w, http.StatusInternalServerError, "internal_error", "サーバーエラーが発生しました")
			return
		}
		row, err := s.q.CreateDemoCode(r.Context(), code)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			continue
		}
		if err != nil {
			MapDBError(w, err)
			return
		}
		JSON(w, http.StatusCreated, map[string]any{"code": row.Code, "created_at": row.CreatedAt, "used_at": nil, "used_by_nickname": nil})
		return
	}
	Error(w, http.StatusInternalServerError, "internal_error", "サーバーエラーが発生しました")
}

func (s *Server) listDemoCodes(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	rows, err := s.q.ListDemoCodes(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	if rows == nil {
		rows = []db.ListDemoCodesRow{}
	}
	JSON(w, http.StatusOK, map[string]any{"codes": rows})
}
