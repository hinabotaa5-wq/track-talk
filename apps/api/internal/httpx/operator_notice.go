package httpx

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/jst"
	"github.com/hinata/track-talk/apps/api/internal/mail"
)

// コーチ応募・問い合わせの運営者メール通知（#624、運営者決定 2026-11-13）。
// 宛先は公開ページ掲載のサポート用アドレスで固定し、新しい環境変数・秘密・送信設定は作らない。
// 問い合わせ本文・応募の実績文はメールに入れない。未ログインの /contact の mailto は対象外。
const (
	operatorNoticeTo                = "tracktalk.office@gmail.com"
	coachApplicationNoticeSubject   = "コーチ応募がありました"
	coachProfileUpdateNoticeSubject = "コーチがプロフィールを変更しました"
	inquiryNoticeSubject            = "お問い合わせがありました"
	operatorNoticeTimeLayout        = "2006-01-02 15:04:05"
)

// operatorAdminLink はメールに載せる管理画面リンク。直リンクが無ければ /admin。
func (s *Server) operatorAdminLink() string {
	if base := strings.TrimSpace(s.cfg.WebBaseURL); base != "" {
		return strings.TrimRight(base, "/") + "/admin"
	}
	return "/admin"
}

func operatorNoticeTime(t time.Time) string {
	return t.In(jst.Zone).Format(operatorNoticeTimeLayout) + " JST"
}

func coachApplicationNoticeMail(name, tier string, events []string, appliedAt time.Time, adminLink string) mail.Message {
	return mail.Message{
		To:      operatorNoticeTo,
		Subject: coachApplicationNoticeSubject,
		Text: strings.Join([]string{
			"コーチの応募がありました。",
			"応募者名：" + name,
			"枠：" + tier,
			"担当種目：" + strings.Join(events, "、"),
			"応募時刻：" + operatorNoticeTime(appliedAt),
			"管理画面：" + adminLink,
		}, "\n"),
	}
}

func inquiryNoticeMail(source, category, subject, nickname string, contactEmail *string, receivedAt time.Time, adminLink string) mail.Message {
	lines := []string{
		"お問い合わせがありました。",
		"送信元：" + source,
		"種類：" + category,
		"件名：" + subject,
		"送信者：" + nickname,
	}
	if contactEmail != nil && strings.TrimSpace(*contactEmail) != "" {
		lines = append(lines, "連絡先メール："+strings.TrimSpace(*contactEmail))
	}
	lines = append(lines, "受付時刻："+operatorNoticeTime(receivedAt), "管理画面："+adminLink)
	return mail.Message{
		To:      operatorNoticeTo,
		Subject: inquiryNoticeSubject,
		Text:    strings.Join(lines, "\n"),
	}
}

// notifyOperatorNotice は保存成功後の運営者通知を送る。送信に失敗しても（メール未設定を含む）
// 呼び出し元の保存は成功のままにし、識別子とエラーの種類だけをログに残す。
// 本文・連絡先の値はログに出さない。再送キュー・outbox は作らない。
func (s *Server) notifyOperatorNotice(ctx context.Context, msg mail.Message, idempotencyKey, idKey, idValue string) {
	if err := s.mail.Send(ctx, msg, idempotencyKey); err != nil {
		slog.Warn("operator notice mail failed", idKey, idValue, "err", err)
	}
}

// coachApplicationNoticeKey の冪等キーはプロフィールID＋保存後の applied_at。
// upsert のため審査待ち中の書き直し・見送り／取下げ後の出し直しでも保存のたびに1通送る。
func coachApplicationNoticeKey(profileID string, appliedAt time.Time) string {
	return "coach-application-notice-" + profileID + "-" + appliedAt.UTC().Format(time.RFC3339Nano)
}

func inquiryNoticeKey(inquiryID string) string {
	return "inquiry-notice-" + inquiryID
}

// notifyCoachApplication はコーチ応募の保存成功後に運営者へ1通送る。応答を失敗させない。
func (s *Server) notifyCoachApplication(ctx context.Context, profileID, name, tier string, events []string, appliedAt time.Time) {
	s.notifyOperatorNotice(ctx,
		coachApplicationNoticeMail(name, tier, events, appliedAt, s.operatorAdminLink()),
		coachApplicationNoticeKey(profileID, appliedAt),
		"profile_id", profileID)
}

// coachProfileUpdateNoticeMail は、承認済みコーチの自己編集の変更通知。
// 変更した項目の名前だけを載せ、変更前後の内容（実績文など）はメールに入れない。
func coachProfileUpdateNoticeMail(name string, fields []string, updatedAt time.Time, adminLink string) mail.Message {
	return mail.Message{
		To:      operatorNoticeTo,
		Subject: coachProfileUpdateNoticeSubject,
		Text: strings.Join([]string{
			"コーチがプロフィールを変更しました。",
			"コーチ名：" + name,
			"変更した項目：" + strings.Join(fields, "、"),
			"変更時刻：" + operatorNoticeTime(updatedAt),
			"管理画面：" + adminLink,
		}, "\n"),
	}
}

// coachProfileUpdateNoticeKey の冪等キーはプロフィールID＋更新後の updated_at。
func coachProfileUpdateNoticeKey(profileID string, updatedAt time.Time) string {
	return "coach-profile-update-notice-" + profileID + "-" + updatedAt.UTC().Format(time.RFC3339Nano)
}

// notifyCoachProfileUpdate は自己編集の保存成功後、実際に変わった項目があるときだけ運営者へ1通送る。応答を失敗させない。
func (s *Server) notifyCoachProfileUpdate(ctx context.Context, profileID, name string, fields []string, updatedAt time.Time) {
	s.notifyOperatorNotice(ctx,
		coachProfileUpdateNoticeMail(name, fields, updatedAt, s.operatorAdminLink()),
		coachProfileUpdateNoticeKey(profileID, updatedAt),
		"profile_id", profileID)
}

// notifyInquiry は問い合わせの保存成功後に運営者へ1通送る。応答を失敗させない。
func (s *Server) notifyInquiry(ctx context.Context, inquiryID, source, category, subject, nickname string, contactEmail *string, receivedAt time.Time) {
	s.notifyOperatorNotice(ctx,
		inquiryNoticeMail(source, category, subject, nickname, contactEmail, receivedAt, s.operatorAdminLink()),
		inquiryNoticeKey(inquiryID),
		"inquiry_id", inquiryID)
}
