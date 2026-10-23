package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/cloudinary"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/push"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// パーソナル指導のやりとり欄。申し込んだ人とコーチが読み書きし、18歳未満のときは保護者もリンクから参加する。
// 期間中だけ送れて、終わったあとは読むだけ。リアルタイムにはせず、送ったら相手に Web Push で知らせる。
// 文章とメニューは、利用者からもコーチからも連絡先らしい文字を止める。止めた文章は保存せず、回数だけ数える。

const maxMessageRunes = 2000

type planMenuDay struct {
	Day     string `json:"day"`
	Content string `json:"content"`
}

// planMenu は、コーチが送る練習メニュー。
type planMenu struct {
	Title string        `json:"title"`
	Days  []planMenuDay `json:"days"`
}

func (m *planMenu) normalize() string {
	m.Title = strings.TrimSpace(m.Title)
	if e := coachTextError("メニューの名前", m.Title, 60); e != "" {
		return e
	}
	if len(m.Days) == 0 || len(m.Days) > 14 {
		return "メニューは1〜14日分にしてください"
	}
	for i := range m.Days {
		m.Days[i].Day = strings.TrimSpace(m.Days[i].Day)
		m.Days[i].Content = strings.TrimSpace(m.Days[i].Content)
		if e := coachTextError("日付や曜日", m.Days[i].Day, 20); e != "" {
			return e
		}
		if e := coachTextError("メニューの中身", m.Days[i].Content, 300); e != "" {
			return e
		}
	}
	return ""
}

func (m planMenu) text() string {
	var b strings.Builder
	b.WriteString(m.Title)
	for _, d := range m.Days {
		b.WriteString("\n" + d.Day + "\n" + d.Content)
	}
	return b.String()
}

// contractParty は、ログインしている人がこの申し込みの本人かコーチかを確かめる。違えば 404。
func (s *Server) contractParty(w http.ResponseWriter, r *http.Request) (db.Profile, db.CoachingContract, string, bool) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return db.Profile{}, db.CoachingContract{}, "", false
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return db.Profile{}, db.CoachingContract{}, "", false
	}
	ct, err := s.q.GetCoachingContract(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && ct.RequesterID != me.ID && ct.CoachID != me.ID) {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return db.Profile{}, db.CoachingContract{}, "", false
	}
	if err != nil {
		MapDBError(w, err)
		return db.Profile{}, db.CoachingContract{}, "", false
	}
	role := "requester"
	if ct.CoachID == me.ID {
		role = "coach"
	}
	return me, ct, role, true
}

// coachingMessageJSON は1通を返す。viewerRole が guardian のときは、保護者が送ったものを mine にする。
func (s *Server) coachingMessageJSON(m db.CoachingMessage, viewer uuid.UUID, viewerRole string) map[string]any {
	mine := m.SenderID != nil && *m.SenderID == viewer
	if viewerRole == "guardian" {
		mine = m.SenderRole == "guardian"
	}
	out := map[string]any{
		"id":          m.ID,
		"sender_role": m.SenderRole,
		"mine":        mine,
		// 送った人が退会していれば false。保護者はアカウントを持たないので常に true。
		"sender_active":        m.SenderID != nil || m.SenderRole == "guardian",
		"body":                 m.Body,
		"video_url":            nil,
		"thumbnail_url":        nil,
		"menu":                 nil,
		"created_at":           m.CreatedAt,
		"review_submission_id": m.ReviewSubmissionID,
	}
	if m.VideoPublicID != nil {
		out["video_url"] = s.cloud.CoachingPlaybackURL(*m.VideoPublicID)
		out["thumbnail_url"] = s.cloud.CoachingThumbnailURL(*m.VideoPublicID)
	}
	if len(m.Menu) > 0 {
		out["menu"] = json.RawMessage(m.Menu)
	}
	return out
}

// messagesVisible は、やりとり欄を読める状態か。止めている間も、それまでのものは読める。
func messagesVisible(status string) bool {
	return status == "active" || status == "suspended" || status == "ended" || status == "refunded"
}

func (s *Server) writeCoachingMessages(w http.ResponseWriter, r *http.Request, ct db.CoachingContract, viewer uuid.UUID, viewerRole string) {
	if !messagesVisible(ct.Status) {
		JSON(w, http.StatusOK, map[string]any{"messages": []any{}})
		return
	}
	msgs, err := s.q.ListCoachingMessages(r.Context(), ct.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, s.coachingMessageJSON(m, viewer, viewerRole))
	}
	JSON(w, http.StatusOK, map[string]any{"messages": out})
}

func (s *Server) listCoachingMessages(w http.ResponseWriter, r *http.Request) {
	me, ct, role, ok := s.contractParty(w, r)
	if !ok {
		return
	}
	s.writeCoachingMessages(w, r, ct, me.ID, role)
}

type coachingMessageReq struct {
	ReviewSubmissionID *uuid.UUID `json:"review_submission_id"`
	Body               string     `json:"body"`
	VideoPublicID      string     `json:"video_public_id"`
	Menu               *planMenu  `json:"menu"`
}

func (s *Server) createCoachingMessage(w http.ResponseWriter, r *http.Request) {
	me, ct, role, ok := s.contractParty(w, r)
	if !ok {
		return
	}
	s.sendCoachingMessage(w, r, ct, &me.ID, role)
}

// sendCoachingMessage は1通を送る。保護者は sender が nil で、動画は送れない（アップロードにログインが要るため）。
func (s *Server) sendCoachingMessage(w http.ResponseWriter, r *http.Request, ct db.CoachingContract, sender *uuid.UUID, role string) {
	switch ct.Status {
	case "active":
	case "suspended":
		Error(w, http.StatusConflict, "conflict", "運営が確認のため、やりとりを止めています")
		return
	default:
		Error(w, http.StatusConflict, "conflict", "期間中だけ送れます")
		return
	}
	var req coachingMessageReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	req.Body = strings.TrimSpace(req.Body)
	fields := map[string]string{}
	if n := len([]rune(req.Body)); n > maxMessageRunes {
		fields["body"] = "2000文字以内で入力してください"
	}
	if req.Menu != nil {
		if role != "coach" {
			fields["menu"] = "メニューはコーチだけが送れます"
		} else if e := req.Menu.normalize(); e != "" {
			fields["menu"] = e
		}
	}
	if req.VideoPublicID != "" {
		if role == "guardian" {
			fields["video_public_id"] = "保護者のページからは動画を送れません"
		} else if !cloudinary.ValidCoachingPublicID(req.VideoPublicID) {
			fields["video_public_id"] = "動画を選び直してください"
		}
	}
	if req.Body == "" && req.VideoPublicID == "" && req.Menu == nil {
		fields["body"] = "メッセージを入力してください"
	}
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	text := req.Body
	if req.Menu != nil {
		text += "\n" + req.Menu.text()
	}
	if looksLikeContact(text) {
		// 回数はアカウントごとに数える。保護者はアカウントがないので数えない。
		if sender != nil {
			if err := s.q.CountBlockedContact(r.Context(), db.CountBlockedContactParams{ContractID: ct.ID, ProfileID: *sender}); err != nil {
				MapDBError(w, err)
				return
			}
		}
		Error(w, http.StatusUnprocessableEntity, "contact_blocked", "連絡先（URL・メールアドレス・SNS の ID・電話番号など）は送れません。TrackTalk の外でのやりとりは規約で禁止されています")
		return
	}
	var videoID *string
	if req.VideoPublicID != "" {
		if !s.coachingVideoFree(w, r, req.VideoPublicID, "video_public_id") {
			return
		}
		if sender == nil || !s.uploadIssuedToMe(w, r, *sender, req.VideoPublicID, "video_public_id", "動画") {
			if sender == nil {
				FieldError(w, map[string]string{"video_public_id": "動画を選び直してください"})
			}
			return
		}
		if err := s.cloud.CoachingVideoReady(r.Context(), req.VideoPublicID); err != nil {
			writeAssetReadyError(w, err, "動画")
			return
		}
		videoID = &req.VideoPublicID
	}
	var menu []byte
	if req.Menu != nil {
		var err error
		if menu, err = json.Marshal(req.Menu); err != nil {
			Error(w, http.StatusInternalServerError, "internal_error", "サーバーエラーが発生しました")
			return
		}
	}
	var m db.CoachingMessage
	err := s.tx(r.Context(), func(q *db.Queries) error {
		current, err := q.LockConsultationContract(r.Context(), ct.ID)
		if err != nil {
			return err
		}
		if current.Status != "active" {
			return &ticketRuleError{"contract_closed", "期間中だけ送れます"}
		}
		support := current.FulfillmentVersion == 1
		if support {
			if err := supportWritable(current, false); err != nil {
				return err
			}
			if current.StartsAt != nil && time.Now().Before(*current.StartsAt) {
				return &ticketRuleError{"support_not_started", "支援開始日からメッセージを送れます"}
			}
			if req.ReviewSubmissionID != nil && (role != "coach" || req.Body == "" && videoID == nil) {
				return &ticketRuleError{"invalid_review_answer", "動画への回答は担当コーチが文章または動画で送ってください"}
			}
		} else if req.ReviewSubmissionID != nil {
			return &ticketRuleError{"invalid_review_answer", "この契約は動画ごとの回答管理に対応していません"}
		}
		if role == "requester" && videoID != nil && current.ServiceType != nil && current.VideoReviews != nil && *current.VideoReviews > 0 {
			used, err := q.CountContractVideoSubmissions(r.Context(), ct.ID)
			if err != nil {
				return err
			}
			if used >= int32(*current.VideoReviews) {
				return &ticketRuleError{"video_limit", "プランの動画添削回数を使い切りました。追加相談から送信できます"}
			}
		}
		if err := lockUnusedCoachingVideo(r.Context(), q, videoID, nil); err != nil {
			return err
		}
		m, err = q.CreateCoachingMessage(r.Context(), db.CreateCoachingMessageParams{
			ID:            uuid.New(),
			ContractID:    ct.ID,
			SenderID:      sender,
			SenderRole:    role,
			Body:          req.Body,
			VideoPublicID: videoID,
			Menu:          menu,
		})
		if err == nil && support {
			if role == "requester" && videoID != nil {
				err = q.CreateSupportVideo(r.Context(), db.CreateSupportVideoParams{QuestionID: m.ID, ContractID: ct.ID})
			}
			if err == nil && req.ReviewSubmissionID != nil {
				n, e := q.AnswerSupportVideo(r.Context(), db.AnswerSupportVideoParams{QuestionID: *req.ReviewSubmissionID, ContractID: ct.ID, AnswerID: &m.ID})
				if e != nil {
					return e
				}
				if n != 1 {
					return &ticketRuleError{"review_already_answered", "この動画は回答済みか、別の契約の動画です"}
				}
				err = q.SetMessageReviewSubmission(r.Context(), db.SetMessageReviewSubmissionParams{ID: m.ID, ReviewSubmissionID: req.ReviewSubmissionID})
				m.ReviewSubmissionID = req.ReviewSubmissionID
			}
		}
		if err != nil || role != "requester" || videoID == nil || !ct.StartOnFirstVideo {
			return err
		}
		// 最初の動画から数えるプランは、利用者の最初の動画が届いた時から期間を数え直す。
		return q.StartCoachingClock(r.Context(), ct.ID)
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		Error(w, http.StatusConflict, "conflict", "この動画はもう送っています")
		return
	}
	if err != nil {
		writeCheckoutError(w, err)
		return
	}
	// 中身は通知に載せない。保護者にはメッセージごとの通知はしない。
	body := "「" + ct.PlanTitle + "」のやりとり欄を開いてください"
	tag := "contract-msg-" + ct.ID.String()
	toRequester := push.Message{Title: "コーチからメッセージが届きました", Body: body, URL: "/coaching/personal/" + ct.ID.String(), Tag: tag}
	toCoach := push.Message{Title: "パーソナル指導に新しいメッセージ", Body: body, URL: "/coaching/personal/" + ct.ID.String(), Tag: tag}
	switch role {
	case "requester":
		s.notify(r.Context(), ct.CoachID, notifyCoaching, toCoach, 24*time.Hour)
	case "coach":
		s.notify(r.Context(), ct.RequesterID, notifyCoaching, toRequester, 24*time.Hour)
	case "guardian":
		toRequester.Title = "保護者がメッセージを送りました"
		toCoach.Title = "保護者からメッセージが届きました"
		s.notify(r.Context(), ct.RequesterID, notifyCoaching, toRequester, 24*time.Hour)
		s.notify(r.Context(), ct.CoachID, notifyCoaching, toCoach, 24*time.Hour)
	}
	viewer := uuid.Nil
	if sender != nil {
		viewer = *sender
	}
	JSON(w, http.StatusCreated, map[string]any{"message": s.coachingMessageJSON(m, viewer, role)})
}

func (s *Server) rateCoachingContract(w http.ResponseWriter, r *http.Request) {
	me, ct, role, ok := s.contractParty(w, r)
	if !ok {
		return
	}
	if role != "requester" {
		Error(w, http.StatusForbidden, "forbidden", "申し込んだ人だけが評価できます")
		return
	}
	var req struct {
		Stars int `json:"stars"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	if req.Stars < 1 || req.Stars > 5 {
		FieldError(w, map[string]string{"stars": "★を1〜5で選んでください"})
		return
	}
	rt, err := s.q.RateCoachingContract(r.Context(), db.RateCoachingContractParams{Stars: int16(req.Stars), ContractID: ct.ID, RequesterID: me.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusConflict, "conflict", "期間が終わったあとに1回だけ評価できます")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"rating": map[string]any{"stars": rt.Stars, "rated_at": rt.RatedAt}})
}

// coachingVideoFree は、非公開動画の ID がまだどこにも使われていないかを見る。使われていたらエラーを書いて false を返す。
// アップロードした本人しか知らない ID だけを受け付け、他人の動画の流用を防ぐ。
func (s *Server) coachingVideoFree(w http.ResponseWriter, r *http.Request, publicID, field string) bool {
	inUse, err := s.q.CoachingVideoInUse(r.Context(), publicID)
	if err != nil {
		MapDBError(w, err)
		return false
	}
	if inUse {
		FieldError(w, map[string]string{field: "この動画は使えません。動画を選び直してください"})
		return false
	}
	return true
}

// 動画は複数の種類の会話に同時利用させない。参照確認と保存を同じロック内で行う。
func lockUnusedCoachingVideo(ctx context.Context, q *db.Queries, id, retained *string) error {
	if id == nil {
		return nil
	}
	if err := q.LockCoachingVideo(ctx, *id); err != nil {
		return err
	}
	if retained != nil && *retained == *id {
		return nil
	}
	used, err := q.CoachingVideoInUse(ctx, *id)
	if err != nil {
		return err
	}
	if used {
		return &ticketRuleError{"video_in_use", "この動画は使用済みです。別の動画を選んでください"}
	}
	return nil
}
