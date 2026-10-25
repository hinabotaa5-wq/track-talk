package httpx

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/push"
	"github.com/jackc/pgx/v5"
)

// 運営から全員へのお知らせ。管理者画面から、全員のベルと、通知をオンにした人の端末へ送る(本人の設定に従う)。
// プッシュは届いたら取り消せないので、自分への試し送信、送る人数の確認、「送信」の入力、
// 同じ id・同じ文面の二重送信の拒否で、送り間違いを防ぐ。
const (
	announcementTitleMax = 40
	announcementBodyMax  = 200
	announcementURLMax   = 200
	// 届かないまま3日たったら、もう出さない。
	announcementTTL = 72 * time.Hour
	// 送り終えるまでの上限。管理者画面の接続が切れても、ここまでは送り続ける。
	announcementSendTimeout = 30 * time.Minute
	// 送信のときに入力してもらう言葉。画面だけでなく API でも確かめる。
	announcementConfirmWord = "送信"
)

type announcementReq struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	URL     string `json:"url"`
	Confirm string `json:"confirm"`
}

type announcementInput struct {
	title, body, url string
}

// announcementPath は、タップしたときに開くサイト内のパスか。外のサイトへは飛ばさない。
func announcementPath(v string) bool {
	if !strings.HasPrefix(v, "/") || strings.HasPrefix(v, "//") || strings.ContainsAny(v, "\\ \t\r\n") || len(v) > announcementURLMax {
		return false
	}
	u, err := url.Parse(v)
	return err == nil && u.Scheme == "" && u.Host == ""
}

func validateAnnouncement(req announcementReq) (announcementInput, map[string]string) {
	in := announcementInput{
		title: strings.TrimSpace(req.Title),
		body:  strings.TrimSpace(req.Body),
		url:   strings.TrimSpace(req.URL),
	}
	if in.url == "" {
		in.url = "/"
	}
	fields := map[string]string{}
	if n := utf8.RuneCountInString(in.title); n < 1 || n > announcementTitleMax {
		fields["title"] = "見出しは1〜40文字で入力してください"
	}
	if n := utf8.RuneCountInString(in.body); n < 1 || n > announcementBodyMax {
		fields["body"] = "本文は1〜200文字で入力してください"
	}
	if !announcementPath(in.url) {
		fields["url"] = "開くページは / から始まるサイト内のパスで入力してください"
	}
	return in, fields
}

func announcementMessage(id string, in announcementInput) push.Message {
	return push.Message{Title: in.title, Body: in.body, URL: in.url, Tag: "announcement-" + id}
}

// adminAnnouncementRecipients は、送る前に確認画面へ出す人数を返す。
func (s *Server) adminAnnouncementRecipients(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	c, _ := findNotificationCategory(notifyAnnouncement)
	row, err := s.q.CountAnnouncementRecipients(r.Context(), c.PushDefault)
	if err != nil {
		MapDBError(w, err)
		return
	}
	pushCount := row.Push
	if !s.push.Configured() {
		pushCount = 0
	}
	JSON(w, http.StatusOK, map[string]any{"in_app": row.InApp, "push": pushCount, "push_configured": s.push.Configured()})
}

func (s *Server) adminListAnnouncements(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	rows, err := s.q.ListAnnouncements(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	if rows == nil {
		rows = []db.ListAnnouncementsRow{}
	}
	JSON(w, http.StatusOK, map[string]any{"announcements": rows})
}

// adminTestAnnouncement は、送る前の見え方を確かめるため、管理者自身のベルと端末にだけ送る。
func (s *Server) adminTestAnnouncement(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	var req announcementReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	in, fields := validateAnnouncement(req)
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	if err := s.q.InsertMessageNotification(r.Context(), db.InsertMessageNotificationParams{
		ID:          uuid.New(),
		RecipientID: p.ID,
		Title:       &in.title,
		Body:        &in.body,
		Url:         &in.url,
	}); err != nil {
		MapDBError(w, err)
		return
	}
	sent := 0
	if s.push.Configured() {
		n, err := s.sendPush(r.Context(), p.ID, announcementMessage("test", in), 10*time.Minute)
		if err != nil {
			MapDBError(w, err)
			return
		}
		sent = n
	}
	JSON(w, http.StatusOK, map[string]any{"push_sent": sent})
}

// adminSendAnnouncement は全員へ送る。ベルに入れ終えてからプッシュを送り、送った人数を返す。
func (s *Server) adminSendAnnouncement(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	var req announcementReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	id, err := uuid.Parse(req.ID)
	if err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "送信の id が不正です")
		return
	}
	in, fields := validateAnnouncement(req)
	if strings.TrimSpace(req.Confirm) != announcementConfirmWord {
		fields["confirm"] = "確認のため「送信」と入力してください"
	}
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	if n, err := s.q.RecentSameAnnouncement(r.Context(), db.RecentSameAnnouncementParams{Title: in.title, Body: in.body}); err != nil {
		MapDBError(w, err)
		return
	} else if n > 0 {
		Error(w, http.StatusConflict, "conflict", "同じ内容のお知らせを1時間以内に送っています")
		return
	}

	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := s.q.WithTx(tx)
	if _, err := q.CreateAnnouncement(r.Context(), db.CreateAnnouncementParams{
		ID:       id,
		SenderID: &p.ID,
		Title:    in.title,
		Body:     in.body,
		Url:      in.url,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(w, http.StatusConflict, "conflict", "このお知らせはもう送りました")
			return
		}
		MapDBError(w, err)
		return
	}
	inApp, err := q.InsertAnnouncementNotifications(r.Context(), id)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if err := q.SetAnnouncementInAppCount(r.Context(), db.SetAnnouncementInAppCountParams{ID: id, InAppCount: int32(inApp)}); err != nil {
		MapDBError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}

	// ベルに入れたあとは、管理者画面の接続が切れても最後まで送る。Cloud Run は返事のあとに CPU を絞るので、
	// 裏ではなく、このリクエストの中で送り切る。
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), announcementSendTimeout)
	defer cancel()
	recipients := 0
	if s.push.Configured() {
		c, _ := findNotificationCategory(notifyAnnouncement)
		ids, err := s.q.ListPushRecipients(ctx, db.ListPushRecipientsParams{Category: notifyAnnouncement, Exclude: []uuid.UUID{}, PushDefault: c.PushDefault})
		if err != nil {
			slog.Warn("announcement: push recipients", "err", err)
		} else {
			recipients = s.sendPushToAll(ctx, ids, announcementMessage(id.String(), in), announcementTTL)
		}
	}
	if err := s.q.FinishAnnouncement(ctx, db.FinishAnnouncementParams{ID: id, PushRecipients: int32(recipients)}); err != nil {
		slog.Warn("announcement: finish", "err", err)
	}
	JSON(w, http.StatusOK, map[string]any{"id": id, "in_app": inApp, "push_recipients": recipients})
}

// adminCancelAnnouncement は、送ったお知らせをベルから消す。届いたプッシュは取り消せない。
func (s *Server) adminCancelAnnouncement(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := s.q.WithTx(tx)
	if _, err := q.CancelAnnouncement(r.Context(), id); err != nil {
		MapDBError(w, err)
		return
	}
	deleted, err := q.DeleteAnnouncementNotifications(r.Context(), &id)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"deleted": deleted})
}
