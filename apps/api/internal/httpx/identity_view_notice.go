package httpx

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/jst"
	"github.com/hinata/track-talk/apps/api/internal/mail"
)

// 身元情報の閲覧通知（#431 作業1、運営者決定 2026-11-11）。
// 管理画面が身元専用サービスで身元情報を表示したあとに、対象コーチの識別子と表示時刻だけをここへ渡し、
// 本体の既存のメール送信で運営者へ1通知らせる。身元専用サービス（金庫）側には送信設定も秘密も置かない。
// ここは身元情報の値を受け取らない（未知の項目は 400 で拒否する）ので、メール・ログ・エラーにも値は出ない。
const (
	identityViewNoticeTo      = "tracktalk.office@gmail.com"
	identityViewNoticeSubject = "身元情報の閲覧通知"
	// 表示時刻として受け付ける幅。画面の時計ずれと再送を見込み、明らかに古い・未来の時刻は受け付けない。
	identityViewNoticePast   = time.Hour
	identityViewNoticeFuture = 5 * time.Minute
)

func identityViewNoticeMail(coach uuid.UUID, viewedAt time.Time) mail.Message {
	return mail.Message{
		To:      identityViewNoticeTo,
		Subject: identityViewNoticeSubject,
		Text: fmt.Sprintf("管理画面でコーチの身元情報が表示されました。対象コーチID：%s／時刻：%s。このメールに身元情報の値は含まれていません。心当たりがない場合はアカウント乗っ取り対応ランブックに従ってください。",
			coach, viewedAt.In(jst.Zone).Format("2006-01-02 15:04:05 JST")),
	}
}

// notifyIdentityView は、管理者だけが呼べる。通知に失敗しても表示そのものは身元専用サービス側で済んでいるので、
// 画面は結果を待たずに表示を続ける。失敗は識別子とエラーの種類だけをログに残す。
func (s *Server) notifyIdentityView(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	coach, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return
	}
	var in struct {
		ViewedAt time.Time `json:"viewed_at"`
	}
	if err := DecodeJSON(r, &in); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", "リクエストが不正です")
		return
	}
	now := time.Now()
	if in.ViewedAt.IsZero() || in.ViewedAt.Before(now.Add(-identityViewNoticePast)) || in.ViewedAt.After(now.Add(identityViewNoticeFuture)) {
		FieldError(w, map[string]string{"viewed_at": "表示時刻を確認してください"})
		return
	}
	key := "identity-view-" + coach.String() + "-" + in.ViewedAt.UTC().Format(time.RFC3339Nano)
	if err := s.mail.Send(r.Context(), identityViewNoticeMail(coach, in.ViewedAt), key); err != nil {
		slog.Warn("identity view notice mail failed", "coach_id", coach, "err", err)
		Error(w, http.StatusServiceUnavailable, "mail_unavailable", "通知を送れませんでした")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
