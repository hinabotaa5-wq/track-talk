package httpx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/jst"
	"github.com/hinata/track-talk/apps/api/internal/mail"
	"github.com/jackc/pgx/v5"
)

// 18歳未満の利用者が自分の PayPay（またはカード）で動画添削と添削のあとの追加相談を買うときの、保護者同意（フル版）。
// 保護者は、本人が入力した保護者のメールに届いた同意用のリンクからだけ同意できる（本人のアカウントでは同意を完結させない）。
// 同意の範囲は、コーチングの利用・本人が自分で払うこと・保護者が選ぶ月の上限・利用規約と安全面のルール。
// 本人は上限の範囲で購入のたびに払える。購入のたびに保護者へ通知し（事前の承認ではなく事後の通知）、
// 保護者は通知のメールのリンクから上限の変更と同意の撤回ができる。
// サポートプランなど保護者が自分で払う契約には、この仕組みは使わない（保護者が契約・支払いの当事者になる）。

// guardianTermsVersion は、保護者が同意した文面の版。同意ページや規約の文面を変えたら上げる。
// 文面はたたき台で、運営者が確定したら版を付け直す。
const guardianTermsVersion = "guardian-consent-2026-10-draft"

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

// minorBlockError は、18歳未満が自分で払う購入を止める理由。
type minorBlockError struct {
	code, message string
	minor         uuid.UUID
}

func (e *minorBlockError) Error() string { return e.message }

// minorSelfPayBlock は、18歳未満が自分で price 円を払ってよいかを確かめる。払えないときは理由を返す。
// 保護者の有効な同意（同意済みで、撤回されていない）があり、今月の合計が上限に収まるときだけ払える。
// 同時の購入で上限をすり抜けないよう、呼ぶ側は LockReviewRequester を取ったトランザクションの q を渡す。
func (s *Server) minorSelfPayBlock(ctx context.Context, q *db.Queries, minorID uuid.UUID, price int32, excludeReview, excludeConsultation uuid.UUID) (*minorBlockError, error) {
	consent, err := q.GetGuardianConsentByMinor(ctx, minorID)
	status := "none"
	if err == nil {
		status = consent.Status
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	switch status {
	case "consented":
	case "declined":
		return &minorBlockError{"guardian_declined", "保護者が同意しなかったため、購入できません", minorID}, nil
	case "revoked":
		return &minorBlockError{"guardian_revoked", "保護者が同意を取り消したため、新しい購入はできません。もう一度使うには、保護者にあらためて同意してもらってください", minorID}, nil
	default:
		return &minorBlockError{"guardian_required", "18歳未満の方は、保護者の同意が済むまで購入できません。「フォーム添削の準備」から保護者にメールを送り、同意してもらってください", minorID}, nil
	}
	// 0円（購入済みのチケットで払う）は、同意だけを確かめる。
	if price == 0 {
		return nil, nil
	}
	limit := consent.SelfPayLimitYen
	used, err := q.SumMinorSelfPaidSpend(ctx, db.SumMinorSelfPaidSpendParams{
		MinorID: minorID, ExcludeReviewID: excludeReview, ExcludeConsultationID: excludeConsultation,
	})
	if err != nil {
		return nil, err
	}
	if used+int64(price) <= int64(limit) {
		return nil, nil
	}
	left := int32(max(int64(limit)-used, 0))
	return &minorBlockError{
		"guardian_limit",
		fmt.Sprintf("今月の上限（%s円）を超えるため、保護者の確認が必要です。今月はあと%s円です。", formatYenNumber(limit), formatYenNumber(left)),
		minorID,
	}, nil
}

// writeMinorBlock は、購入を止めた理由を返す。上限を超えたときは、保護者に上限の変更をお願いするメールを送る。
func (s *Server) writeMinorBlock(w http.ResponseWriter, r *http.Request, e *minorBlockError) {
	msg := e.message
	if e.code == "guardian_limit" {
		if s.requestGuardianLimitRaise(r.Context(), e.minor, clientIP(r)) {
			msg += "保護者に、上限の変更をお願いするメールを送りました。"
		} else {
			msg += "保護者に上限を上げてもらうと購入できます（お願いのメールは12時間に1回まで送ります）。"
		}
	}
	Error(w, http.StatusUnprocessableEntity, e.code, msg)
}

// writeMinorBlockError は、err が購入を止めた理由なら返して true。
func (s *Server) writeMinorBlockError(w http.ResponseWriter, r *http.Request, err error) bool {
	var e *minorBlockError
	if !errors.As(err, &e) {
		return false
	}
	s.writeMinorBlock(w, r, e)
	return true
}

// requestGuardianLimitRaise は、保護者に上限の変更をお願いするメールを送る。12時間に1回まで。送ったら true。
func (s *Server) requestGuardianLimitRaise(ctx context.Context, minorID uuid.UUID, ip string) bool {
	if !s.mail.Configured() {
		return false
	}
	ctx = context.WithoutCancel(ctx)
	consent, err := s.q.ClaimGuardianLimitRequest(ctx, minorID)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			slog.Error("claim guardian limit request", "err", err)
		}
		return false
	}
	link, err := s.newGuardianManageLink(ctx, s.q, minorID)
	if err != nil {
		slog.Error("guardian manage link", "err", err)
		_ = s.q.ReleaseGuardianLimitRequest(ctx, minorID)
		return false
	}
	id := uuid.New()
	m := guardianLimitRequestMail(*consent.GuardianEmail, consent.SelfPayLimitYen, link)
	status, lastErr := "sent", (*string)(nil)
	if err := s.mail.Send(ctx, m, "guardian-limit-"+id.String()); err != nil {
		slog.Warn("guardian limit request mail", "err", err)
		status, lastErr = "failed", optionalText(err.Error())
		_ = s.q.ReleaseGuardianLimitRequest(ctx, minorID)
	}
	if _, err := s.q.CreateGuardianNotice(ctx, db.CreateGuardianNoticeParams{
		ID: id, MinorID: minorID, Kind: "limit_request", GuardianEmail: *consent.GuardianEmail, Status: status,
	}); err != nil {
		slog.Error("guardian notice", "kind", "limit_request", "err", err)
	} else if lastErr != nil {
		_ = s.q.MarkGuardianNotice(ctx, db.MarkGuardianNoticeParams{ID: id, Status: "failed", LastError: lastErr})
	}
	if status != "sent" {
		return false
	}
	recordGuardianEvent(ctx, s.q, db.InsertGuardianConsentEventParams{
		MinorID: minorID, Event: "limit_request_sent", GuardianEmail: consent.GuardianEmail,
		SelfPayLimitYen: &consent.SelfPayLimitYen, Ip: optionalText(ip),
	})
	return true
}

// queueGuardianPurchaseNotice は、18歳未満の本人が払った購入の通知を、支払いと同じトランザクションで予約する。
// 本人が18歳未満でない、または保護者のメールがないときは何もしない。予約したら行を返す（送るのは呼ぶ側）。
func queueGuardianPurchaseNotice(ctx context.Context, q *db.Queries, minorID uuid.UUID, refType string, refID uuid.UUID, amount int32, coachName string) (*db.GuardianNotice, error) {
	p, err := q.GetProfileByID(ctx, minorID)
	if err != nil {
		return nil, err
	}
	if p.BirthMonth == nil || !isMinor(*p.BirthMonth, time.Now()) {
		return nil, nil
	}
	consent, err := q.GetGuardianConsentByMinor(ctx, minorID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && consent.GuardianEmail == nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	now := time.Now()
	n, err := q.CreateGuardianNotice(ctx, db.CreateGuardianNoticeParams{
		ID: uuid.New(), MinorID: minorID, Kind: "purchase", RefType: &refType, RefID: &refID,
		GuardianEmail: *consent.GuardianEmail, AmountYen: &amount, CoachName: optionalText(coachName), PurchasedAt: &now,
		Status: "pending",
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &n, nil
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
	if err := s.mail.Send(ctx, guardianPurchaseMail(n, link), "guardian-purchase-"+n.ID.String()); err != nil {
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
	if err := s.mail.Send(ctx, guardianConsentedMail(*consent.GuardianEmail, consent, link), "guardian-consented-"+id.String()); err != nil {
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

// 以下のメールの文面はたたき台。運営者が確定する。

const guardianMailFooter = "\n・このメールのリンクは保護者の方だけが使うものです。お子さんを含め、他の人には渡さないでください。\n" +
	"・リンクは30日間使えます。期限が切れたあとも、次の通知のメールに新しいリンクが入ります。\n" +
	"・ご不明な点は、TrackTalk のお問い合わせフォームからご連絡ください。\n\n" +
	"TrackTalk\n"

// guardianConsentedMail は、同意を受け付けたことの確認。
func guardianConsentedMail(to string, c db.GuardianConsent, link string) mail.Message {
	when := ""
	if c.ConsentedAt != nil {
		when = c.ConsentedAt.In(jst.Zone).Format("2006年1月2日 15:04")
	}
	return mail.Message{
		To:      to,
		Subject: "【TrackTalk】保護者の同意を受け付けました",
		Text: "TrackTalk のオンラインコーチング（動画添削・追加相談）について、保護者の同意を受け付けました。\n\n" +
			"同意した日時: " + when + "\n" +
			"お子さんが支払う月の上限: " + guardianLimitText(&c.SelfPayLimitYen) + "\n\n" +
			"この範囲で、お子さんは購入のたびにお子さん自身の PayPay（またはカード）でお支払いします。\n" +
			"購入があるたびに、金額・コーチ名・日時をこのアドレスにお知らせします。\n\n" +
			"上限の変更と同意の取り消しは、次のページからいつでもできます。\n" + link + "\n" +
			guardianMailFooter,
	}
}

// guardianPurchaseMail は、お子さんの購入の通知。
func guardianPurchaseMail(n db.GuardianNotice, link string) mail.Message {
	what := "動画添削"
	if n.RefType != nil && *n.RefType == "consultation" {
		what = "追加相談"
	}
	amount := int32(0)
	if n.AmountYen != nil {
		amount = *n.AmountYen
	}
	coach := "（不明）"
	if n.CoachName != nil {
		coach = *n.CoachName
	}
	at := n.CreatedAt
	if n.PurchasedAt != nil {
		at = *n.PurchasedAt
	}
	return mail.Message{
		To:      n.GuardianEmail,
		Subject: "【TrackTalk】お子さんが" + what + "を購入しました（" + formatYenNumber(amount) + "円）",
		Text: "保護者の方が同意した範囲で、お子さんが TrackTalk のオンラインコーチングを購入しました。\n\n" +
			"内容: " + what + "\n" +
			"金額: " + formatYenNumber(amount) + "円\n" +
			"コーチ: " + coach + "\n" +
			"日時: " + at.In(jst.Zone).Format("2006年1月2日 15:04") + "\n\n" +
			"このお知らせは、購入のたびに事後にお送りしています。月の上限の変更や同意の取り消しは、次のページからできます。\n" +
			"取り消したあとは新しい購入ができなくなります（すでに購入した分には影響しません）。\n" + link + "\n" +
			guardianMailFooter,
	}
}

// guardianLimitRequestMail は、お子さんが上限を超える購入をしようとしたときの、上限の変更のお願い。
func guardianLimitRequestMail(to string, limit int32, link string) mail.Message {
	return mail.Message{
		To:      to,
		Subject: "【TrackTalk】月の支払い上限の確認のお願い",
		Text: "お子さんが TrackTalk のオンラインコーチングで、今月の支払い上限（" + guardianLimitText(&limit) + "）を超える購入をしようとしたため、購入を止めています。\n\n" +
			"上限を上げてもよい場合は、次のページから変更してください。変更しない場合は、何もしなくてかまいません（上限は毎月1日にリセットされます）。\n" +
			link + "\n" +
			guardianMailFooter,
	}
}
