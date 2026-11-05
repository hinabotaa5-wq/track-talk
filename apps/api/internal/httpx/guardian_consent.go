package httpx

import (
	"context"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/jst"
	"github.com/hinata/track-talk/apps/api/internal/mail"
)

// 保護者同意（フル版）の仕組み。保護者は、本人が入力した保護者のメールに届いた同意用のリンクからだけ同意できる
// （本人のアカウントでは同意を完結させない）。保護者のページは、パーソナル指導など保護者が契約する指導の入口として使い続ける。
//
// 18歳未満が自分で払う動画添削・追加相談は、この同意と月の上限では止めず、購入のたびの宣言で確かめる（minor_purchase.go）。
// それに合わせて、動画添削・追加相談の購入の通知と、上限で止めたときの上限変更のお願いのメールは新しく作らない。
// 前の仕組みで予約済みの購入の通知だけは、tick が送り直す（retryGuardianNotices）。同意・証跡・通知の記録は消さず、保管期間の管理下に残す。

// guardianTermsVersion は、保護者が同意した文面の版。同意ページや規約の文面を変えたら上げる。
const guardianTermsVersion = "guardian-consent-2026-11-support-plan"

// minorRecordRetentionAge は、退会した本人の保護者同意の証跡と購入の記録、購入のたびの宣言を残す年齢。
// 本人がこの年齢になる月の終わりまで残す。未成年者取消しの取消権は成年（18歳）になってから5年で消える（民法126条）ため、それに合わせる。
const minorRecordRetentionAge = 23

// guardianRecordRetentionYears は、生まれた年月を書き写す前（マイグレーション134より前）に退会した本人の記録を残す年数（最後の取引から）。
const guardianRecordRetentionYears = 5

// guardianManageLinkLife は、上限の変更・撤回のためのリンクが使える期間。メールを送るたびに新しく作る。
const guardianManageLinkLife = 30 * 24 * time.Hour

// guardianAccess は、保護者のリンクで分かること。manage は、同意用ではなく上限の変更・撤回のためのリンクで開いたとき。
type guardianAccess struct {
	hash    string
	consent db.GuardianConsent
	manage  bool
}

// newGuardianManageLink は、上限の変更・撤回のためのリンクを作って URL を返す。
func (s *Server) newGuardianManageLink(ctx context.Context, q *db.Queries, minorID uuid.UUID) (string, error) {
	token, hash, err := newGuardianToken()
	if err != nil {
		return "", err
	}
	if err := q.CreateGuardianManageLink(ctx, db.CreateGuardianManageLinkParams{
		TokenHash: hash, MinorID: minorID, ExpiresAt: time.Now().Add(guardianManageLinkLife),
	}); err != nil {
		return "", err
	}
	return s.cfg.WebBaseURL + "/guardian/" + token, nil
}

// recordGuardianEvent は、同意の証跡を1行足す。残せなくても操作そのものは止めず、記録だけする。
func recordGuardianEvent(ctx context.Context, q *db.Queries, ev db.InsertGuardianConsentEventParams) {
	if err := q.InsertGuardianConsentEvent(ctx, ev); err != nil {
		slog.Error("guardian consent event", "event", ev.Event, "err", err)
	}
}

func optionalText(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

// sendGuardianNotice は、予約した購入の通知を送り、結果を記録する。送れなかったものは tick が送り直す。
func (s *Server) sendGuardianNotice(ctx context.Context, n db.GuardianNotice) bool {
	ctx = context.WithoutCancel(ctx)
	if !s.mail.Configured() {
		return false
	}
	link, err := s.newGuardianManageLink(ctx, s.q, n.MinorID)
	if err != nil {
		slog.Error("guardian manage link", "err", err)
		return false
	}
	consent, err := s.q.GetGuardianConsentByMinor(ctx, n.MinorID)
	if err != nil {
		slog.Error("guardian purchase consent", "err", err)
		return false
	}
	used, err := s.q.SumMinorSelfPaidSpend(ctx, db.SumMinorSelfPaidSpendParams{MinorID: n.MinorID})
	if err != nil {
		slog.Error("guardian purchase month spend", "err", err)
		return false
	}
	m := guardianPurchaseMail(n, guardianChildName(ctx, s.q, n.MinorID), used, consent.SelfPayLimitYen, link)
	if err := s.mail.Send(ctx, m, "guardian-purchase-"+n.ID.String()); err != nil {
		slog.Warn("guardian purchase mail", "notice", n.ID, "err", err)
		if err := s.q.MarkGuardianNotice(ctx, db.MarkGuardianNoticeParams{ID: n.ID, Status: "failed", LastError: optionalText(err.Error())}); err != nil {
			slog.Error("mark guardian notice", "err", err)
		}
		return false
	}
	if err := s.q.MarkGuardianNotice(ctx, db.MarkGuardianNoticeParams{ID: n.ID, Status: "sent"}); err != nil {
		slog.Error("mark guardian notice", "err", err)
	}
	return true
}

// retryGuardianNotices は tick から呼ぶ。送れていない購入の通知を送り直す。
func (s *Server) retryGuardianNotices(ctx context.Context) (int, error) {
	if !s.mail.Configured() {
		return 0, nil
	}
	rows, err := s.q.ListGuardianNoticesToSend(ctx)
	if err != nil {
		return 0, err
	}
	sent := 0
	for _, n := range rows {
		if s.sendGuardianNotice(ctx, n) {
			sent++
		}
	}
	return sent, nil
}

// sendGuardianConsentedMail は、同意を受け付けたことと、上限の変更・撤回のリンクを保護者に送る。
func (s *Server) sendGuardianConsentedMail(ctx context.Context, consent db.GuardianConsent) {
	ctx = context.WithoutCancel(ctx)
	if consent.GuardianEmail == nil || !s.mail.Configured() {
		return
	}
	link, err := s.newGuardianManageLink(ctx, s.q, consent.MinorID)
	if err != nil {
		slog.Error("guardian manage link", "err", err)
		return
	}
	id := uuid.New()
	status, lastErr := "sent", (*string)(nil)
	if err := s.mail.Send(ctx, guardianConsentedMail(*consent.GuardianEmail, guardianChildName(ctx, s.q, consent.MinorID), consent, link), "guardian-consented-"+id.String()); err != nil {
		slog.Warn("guardian consented mail", "err", err)
		status, lastErr = "failed", optionalText(err.Error())
	}
	if _, err := s.q.CreateGuardianNotice(ctx, db.CreateGuardianNoticeParams{
		ID: id, MinorID: consent.MinorID, Kind: "consented", GuardianEmail: *consent.GuardianEmail, Status: status,
	}); err != nil {
		slog.Error("guardian notice", "kind", "consented", "err", err)
	} else if lastErr != nil {
		_ = s.q.MarkGuardianNotice(ctx, db.MarkGuardianNoticeParams{ID: id, Status: "failed", LastError: lastErr})
	}
}

// guardianChildName は、保護者へのメールに書くお子さまのアカウント名（ニックネーム）。
func guardianChildName(ctx context.Context, q *db.Queries, minorID uuid.UUID) string {
	p, err := q.GetProfileByID(ctx, minorID)
	if err != nil {
		slog.Warn("guardian mail child name", "err", err)
		return accountNameText(nil)
	}
	return accountNameText(p.Nickname)
}

func accountNameText(nickname *string) string {
	if nickname == nil || strings.TrimSpace(*nickname) == "" {
		return "未設定"
	}
	return *nickname
}

func guardianItemText(refType string) string {
	if refType == "consultation" {
		return "追加相談"
	}
	return "動画添削"
}

// guardianCoachText は、メールの「担当コーチ」。コーチを選ばずに頼んだ添削は、支払いのあとに担当が決まる。
func guardianCoachText(name string) string {
	if name == "" {
		return "これから決まります"
	}
	return name
}

func guardianMailTime(t time.Time) string {
	return t.In(jst.Zone).Format("2006年1月2日 15:04")
}

// 以下のメールの文面は、前の版（guardian-consent-2026-10-final）の運営者の確定版のまま。

const guardianMailGreeting = "TrackTalkをご利用いただきありがとうございます。\n"

const guardianMailSignature = "\nTrackTalk 運営者\n"

// guardianConsentedMail は、同意を受け付けたことの確認。
func guardianConsentedMail(to, child string, c db.GuardianConsent, link string) mail.Message {
	when := ""
	if c.ConsentedAt != nil {
		when = guardianMailTime(*c.ConsentedAt)
	}
	return mail.Message{
		To:      to,
		Subject: "保護者同意を受け付けました",
		Text: guardianMailGreeting +
			"お子さま（アカウント名：" + child + "）の有料サービス利用について、保護者の方の同意を受け付けました。\n\n" +
			"・同意日時：" + when + "\n" +
			"・お子さまが自分で支払える月の上限：" + formatYenNumber(c.SelfPayLimitYen) + "円\n\n" +
			"お子さまが購入するたびに、このメールアドレスへ購入内容をお知らせします。\n" +
			"上限の変更、または同意の取り消しは、下記のリンクからいつでも行えます。\n\n" +
			"上限の変更・同意の取り消しはこちら：" + link + "\n" +
			guardianMailSignature,
	}
}

// guardianPurchaseMail は、お子さまの購入の通知。used は今月の利用合計（この購入を含む）。
func guardianPurchaseMail(n db.GuardianNotice, child string, used int64, limit int32, link string) mail.Message {
	what := guardianItemText("")
	if n.RefType != nil {
		what = guardianItemText(*n.RefType)
	}
	amount := int32(0)
	if n.AmountYen != nil {
		amount = *n.AmountYen
	}
	coach := ""
	if n.CoachName != nil {
		coach = *n.CoachName
	}
	at := n.CreatedAt
	if n.PurchasedAt != nil {
		at = *n.PurchasedAt
	}
	return mail.Message{
		To:      n.GuardianEmail,
		Subject: "お子さまの購入のお知らせ（" + formatYenNumber(amount) + "円）",
		Text: guardianMailGreeting +
			"お子さま（アカウント名：" + child + "）が有料サービスを購入しました。\n\n" +
			"・内容：" + what + "（担当コーチ：" + guardianCoachText(coach) + "）\n" +
			"・金額：" + formatYenNumber(amount) + "円\n" +
			"・日時：" + guardianMailTime(at) + "\n" +
			"・今月の利用合計：" + formatYen64(used) + "円（月の上限：" + formatYenNumber(limit) + "円）\n\n" +
			"上限の変更、または同意の取り消しは、下記のリンクから行えます。\n" +
			link + "\n\n" +
			"購入に心当たりがない場合は、このメールへの返信で運営者までご連絡ください。\n" +
			guardianMailSignature,
	}
}

func formatYen64(v int64) string {
	return formatYenNumber(int32(min(v, int64(math.MaxInt32))))
}
