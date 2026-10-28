package httpx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strings"
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
const guardianTermsVersion = "guardian-consent-2026-10-final"

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

// minorPurchase は、18歳未満が自分で払おうとしている購入。上限で止めたときの、保護者へのお願いのメールに書く。
type minorPurchase struct {
	refType   string // "review" または "consultation"
	coachName string
	coachID   *uuid.UUID // coachName が空でコーチが決まっているときは、送る時に名前を引く
	price     int32
}

// minorBlockError は、18歳未満が自分で払う購入を止める理由。上限で止めたときは、購入と今月の合計・上限も持つ。
type minorBlockError struct {
	code, message string
	minor         uuid.UUID
	item          minorPurchase
	used          int64
	limit         int32
}

func (e *minorBlockError) Error() string { return e.message }

// minorSelfPayBlock は、18歳未満が自分で price 円を払ってよいかを確かめる。払えないときは理由を返す。
// 保護者の有効な同意（同意済みで、撤回されていない）があり、今月の合計が上限に収まるときだけ払える。
// 同時の購入で上限をすり抜けないよう、呼ぶ側は LockReviewRequester を取ったトランザクションの q を渡す。
func (s *Server) minorSelfPayBlock(ctx context.Context, q *db.Queries, minorID uuid.UUID, item minorPurchase, excludeReview, excludeConsultation uuid.UUID) (*minorBlockError, error) {
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
		return &minorBlockError{code: "guardian_declined", message: "保護者の方の同意が得られなかったため、有料サービスは利用できません。", minor: minorID}, nil
	case "revoked":
		return &minorBlockError{code: "guardian_revoked", message: "保護者の方の同意が取り消されたため、有料サービスは利用できません。再度利用するには、保護者の方の同意が必要です。", minor: minorID}, nil
	case "pending":
		return &minorBlockError{code: "guardian_required", message: "有料サービスの利用には、保護者の方の同意が必要です。保護者の方に同意のお願いメールを送っています。同意が完了すると、ここから依頼できるようになります。", minor: minorID}, nil
	default:
		// まだ保護者のメールを入力していない（お願いのメールを送っていない）。
		return &minorBlockError{code: "guardian_required", message: "有料サービスの利用には、保護者の方の同意が必要です。保護者の方に同意のお願いメールを送ってください。同意が完了すると、ここから依頼できるようになります。", minor: minorID}, nil
	}
	price := item.price
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
	return &minorBlockError{
		code:    "guardian_limit",
		message: fmt.Sprintf("今月の上限（%s円）に達したため、この購入はできませんでした。", formatYenNumber(limit)),
		minor:   minorID, item: item, used: used, limit: limit,
	}, nil
}

// writeMinorBlock は、購入を止めた理由を返す。上限を超えたときは、保護者に上限の変更をお願いするメールを送る。
func (s *Server) writeMinorBlock(w http.ResponseWriter, r *http.Request, e *minorBlockError) {
	msg := e.message
	if e.code == "guardian_limit" {
		if s.requestGuardianLimitRaise(r.Context(), e, clientIP(r)) {
			msg += "保護者の方に、上限の引き上げをお願いするメールを送りました。"
		} else {
			msg += "上限の変更は保護者の方にお願いしてください。"
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

// requestGuardianLimitRaise は、保護者に上限の変更をお願いするメールを送る。12時間に1回まで。
// 送った、またはこの12時間のうちに送ってあるときは true。
func (s *Server) requestGuardianLimitRaise(ctx context.Context, e *minorBlockError, ip string) bool {
	if !s.mail.Configured() {
		return false
	}
	minorID := e.minor
	ctx = context.WithoutCancel(ctx)
	consent, err := s.q.ClaimGuardianLimitRequest(ctx, minorID)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			slog.Error("claim guardian limit request", "err", err)
			return false
		}
		// 同意済みで行が返らないのは、12時間のうちにもう送ってあるとき。
		return true
	}
	link, err := s.newGuardianManageLink(ctx, s.q, minorID)
	if err != nil {
		slog.Error("guardian manage link", "err", err)
		_ = s.q.ReleaseGuardianLimitRequest(ctx, minorID)
		return false
	}
	id := uuid.New()
	coach := e.item.coachName
	if coach == "" && e.item.coachID != nil {
		if coach, err = coachDisplayName(ctx, s.q, *e.item.coachID); err != nil {
			slog.Warn("guardian limit request coach", "err", err)
		}
	}
	m := guardianLimitRequestMail(*consent.GuardianEmail, guardianLimitRequest{
		child: guardianChildName(ctx, s.q, minorID), refType: e.item.refType, coach: coach,
		price: e.item.price, used: e.used, limit: consent.SelfPayLimitYen,
	}, link)
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

// 以下のメールの文面は運営者の確定版（guardianTermsVersion と同じ版）。

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

// guardianLimitRequest は、上限で止めた購入の中身。used はこの購入を含まない今月の利用合計。
type guardianLimitRequest struct {
	child, refType, coach string
	price                 int32
	used                  int64
	limit                 int32
}

// guardianLimitRequestMail は、お子さまが上限を超える購入をしようとしたときの、上限の変更のお願い。
func guardianLimitRequestMail(to string, r guardianLimitRequest, link string) mail.Message {
	return mail.Message{
		To:      to,
		Subject: "お子さまの購入が月の上限に達しました",
		Text: guardianMailGreeting +
			"お子さま（アカウント名：" + r.child + "）が有料サービスの購入を希望しましたが、今月の利用合計が、設定いただいた月の上限（" +
			formatYenNumber(r.limit) + "円）に達しているため、購入を止めています。\n\n" +
			"・購入しようとした内容：" + guardianItemText(r.refType) + "（担当コーチ：" + guardianCoachText(r.coach) + "）\n" +
			"・金額：" + formatYenNumber(r.price) + "円\n" +
			"・今月の利用合計：" + formatYen64(r.used) + "円\n\n" +
			"購入を許可する場合は、下記のリンクから上限を引き上げてください。変更しない場合、お子さまは来月になるまで新たな購入はできません。\n\n" +
			"上限の変更はこちら：" + link + "\n" +
			guardianMailSignature,
	}
}

func formatYen64(v int64) string {
	return formatYenNumber(int32(min(v, int64(math.MaxInt32))))
}
