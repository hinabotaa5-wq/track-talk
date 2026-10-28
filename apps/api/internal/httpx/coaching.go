package httpx

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/cloudinary"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/jst"
	"github.com/hinata/track-talk/apps/api/internal/mail"
	"github.com/hinata/track-talk/apps/api/internal/push"
	"github.com/jackc/pgx/v5"
)

// 18歳未満の月の上限で選べる金額。nil は上限なし。
// 保護者が払うパーソナル指導・サポートプランなどにかかる上限で、本人が自分で払う分の上限（selfPayLimitOptions）とは別。
var guardianLimitOptions = []int32{1000, 3000, 5000}

// 親が上限を選ばなかったときの金額。
const defaultGuardianLimitYen int32 = 3000

// 本人が自分で払う動画添削・追加相談の、月の上限で保護者が選べる金額。上限なしは選べない。
var selfPayLimitOptions = []int32{1500, 3000, 5000, 10000}

// 保護者が本人の支払いの上限を選ばなかったときの金額（いちばん低い候補）。
const defaultSelfPayLimitYen int32 = 1500

// isMinor は、生まれた年月から18歳未満かを判定する。
// 誕生日が分からないので、生まれた月の翌月1日を18歳の誕生日とみなす（大人として扱うのが遅れる側に倒す）。
func isMinor(birthMonth, now time.Time) bool {
	adult := time.Date(birthMonth.Year()+18, birthMonth.Month()+1, 1, 0, 0, 0, 0, jst.Zone)
	return now.In(jst.Zone).Before(adult)
}

// parseBirthMonth は "2009-04" の形を受け取り、その月の1日を返す。
func parseBirthMonth(raw string, now time.Time) (time.Time, string) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}, "生まれた年月を入力してください"
	}
	month, err := time.Parse("2006-01", value)
	if err != nil {
		return time.Time{}, "生まれた年月が不正です"
	}
	today := now.In(jst.Zone)
	thisMonth := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
	if month.After(thisMonth) {
		return time.Time{}, "生まれた年月が不正です"
	}
	if month.Before(time.Date(today.Year()-100, today.Month(), 1, 0, 0, 0, 0, time.UTC)) {
		return time.Time{}, "生まれた年月が不正です"
	}
	return month, ""
}

func newGuardianToken() (token, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(buf)
	return token, hashGuardianToken(token), nil
}

func hashGuardianToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func guardianStatusJSON(c db.GuardianConsent) map[string]any {
	return map[string]any{
		"status":            c.Status,
		"monthly_limit_yen": c.MonthlyLimitYen,
		"allow_publish":     c.AllowPublish,
		"consented_at":      c.ConsentedAt,
		"guardian_email":    c.GuardianEmail,
		// 本人が自分で払う動画添削・追加相談の月の上限と、今月の合計（writeCoachingMe で入れる）。
		"self_pay_limit_yen": c.SelfPayLimitYen,
		"month_spent_yen":    nil,
	}
}

// coachingMe は、添削を頼む前に必要な状態（年齢、保護者の同意、通知を受け取る端末の数）を返す。
func (s *Server) coachingMe(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	s.writeCoachingMe(w, r, p)
}

func (s *Server) writeCoachingMe(w http.ResponseWriter, r *http.Request, p db.Profile) {
	out := map[string]any{
		"birth_month":  nil,
		"is_minor":     nil,
		"guardian":     nil,
		"push_devices": int64(0),
		"demo":         p.CoachingDemo,
	}
	if p.BirthMonth != nil {
		out["birth_month"] = p.BirthMonth.Format("2006-01")
		out["is_minor"] = isMinor(*p.BirthMonth, time.Now())
	}
	consent, err := s.q.GetGuardianConsentByMinor(r.Context(), p.ID)
	switch {
	case err == nil:
		g := guardianStatusJSON(consent)
		// 今月、本人が自分で払った動画添削・追加相談の合計。購入の前に「あと何円」を出すのに使う。
		spent, err := s.q.SumMinorSelfPaidSpend(r.Context(), db.SumMinorSelfPaidSpendParams{MinorID: p.ID})
		if err != nil {
			MapDBError(w, err)
			return
		}
		g["month_spent_yen"] = spent
		out["guardian"] = g
	case !errors.Is(err, pgx.ErrNoRows):
		MapDBError(w, err)
		return
	}
	devices, err := s.q.CountPushSubscriptions(r.Context(), p.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	out["push_devices"] = devices
	JSON(w, http.StatusOK, out)
}

type birthMonthReq struct {
	BirthMonth string `json:"birth_month"`
}

func (s *Server) setBirthMonth(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	var req birthMonthReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	if p.BirthMonth != nil {
		Error(w, http.StatusConflict, "conflict", "生まれた年月はすでに登録されています。変更はお問い合わせからお願いします")
		return
	}
	month, message := parseBirthMonth(req.BirthMonth, time.Now())
	if message != "" {
		FieldError(w, map[string]string{"birth_month": message})
		return
	}
	updated, err := s.q.SetBirthMonth(r.Context(), db.SetBirthMonthParams{ID: p.ID, BirthMonth: &month})
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusConflict, "conflict", "生まれた年月はすでに登録されています。変更はお問い合わせからお願いします")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	s.writeCoachingMe(w, r, updated)
}

// 保護者の同意のリンクを1日（日本時間）に送れる回数。
const guardianLinkDailyLimit int32 = 5

type guardianLinkReq struct {
	GuardianEmail string `json:"guardian_email"`
}

// issueGuardianLink は、18歳未満の本人に頼まれて、保護者の同意のリンクを保護者のメールあてに送る。
// 本人だけで同意を済ませられないよう、トークンは本人には返さない（メールの中にだけある）。
// 送り直すと前のリンクは使えなくなる。保護者が一度答えたあとは、送り先は前と同じアドレスだけ。
func (s *Server) issueGuardianLink(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	if p.BirthMonth == nil {
		Error(w, http.StatusUnprocessableEntity, "validation_error", "先に生まれた年月を登録してください")
		return
	}
	if !isMinor(*p.BirthMonth, time.Now()) {
		Error(w, http.StatusUnprocessableEntity, "validation_error", "18歳以上の人は保護者の同意はいりません")
		return
	}
	var req guardianLinkReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	current, err := s.q.GetGuardianConsentByMinor(r.Context(), p.ID)
	exists := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		MapDBError(w, err)
		return
	}
	email := strings.TrimSpace(req.GuardianEmail)
	if email == "" && exists && current.GuardianEmail != nil {
		email = *current.GuardianEmail
	}
	if e := ContactEmail(email); e != "" {
		if email == "" {
			e = "保護者のメールアドレスを入力してください"
		}
		FieldError(w, map[string]string{"guardian_email": e})
		return
	}
	if identity, _ := userIdentity(r.Context()); identity.Email != "" && strings.EqualFold(identity.Email, email) {
		FieldError(w, map[string]string{"guardian_email": "自分のメールアドレスではなく、保護者のメールアドレスを入力してください"})
		return
	}
	if exists && current.Status != "pending" && (current.GuardianEmail == nil || !strings.EqualFold(*current.GuardianEmail, email)) {
		Error(w, http.StatusConflict, "guardian_email_locked", "保護者が一度答えたあとは、送り先のメールアドレスを変えられません。変えたいときはお問い合わせください")
		return
	}
	if !s.mail.Configured() {
		Error(w, http.StatusServiceUnavailable, "mail_unavailable", "いまは保護者にメールを送れません。しばらくしてからお試しください")
		return
	}
	token, hash, err := newGuardianToken()
	if err != nil {
		Error(w, http.StatusInternalServerError, "internal_error", "サーバーエラーが発生しました")
		return
	}
	// 新しいリンクに差し替えるのは、メールを送れたときだけ。送れなかったら、
	// 保護者が持っている前のリンクを使えるままにし、1日の回数も減らさない。
	var consent db.GuardianConsent
	errMail := errors.New("guardian link mail")
	err = s.tx(r.Context(), func(q *db.Queries) error {
		var err error
		consent, err = q.IssueGuardianLink(r.Context(), db.IssueGuardianLinkParams{
			MinorID: p.ID, TokenHash: hash, GuardianEmail: &email, DailyLimit: guardianLinkDailyLimit,
		})
		if err != nil {
			return err
		}
		if err := s.mail.Send(r.Context(), guardianLinkMail(email, accountNameText(p.Nickname), s.cfg.WebBaseURL+"/guardian/"+token), "guardian-link-"+hash); err != nil {
			slog.Warn("guardian link mail", "err", err)
			return errMail
		}
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusTooManyRequests, "rate_limited", "今日はこれ以上送れません。明日もう一度お試しください")
		return
	}
	if errors.Is(err, errMail) {
		Error(w, http.StatusBadGateway, "mail_failed", "保護者にメールを送れませんでした。アドレスを確かめて、もう一度お試しください")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	recordGuardianEvent(r.Context(), s.q, db.InsertGuardianConsentEventParams{
		MinorID: p.ID, Event: "link_sent", GuardianEmail: &email, Ip: optionalText(clientIP(r)),
	})
	if _, err := s.q.CreateGuardianNotice(r.Context(), db.CreateGuardianNoticeParams{
		ID: uuid.New(), MinorID: p.ID, Kind: "consent_request", GuardianEmail: email, Status: "sent",
	}); err != nil {
		slog.Error("guardian notice", "kind", "consent_request", "err", err)
	}
	JSON(w, http.StatusCreated, guardianStatusJSON(consent))
}

// guardianLinkMail は、保護者に送る同意のお願い。文面は運営者の確定版。
func guardianLinkMail(to, child, link string) mail.Message {
	return mail.Message{
		To:      to,
		Subject: "お子さまの有料サービス利用についての同意のお願い",
		Text: guardianMailGreeting +
			"お子さま（アカウント名：" + child + "）から、有料サービス（動画添削・追加相談）を利用するための同意依頼が届いています。\n\n" +
			"未成年のお子さまが有料サービスを利用するには、保護者の方の同意が必要です。下記のリンクから、内容をご確認のうえ、同意のお手続きをお願いいたします。\n\n" +
			"同意手続きはこちら：" + link + "\n" +
			"（リンクの有効期限は30日です。このリンクからは1回だけ同意できます。）\n\n" +
			"同意いただくと、お子さまは保護者の方が設定した月の上限の範囲で、ご自身のクレジットカードまたはPayPayでサービスを購入できるようになります。購入のたびに、このメールアドレスへお知らせします。\n\n" +
			"心当たりがない場合は、このメールを破棄してください。同意しない限り、お子さまが有料サービスを購入することはありません。\n" +
			guardianMailSignature,
	}
}

// guardianByToken は、保護者のリンクを確かめる。同意用のリンク（本人が送ったもの。出してから30日）か、
// 上限の変更・撤回のためのリンク（保護者へのメールに入れたもの。作ってから30日）のどちらか。
func (s *Server) guardianByToken(w http.ResponseWriter, r *http.Request) (guardianAccess, bool) {
	const gone = "このリンクは使えません。お子さんに頼んで、新しいリンクをメールで送ってもらってください"
	token := chi.URLParam(r, "token")
	if len(token) != 43 {
		Error(w, http.StatusNotFound, "not_found", gone)
		return guardianAccess{}, false
	}
	hash := hashGuardianToken(token)
	consent, err := s.q.GetGuardianConsentByToken(r.Context(), hash)
	if err == nil {
		return guardianAccess{hash: hash, consent: consent}, true
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		MapDBError(w, err)
		return guardianAccess{}, false
	}
	consent, err = s.q.GetGuardianConsentByManageLink(r.Context(), hash)
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusNotFound, "not_found", gone)
		return guardianAccess{}, false
	}
	if err != nil {
		MapDBError(w, err)
		return guardianAccess{}, false
	}
	return guardianAccess{hash: hash, consent: consent, manage: true}, true
}

// guardianPublicJSON は保護者のページに返す内容。子のアカウントの情報は含めない。
// can_consent は、このリンクで同意できるか（同意用のリンクで、まだ同意に使っていない）。
func guardianPublicJSON(a guardianAccess) map[string]any {
	c := a.consent
	options := make([]any, 0, len(guardianLimitOptions)+1)
	for _, v := range guardianLimitOptions {
		options = append(options, v)
	}
	options = append(options, nil)
	canConsent := !a.manage && c.Status != "consented" && (c.ConsentTokenHash == nil || *c.ConsentTokenHash != a.hash)
	return map[string]any{
		"can_consent":       canConsent,
		"terms_version":     guardianTermsVersion,
		"status":            c.Status,
		"guardian_name":     c.GuardianName,
		"relationship":      c.Relationship,
		"monthly_limit_yen": c.MonthlyLimitYen,
		"allow_publish":     c.AllowPublish,
		"consented_at":      c.ConsentedAt,
		"limit_options":     options,
		"default_limit_yen": defaultGuardianLimitYen,
		// 本人が自分で払う動画添削・追加相談の月の上限。
		"self_pay_limit_yen":         c.SelfPayLimitYen,
		"self_pay_limit_options":     selfPayLimitOptions,
		"default_self_pay_limit_yen": defaultSelfPayLimitYen,
	}
}

func (s *Server) getGuardian(w http.ResponseWriter, r *http.Request) {
	a, ok := s.guardianByToken(w, r)
	if !ok {
		return
	}
	JSON(w, http.StatusOK, guardianPublicJSON(a))
}

// parseGuardianLimit は月の上限を読む。項目がなければ既定の3,000円、null なら上限なし。
func parseGuardianLimit(raw json.RawMessage) (*int32, bool) {
	if len(raw) == 0 {
		v := defaultGuardianLimitYen
		return &v, true
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, true
	}
	var v int32
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, false
	}
	for _, option := range guardianLimitOptions {
		if v == option {
			return &v, true
		}
	}
	return nil, false
}

// parseSelfPayLimit は、本人が自分で払う分の月の上限を読む。項目がなければ fallback。候補にない値と null は不正。
func parseSelfPayLimit(raw json.RawMessage, fallback int32) (int32, bool) {
	if len(raw) == 0 {
		return fallback, true
	}
	var v int32
	if err := json.Unmarshal(raw, &v); err != nil {
		return 0, false
	}
	for _, option := range selfPayLimitOptions {
		if v == option {
			return v, true
		}
	}
	return 0, false
}

type guardianConsentReq struct {
	GuardianName    string          `json:"guardian_name"`
	Relationship    string          `json:"relationship"`
	MonthlyLimitYen json.RawMessage `json:"monthly_limit_yen"`
	SelfPayLimitYen json.RawMessage `json:"self_pay_limit_yen"`
	AllowPublish    bool            `json:"allow_publish"`
	Agree           bool            `json:"agree"`
}

// consentGuardian は、保護者が同意する。同意用のリンクからだけ、1つのリンクにつき1回だけできる。
// 同意した日時・IP・同意した文面の版を残し、保護者に確認のメール（上限の変更・撤回のリンク入り）を送る。
func (s *Server) consentGuardian(w http.ResponseWriter, r *http.Request) {
	a, ok := s.guardianByToken(w, r)
	if !ok {
		return
	}
	current := a.consent
	if a.manage {
		Error(w, http.StatusForbidden, "consent_link_required", "同意は、お子さんが送った同意用のリンクからだけできます")
		return
	}
	if current.Status == "consented" {
		Error(w, http.StatusConflict, "already_consented", "すでに同意済みです")
		return
	}
	if current.ConsentTokenHash != nil && *current.ConsentTokenHash == a.hash {
		Error(w, http.StatusConflict, "consent_link_used", "このリンクは同意に使用済みです。もう一度同意する場合は、お子さんに新しいリンクを送ってもらってください")
		return
	}
	var req guardianConsentReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	fields := map[string]string{}
	name := strings.TrimSpace(req.GuardianName)
	switch n := len([]rune(name)); {
	case n == 0:
		fields["guardian_name"] = "保護者のお名前を入力してください"
	case n > 50:
		fields["guardian_name"] = "お名前は50文字以内で入力してください"
	}
	switch req.Relationship {
	case "father", "mother", "other":
	default:
		fields["relationship"] = "続柄を選んでください"
	}
	limit, valid := parseGuardianLimit(req.MonthlyLimitYen)
	if !valid {
		fields["monthly_limit_yen"] = "月の上限を選んでください"
	}
	selfPay, valid := parseSelfPayLimit(req.SelfPayLimitYen, defaultSelfPayLimitYen)
	if !valid {
		fields["self_pay_limit_yen"] = "お子さんが支払う月の上限を選んでください"
	}
	if !req.Agree {
		fields["agree"] = "内容を確認して、同意にチェックを入れてください"
	}
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	relationship := req.Relationship
	ip := clientIP(r)
	terms := guardianTermsVersion
	var consent db.GuardianConsent
	err := s.tx(r.Context(), func(q *db.Queries) error {
		var err error
		consent, err = q.ConsentGuardian(r.Context(), db.ConsentGuardianParams{
			TokenHash:       a.hash,
			GuardianName:    &name,
			Relationship:    &relationship,
			MonthlyLimitYen: limit,
			SelfPayLimitYen: selfPay,
			AllowPublish:    req.AllowPublish,
			ConsentIp:       optionalText(ip),
			TermsVersion:    &terms,
		})
		if err != nil {
			return err
		}
		return q.InsertGuardianConsentEvent(r.Context(), db.InsertGuardianConsentEventParams{
			MinorID: consent.MinorID, Event: "consented", GuardianEmail: consent.GuardianEmail,
			GuardianName: &name, Relationship: &relationship, SelfPayLimitYen: &selfPay,
			TermsVersion: &terms, Ip: optionalText(ip),
		})
	})
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusConflict, "consent_link_used", "このリンクでは同意できません。お子さんに新しいリンクを送ってもらってください")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	s.notifyGuardianConsented(r.Context(), consent)
	s.sendGuardianConsentedMail(r.Context(), consent)
	JSON(w, http.StatusOK, guardianPublicJSON(guardianAccess{hash: a.hash, consent: consent}))
}

// notifyGuardianConsented は、保護者が同意したことを本人に知らせる。
// 届かなくても同意は済んでいるので、失敗は記録だけして返事を遅らせすぎない。
func (s *Server) notifyGuardianConsented(ctx context.Context, consent db.GuardianConsent) {
	s.notify(ctx, consent.MinorID, notifyImportant, push.Message{
		Title: "保護者が同意しました",
		Body:  "動画添削・追加相談を" + guardianLimitText(&consent.SelfPayLimitYen) + "の範囲で購入できます",
		URL:   "/me/coaching",
		Tag:   "guardian-consent",
	}, 24*time.Hour)
}

func guardianLimitText(limit *int32) string {
	if limit == nil {
		return "上限なし"
	}
	return "月" + formatYenNumber(*limit) + "円まで"
}

// formatYenNumber は 3000 を "3,000" にする。
func formatYenNumber(v int32) string {
	digits := strconv.Itoa(int(v))
	var b strings.Builder
	for i, c := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return b.String()
}

// guardianSettingsReq は、同意したあとの設定の変更。送らなかった項目は今のまま。
type guardianSettingsReq struct {
	MonthlyLimitYen json.RawMessage `json:"monthly_limit_yen"`
	SelfPayLimitYen json.RawMessage `json:"self_pay_limit_yen"`
	AllowPublish    *bool           `json:"allow_publish"`
}

// updateGuardian は、保護者が月の上限と公開の可否を変える。同意用・変更用のどちらのリンクからでもできる。
// 本人が自分で払う分の上限の変更は、証跡に残す。
func (s *Server) updateGuardian(w http.ResponseWriter, r *http.Request) {
	a, ok := s.guardianByToken(w, r)
	if !ok {
		return
	}
	if a.consent.Status != "consented" {
		Error(w, http.StatusConflict, "conflict", "同意したあとに変更できます")
		return
	}
	var req guardianSettingsReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	if len(req.MonthlyLimitYen) == 0 && len(req.SelfPayLimitYen) == 0 && req.AllowPublish == nil {
		FieldError(w, map[string]string{"self_pay_limit_yen": "変更する項目を選んでください"})
		return
	}
	fields := map[string]string{}
	var consent db.GuardianConsent
	err := s.tx(r.Context(), func(q *db.Queries) error {
		before, err := q.GetGuardianConsentByMinor(r.Context(), a.consent.MinorID)
		if err != nil {
			return err
		}
		limit := before.MonthlyLimitYen
		if len(req.MonthlyLimitYen) > 0 {
			var valid bool
			if limit, valid = parseGuardianLimit(req.MonthlyLimitYen); !valid {
				fields["monthly_limit_yen"] = "月の上限を選んでください"
			}
		}
		selfPay, valid := parseSelfPayLimit(req.SelfPayLimitYen, before.SelfPayLimitYen)
		if !valid {
			fields["self_pay_limit_yen"] = "お子さんが支払う月の上限を選んでください"
		}
		if len(fields) > 0 {
			return errInvalidSettings
		}
		allowPublish := before.AllowPublish
		if req.AllowPublish != nil {
			allowPublish = *req.AllowPublish
		}
		consent, err = q.UpdateGuardianSettings(r.Context(), db.UpdateGuardianSettingsParams{
			MinorID:         a.consent.MinorID,
			MonthlyLimitYen: limit,
			AllowPublish:    allowPublish,
			SelfPayLimitYen: selfPay,
		})
		if err != nil {
			return err
		}
		if before.SelfPayLimitYen == selfPay {
			return nil
		}
		return q.InsertGuardianConsentEvent(r.Context(), db.InsertGuardianConsentEventParams{
			MinorID: consent.MinorID, Event: "limit_changed", GuardianEmail: consent.GuardianEmail,
			SelfPayLimitYen: &selfPay, PreviousSelfPayLimitYen: &before.SelfPayLimitYen, Ip: optionalText(clientIP(r)),
		})
	})
	if errors.Is(err, errInvalidSettings) {
		FieldError(w, fields)
		return
	}
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusConflict, "conflict", "同意したあとに変更できます")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	a.consent = consent
	JSON(w, http.StatusOK, guardianPublicJSON(a))
}

var errInvalidSettings = errors.New("invalid guardian settings")

// declineGuardian は、同意する前に「同意しない」と答える。同意用のリンクからだけ。
func (s *Server) declineGuardian(w http.ResponseWriter, r *http.Request) {
	a, ok := s.guardianByToken(w, r)
	if !ok {
		return
	}
	if a.manage {
		Error(w, http.StatusConflict, "conflict", "同意を取りやめる場合は「同意を取り消す」を選んでください")
		return
	}
	consent, err := s.q.DeclineGuardian(r.Context(), a.hash)
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusConflict, "conflict", "同意を取りやめる場合は「同意を取り消す」を選んでください")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	recordGuardianEvent(r.Context(), s.q, db.InsertGuardianConsentEventParams{
		MinorID: consent.MinorID, Event: "declined", GuardianEmail: consent.GuardianEmail, Ip: optionalText(clientIP(r)),
	})
	a.consent = consent
	JSON(w, http.StatusOK, guardianPublicJSON(a))
}

// revokeGuardian は、保護者が同意を取り消す。以後の新しい購入は止まり、すでに購入した分はそのまま。
func (s *Server) revokeGuardian(w http.ResponseWriter, r *http.Request) {
	a, ok := s.guardianByToken(w, r)
	if !ok {
		return
	}
	consent, err := s.q.RevokeGuardian(r.Context(), a.consent.MinorID)
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusConflict, "conflict", "まだ同意していません")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	recordGuardianEvent(r.Context(), s.q, db.InsertGuardianConsentEventParams{
		MinorID: consent.MinorID, Event: "revoked", GuardianEmail: consent.GuardianEmail,
		SelfPayLimitYen: &consent.SelfPayLimitYen, Ip: optionalText(clientIP(r)),
	})
	s.notify(r.Context(), consent.MinorID, notifyImportant, push.Message{
		Title: "保護者が同意を取り消しました",
		Body:  "新しい動画添削・追加相談は購入できません。購入済みの分はそのまま使えます",
		URL:   "/me/coaching",
		Tag:   "guardian-consent",
	}, 24*time.Hour)
	a.consent = consent
	JSON(w, http.StatusOK, guardianPublicJSON(a))
}

// signCoachingVideoUpload は、添削の動画を非公開でアップロードするための署名を返す。
func (s *Server) signCoachingVideoUpload(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	s.issueUpload(w, r, p.ID, "動画", func() (cloudinary.UploadSignature, error) {
		return s.cloud.SignedCoachingUpload(time.Now())
	})
}
