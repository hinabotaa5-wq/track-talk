package httpx

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5"
)

// 新しいコーチが加わったときの全ユーザーへの自動通知（本体 #438）。
// 発火点は承認そのものではなく、指名候補に初めて入った確定後（受付開始の確定後）。
// 承認時点では受付が止まっており（on_duty=false）、口座登録（外部サービスの登録）が要る場合もあるため、
// 受付開始（PATCH /v1/coach/me の off→on 遷移）の確定後に、候補条件をそのまま確かめて送る。

const (
	// newCoachAnnouncementTitle は運営者決定の固定文言。
	newCoachAnnouncementTitle = "新しいコーチが加わりました"
	// newCoachAnnounceTimeout は自動通知の配信にかける上限。送り切れなかった分は
	// push_outbox に残り、tick の送り直しで届く（既存の一斉配信の回復まわり）。
	newCoachAnnounceTimeout = 2 * time.Minute
)

// newCoachAnnouncementID は、そのコーチの自動通知の id。同じコーチでは何度呼んでも同じになり、
// 送り直し・再試行で二重に作らない（announcements と push_outbox の dedup_key の両方で効く）。
func newCoachAnnouncementID(coach uuid.UUID) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("new-coach:"+coach.String()))
}

// newCoachAnnouncementBody は運営者決定の文面。公開表示名だけを使い、実績・肩書き・個人情報は足さない。
func newCoachAnnouncementBody(display string) string {
	display = strings.TrimSpace(display)
	if display == "" {
		display = "コーチ"
	}
	body := display + "コーチが動画添削のコーチに加わりました"
	// announcements の本文は200文字まで。表示名が長いときは名前側を詰める。
	for utf8.RuneCountInString(body) > announcementBodyMax && utf8.RuneCountInString(display) > 1 {
		display = string([]rune(display)[:utf8.RuneCountInString(display)-1])
		body = display + "コーチが動画添削のコーチに加わりました"
	}
	return body
}

// newCoachAnnouncementURL はタップ遷移先。一覧が公開中ならそのコーチの詳細、閉じている間は
// 指名申し込み（/coaching/new。一覧が閉じていても指名申込みは動く。「準備中」には着地させない）。
func (s *Server) newCoachAnnouncementURL(coach uuid.UUID) string {
	if s.cfg.CoachingCoachList {
		return "/coaches/" + coach.String()
	}
	return "/coaching/new"
}

// isNewCoachNominatable は、そのコーチが指名候補に実際に入るかを、nominees の実装条件そのままに確かめる。
// 承認ステータスだけでは判定しない。いずれかの担当種目で1件でも候補に現れれば候補入りとみなす。
func (s *Server) isNewCoachNominatable(ctx context.Context, coach uuid.UUID, events []string) bool {
	for _, event := range events {
		if !validReviewEvent(event) {
			continue
		}
		rows, err := s.reviewNominees(ctx, s.q, uuid.Nil, event, &coach)
		if err != nil {
			slog.Warn("new coach: nominees", "err", err)
			return false
		}
		if len(rows) > 0 {
			return true
		}
	}
	return false
}

// maybeAnnounceNewCoach は、受付開始の確定後に呼ぶ。通知の成否で受付開始を巻き戻さない。
// 初回の候補入りだけ送り、停止→再開・再承認・編集・再試行では送らない。失敗は記録だけにする。
func (s *Server) maybeAnnounceNewCoach(ctx context.Context, coach uuid.UUID) {
	disp, err := s.q.GetNewCoachDisplay(ctx, coach)
	if err != nil {
		slog.Warn("new coach: display", "err", err)
		return
	}
	if disp.Status != "approved" || !disp.OnDuty {
		return
	}
	if !s.isNewCoachNominatable(ctx, coach, disp.Events) {
		return
	}
	// 一時ミュート中は送信の記録を一切残さない（1回限りの記録を消費しない）。
	// 受付が開いていれば許可リスト内だけに送り、閉じていれば何もしない。
	if s.cfg.CoachingNotificationsMuted {
		if disp.IsAdmin {
			return
		}
		if !s.cfg.CoachingNewAcceptance {
			return
		}
		s.sendNewCoachAnnouncementMuted(ctx, coach, disp.DisplayName)
		return
	}
	// 発火済み記録を原子的に取る。既にあれば（再開・再承認・編集・再試行）何もしない。
	// 受付が閉じている間の初回もここで記録し、開放後にまとめて送らない。
	id := newCoachAnnouncementID(coach)
	if _, err := s.q.ClaimNewCoachAnnouncement(ctx, coach); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return
		}
		slog.Warn("new coach: claim", "err", err)
		return
	}
	// スタッフ権限のアカウント（運営者のテスト用コーチ等）は告知しない。記録は残す。
	if disp.IsAdmin {
		return
	}
	// 受付が閉じている間の候補入りは送らない。記録は残っているので開放後にまとめて送らない。
	if !s.cfg.CoachingNewAcceptance {
		return
	}
	s.sendNewCoachAnnouncement(ctx, coach, disp.DisplayName, id)
}

// sendNewCoachAnnouncement は、既存の運営お知らせの一斉配信の仕組みをそのまま使う。
// ベルは announcement カテゴリの設定に従いコーチ本人を除いて入れ、プッシュは待ちに入れて送る。
// 途中で止まっても同じ id の送り直しで未送信分だけ送り、二重に入れない。
func (s *Server) sendNewCoachAnnouncement(ctx context.Context, coach uuid.UUID, display string, id uuid.UUID) {
	in := announcementInput{
		title: newCoachAnnouncementTitle,
		body:  newCoachAnnouncementBody(display),
		url:   s.newCoachAnnouncementURL(coach),
	}
	if !announcementPath(in.url) {
		slog.Warn("new coach: url")
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), newCoachAnnounceTimeout)
	defer cancel()
	if _, err := s.q.CreateAnnouncement(ctx, db.CreateAnnouncementParams{
		ID: id, SenderID: nil, Title: in.title, Body: in.body, Url: in.url,
	}); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			slog.Warn("new coach: announcement", "err", err)
			return
		}
		existing, gerr := s.q.GetAnnouncement(ctx, id)
		if gerr != nil {
			slog.Warn("new coach: existing", "err", gerr)
			return
		}
		if existing.Title != in.title || existing.Body != in.body || existing.Url != in.url {
			slog.Warn("new coach: announcement mismatch")
			return
		}
	}
	inApp, err := s.q.InsertAnnouncementNotificationsExcluding(ctx, db.InsertAnnouncementNotificationsExcludingParams{
		AnnouncementID: id, Exclude: coach,
	})
	if err != nil {
		slog.Warn("new coach: bell", "err", err)
		return
	}
	if err := s.q.SetAnnouncementInAppCount(ctx, db.SetAnnouncementInAppCountParams{ID: id, InAppCount: int32(inApp)}); err != nil {
		slog.Warn("new coach: bell count", "err", err)
	}
	if err := s.q.MarkNewCoachNotified(ctx, db.MarkNewCoachNotifiedParams{CoachID: coach, AnnouncementID: &id}); err != nil {
		slog.Warn("new coach: notified", "err", err)
	}
	if !s.push.Configured() {
		if err := s.q.FinishAnnouncement(ctx, db.FinishAnnouncementParams{ID: id}); err != nil {
			slog.Warn("new coach: finish", "err", err)
		}
		return
	}
	c, _ := findNotificationCategory(notifyAnnouncement)
	ids, err := s.q.ListPushRecipients(ctx, db.ListPushRecipientsParams{Category: notifyAnnouncement, Exclude: []uuid.UUID{coach}, PushDefault: c.PushDefault})
	if err != nil {
		slog.Warn("new coach: recipients", "err", err)
		return
	}
	msg := announcementMessage(id.String(), in)
	prefix := "announcement:" + id.String() + ":"
	expiresAt := s.now().Add(announcementTTL)
	for _, rid := range ids {
		s.enqueuePushOutbox(ctx, rid, notifyAnnouncement, msg, announcementTTL, expiresAt, prefix+rid.String())
	}
	s.drainPushOutbox(ctx)
	if counts, err := s.q.CountPushOutboxByPrefix(ctx, prefix+"%"); err != nil {
		slog.Warn("new coach: count", "err", err)
	} else if counts.Pending == 0 {
		if err := s.q.FinishAnnouncement(ctx, db.FinishAnnouncementParams{ID: id, PushRecipients: int32(counts.Sent)}); err != nil {
			slog.Warn("new coach: finish", "err", err)
		}
	}
	// 送り切れなかった分は待ちに残り、tick の送り直しで届く。ベルは既に入っている。
}

// sendNewCoachAnnouncementMuted は、一時ミュート中の新規コーチ通知を許可リスト内だけに送る。
// announcements 行・ベルの一括挿入・new_coach_announcements の記録は作らない（1回限りの扱いを狂わせない）。
// 許可リスト内の受信設定（ベル／プッシュ）は通常どおり尊重する。暫定リビジョン専用。
func (s *Server) sendNewCoachAnnouncementMuted(ctx context.Context, coach uuid.UUID, display string) {
	in := announcementInput{
		title: newCoachAnnouncementTitle,
		body:  newCoachAnnouncementBody(display),
		url:   s.newCoachAnnouncementURL(coach),
	}
	if !announcementPath(in.url) {
		slog.Warn("new coach: url")
		return
	}
	id := newCoachAnnouncementID(coach)
	msg := announcementMessage(id.String(), in)
	for _, to := range s.coachingAllowlistedIDs() {
		// コーチ本人には送らない（通常時と合わせる）。
		if to == coach {
			continue
		}
		s.notify(ctx, to, notifyAnnouncement, msg, announcementTTL)
	}
}
