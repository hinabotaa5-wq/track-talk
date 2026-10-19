package httpx

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/cloudinary"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/jst"
	"github.com/hinata/track-talk/apps/api/internal/push"
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

// 添削を受け付ける種目。最初は100mだけにし、コーチが増えたら足す。
var reviewEvents = []string{"100m"}

func validReviewEvent(event string) bool {
	return oneOf(event, reviewEvents)
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

// reviewDueAt は返却の目安。設計書の既定どおり、送った翌日の12時（日本時間）にする。
func reviewDueAt(now time.Time) time.Time {
	local := now.In(jst.Zone)
	return time.Date(local.Year(), local.Month(), local.Day()+1, 12, 0, 0, 0, jst.Zone)
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
	if onDuty && !current.OnDuty && !s.payoutsReady(s.refreshPayoutsEnabled(r.Context(), current)) {
		Error(w, http.StatusUnprocessableEntity, "payouts_required", payoutsNotReadyMessage)
		return
	}
	if onDuty && !current.OnDuty {
		// 登録の件数だけでは、届かない宛先でも受付中になれてしまう。通知を使えるときは、
		// 受付を始めた知らせを実際に送って、1台以上に届いたことを確かめる。
		var devices int64
		var err error
		if s.push.Configured() {
			var sent int
			sent, err = s.sendPush(r.Context(), p.ID, push.Message{
				Title: "依頼の受付を始めました",
				Body:  "フォーム添削の依頼が届くと、この端末に通知します",
				URL:   "/coach",
				Tag:   "coach-duty",
			}, 10*time.Minute)
			devices = int64(sent)
		} else {
			devices, err = s.q.CountPushSubscriptions(r.Context(), p.ID)
		}
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
	JSON(w, http.StatusOK, map[string]any{"application": s.coachProfileJSON(c)})
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
	// 支払い方法（card か paypay）。Stripe の鍵があるときだけ使う。
	Method string `json:"method"`
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
	if !validReviewEvent(req.Event) {
		fields["event"] = "いまは100mだけ受け付けています"
	}
	focus, known := normalizeFocus(req.Focus)
	if !known || len(focus) == 0 {
		fields["focus"] = "見てほしい点を1つ以上選んでください"
	}
	if !cloudinary.ValidCoachingPublicID(req.VideoPublicID) {
		fields["video_public_id"] = "動画を選んでください"
	}
	payments := s.stripe.Configured()
	if payments && !validReviewPayMethod(req.Method) {
		fields["method"] = "支払い方法を選んでください"
	}
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	if p.BirthMonth == nil {
		Error(w, http.StatusUnprocessableEntity, "birth_month_required", "先に生まれた年月を登録してください")
		return
	}
	minor := isMinor(*p.BirthMonth, time.Now())
	if minor {
		if code, message := s.minorRequestBlock(r.Context(), s.q, p.ID); code != "" {
			Error(w, http.StatusUnprocessableEntity, code, message)
			return
		}
		if payments {
			msg, err := s.minorMonthlyBlock(r.Context(), p.ID, uuid.Nil, reviewPrices[req.Tier])
			if err != nil {
				MapDBError(w, err)
				return
			}
			if msg != "" {
				Error(w, http.StatusUnprocessableEntity, "monthly_limit", msg)
				return
			}
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
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := s.q.WithTx(tx)
	if minor {
		// 同時に送られた2本が、どちらも「まだ0本」と読んで作られないように、ロックを取ってから確かめ直す。
		if err := q.LockReviewRequester(r.Context(), p.ID); err != nil {
			MapDBError(w, err)
			return
		}
		if code, message := s.minorRequestBlock(r.Context(), q, p.ID); code != "" {
			Error(w, http.StatusUnprocessableEntity, code, message)
			return
		}
	}
	rr, err := q.CreateReviewRequest(r.Context(), db.CreateReviewRequestParams{
		ID:            uuid.New(),
		RequesterID:   p.ID,
		Tier:          req.Tier,
		Event:         req.Event,
		Focus:         focus,
		VideoPublicID: req.VideoPublicID,
		DueAt:         due,
		// 支払いがあるときは、支払いが届いてから割り当てを始める。
		Status: map[bool]string{true: "awaiting_payment", false: "queued"}[payments],
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		Error(w, http.StatusConflict, "conflict", "この動画はもう送っています（お支払いがまだなら、送った依頼の一覧から払えます）")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}
	if payments {
		url, err := s.reviewCheckoutURL(r.Context(), rr, req.Method)
		if err != nil {
			// 支払い画面を作れなかったら、同じ動画で送り直せるように依頼を消す。
			if derr := s.q.DeleteAwaitingReviewRequest(r.Context(), rr.ID); derr != nil {
				slog.Warn("delete unpaid review request", "err", derr)
			}
			writeCheckoutError(w, err)
			return
		}
		out := s.reviewRequestJSON(rr)
		out["checkout_url"] = url
		JSON(w, http.StatusCreated, out)
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
func (s *Server) minorRequestBlock(ctx context.Context, q *db.Queries, minorID uuid.UUID) (string, string) {
	consent, err := q.GetGuardianConsentByMinor(ctx, minorID)
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
	sent, err := q.CountReviewRequestsByRequester(ctx, minorID)
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
	s.writeRequesterReview(w, r, rr)
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
