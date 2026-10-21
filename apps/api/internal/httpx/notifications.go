package httpx

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/push"
	"github.com/jackc/pgx/v5"
)

// 通知の種類。マイページの通知タブで、種類ごとにベル(アプリ内)とプッシュを切り替えられる。
const (
	notifyComment     = "comment"     // 自分の投稿へのコメント、自分のコメントへの返信
	notifyPractice    = "practice"    // 練習の参加申請・承認・解散後の評価
	notifyRecruitment = "recruitment" // 新しい練習の募集
	notifyInjury      = "injury"      // ケガの投稿の経過報告
	notifyReview      = "review"      // フォーム添削の返答・補足・期限切れなど
	notifyCoaching    = "coaching"    // パーソナル指導のやりとり、コーチの応募の結果
	notifyMonthly     = "monthly"     // 月1回の大会の記録のお知らせ
	// notifyImportant は切り替えられない知らせ。お金(支払い・返金・送金)、保護者の同意、運営の対応、
	// 運営への通報、受付中のコーチへの添削の打診。届かないと本人が損をするか、運営が対応できない。
	notifyImportant = "important"
)

type notificationCategory struct {
	Key string
	// 設定の行がないときの値。募集は全員に届くので、プッシュは自分でオンにした人だけにする。
	InAppDefault bool
	PushDefault  bool
}

// notificationCategories は設定画面に並べる順。migrations の notification_settings の category と合わせる。
var notificationCategories = []notificationCategory{
	{Key: notifyComment, InAppDefault: true, PushDefault: true},
	{Key: notifyPractice, InAppDefault: true, PushDefault: true},
	{Key: notifyRecruitment, InAppDefault: true, PushDefault: false},
	{Key: notifyInjury, InAppDefault: true, PushDefault: true},
	{Key: notifyReview, InAppDefault: true, PushDefault: true},
	{Key: notifyCoaching, InAppDefault: true, PushDefault: true},
	{Key: notifyMonthly, InAppDefault: true, PushDefault: true},
}

func findNotificationCategory(key string) (notificationCategory, bool) {
	for _, c := range notificationCategories {
		if c.Key == key {
			return c, true
		}
	}
	return notificationCategory{}, false
}

// kindCategory は、ベルの通知の kind がどの種類の設定に従うか。
func kindCategory(kind string) string {
	switch kind {
	case "comment", "comment_reply":
		return notifyComment
	case "practice_application", "practice_accepted", "practice_rate":
		return notifyPractice
	case "recruitment_post":
		return notifyRecruitment
	case "injury_update":
		return notifyInjury
	case "review_offer":
		return notifyImportant
	default:
		return notifyReview
	}
}

// notificationWanted は、その人がその種類をベル(inApp)かプッシュで受け取るか。
func notificationWanted(ctx context.Context, q *db.Queries, to uuid.UUID, category string, inApp bool) (bool, error) {
	c, ok := findNotificationCategory(category)
	if !ok {
		return true, nil
	}
	row, err := q.GetNotificationSetting(ctx, db.GetNotificationSettingParams{ProfileID: to, Category: category})
	if errors.Is(err, pgx.ErrNoRows) {
		if inApp {
			return c.InAppDefault, nil
		}
		return c.PushDefault, nil
	}
	if err != nil {
		return false, err
	}
	if inApp {
		return row.InApp, nil
	}
	return row.Push, nil
}

func insertNotification(ctx context.Context, q *db.Queries, recipient uuid.UUID, kind string, postID uuid.UUID, commentID, applicationID *uuid.UUID) error {
	if ok, err := notificationWanted(ctx, q, recipient, kindCategory(kind), true); err != nil || !ok {
		return err
	}
	return q.InsertNotification(ctx, db.InsertNotificationParams{
		ID:            uuid.New(),
		RecipientID:   recipient,
		Kind:          kind,
		PostID:        &postID,
		CommentID:     commentID,
		ApplicationID: applicationID,
	})
}

func insertReviewNotification(ctx context.Context, q *db.Queries, recipient uuid.UUID, kind string, requestID uuid.UUID) error {
	if ok, err := notificationWanted(ctx, q, recipient, kindCategory(kind), true); err != nil || !ok {
		return err
	}
	return q.InsertReviewNotification(ctx, db.InsertReviewNotificationParams{ID: uuid.New(), RecipientID: recipient, Kind: kind, ReviewRequestID: &requestID})
}

// notify は、投稿や依頼にひもづかない知らせを、ベルとプッシュの両方で送る(本人の設定に従う)。
// どちらも補足の知らせなので、失敗は記録だけして呼び出し元の処理は止めない。
func (s *Server) notify(ctx context.Context, to uuid.UUID, category string, msg push.Message, ttl time.Duration) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if ok, err := notificationWanted(ctx, s.q, to, category, true); err != nil {
		slog.Warn("notification setting", "err", err)
	} else if ok {
		url := msg.URL
		if url == "" {
			url = "/"
		}
		if err := s.q.InsertMessageNotification(ctx, db.InsertMessageNotificationParams{
			ID:          uuid.New(),
			RecipientID: to,
			Title:       &msg.Title,
			Body:        &msg.Body,
			Url:         &url,
		}); err != nil {
			slog.Warn("message notification", "err", err)
		}
	}
	s.pushQuietly(ctx, to, category, msg, ttl)
}

// postPush は、コミットのあとに送る、投稿にひもづく知らせのプッシュ。
type postPush struct {
	to       uuid.UUID
	category string
	msg      push.Message
}

func (s *Server) sendPostPushes(ctx context.Context, pushes []postPush) {
	for _, p := range pushes {
		s.pushQuietly(ctx, p.to, p.category, p.msg, 24*time.Hour)
	}
}

// postPushMessage は投稿の知らせのプッシュ。ケガの投稿もあるので、投稿のタイトルや中身は載せない。
func postPushMessage(title string, postID uuid.UUID) push.Message {
	return push.Message{
		Title: title,
		Body:  "TrackTalk を開いて確認してください",
		URL:   "/posts/" + postID.String(),
		Tag:   "post-" + postID.String(),
	}
}

// pushToMany は、たくさんの人へ同じプッシュを送る。リクエストを待たせないよう、裏で送る。
func (s *Server) pushToMany(ctx context.Context, category string, exclude []uuid.UUID, msg push.Message, ttl time.Duration) {
	if !s.push.Configured() {
		return
	}
	c, _ := findNotificationCategory(category)
	ids, err := s.q.ListPushRecipients(ctx, db.ListPushRecipientsParams{Category: category, Exclude: exclude, PushDefault: c.PushDefault})
	if err != nil {
		slog.Warn("push recipients", "category", category, "err", err)
		return
	}
	if len(ids) == 0 {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Minute)
		defer cancel()
		s.sendPushToAll(ctx, ids, msg, ttl)
	}()
}

type notificationSettingJSON struct {
	Category string `json:"category"`
	InApp    bool   `json:"in_app"`
	Push     bool   `json:"push"`
}

func (s *Server) listNotificationSettings(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	rows, err := s.q.ListNotificationSettings(r.Context(), p.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	saved := map[string]db.ListNotificationSettingsRow{}
	for _, row := range rows {
		saved[row.Category] = row
	}
	out := make([]notificationSettingJSON, 0, len(notificationCategories))
	for _, c := range notificationCategories {
		item := notificationSettingJSON{Category: c.Key, InApp: c.InAppDefault, Push: c.PushDefault}
		if row, ok := saved[c.Key]; ok {
			item.InApp, item.Push = row.InApp, row.Push
		}
		out = append(out, item)
	}
	JSON(w, http.StatusOK, map[string]any{"settings": out})
}

type notificationSettingReq struct {
	InApp *bool `json:"in_app"`
	Push  *bool `json:"push"`
}

func (s *Server) updateNotificationSetting(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	category := chi.URLParam(r, "category")
	if _, ok := findNotificationCategory(category); !ok {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return
	}
	var req notificationSettingReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	if req.InApp == nil || req.Push == nil {
		Error(w, http.StatusUnprocessableEntity, "validation_error", "in_app と push を指定してください")
		return
	}
	if err := s.q.UpsertNotificationSetting(r.Context(), db.UpsertNotificationSettingParams{
		ProfileID: p.ID,
		Category:  category,
		InApp:     *req.InApp,
		Push:      *req.Push,
	}); err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, notificationSettingJSON{Category: category, InApp: *req.InApp, Push: *req.Push})
}

func notificationJSON(row db.ListMyNotificationsRow) map[string]any {
	return map[string]any{
		"id":                row.ID,
		"kind":              row.Kind,
		"post_id":           row.PostID,
		"post_title":        row.PostTitle,
		"review_request_id": row.ReviewRequestID,
		"title":             row.Title,
		"body":              row.Body,
		"url":               row.Url,
		"read":              row.ReadAt != nil,
		"created_at":        row.CreatedAt,
	}
}
func (s *Server) listNotifications(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	page, per, offset := s.queryPage(r)
	total, err := s.q.CountMyNotifications(r.Context(), p.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	unread, err := s.q.CountUnreadNotifications(r.Context(), p.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	rows, err := s.q.ListMyNotifications(r.Context(), db.ListMyNotificationsParams{
		RecipientID: p.ID,
		Limit:       per,
		Offset:      offset,
	})
	if err != nil {
		MapDBError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, notificationJSON(row))
	}
	JSON(w, http.StatusOK, map[string]any{
		"notifications": out,
		"unread_count":  unread,
		"meta":          meta{Page: int(page), PerPage: int(per), Total: total},
	})
}

func (s *Server) markNotificationRead(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	n, err := s.q.MarkNotificationRead(r.Context(), db.MarkNotificationReadParams{
		ID:          id,
		RecipientID: p.ID,
	})
	if err != nil {
		MapDBError(w, err)
		return
	}
	if n == 0 {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) markAllNotificationsRead(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	if err := s.q.MarkAllNotificationsRead(r.Context(), p.ID); err != nil {
		MapDBError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
