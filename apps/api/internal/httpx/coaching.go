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
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/jst"
	"github.com/hinata/track-talk/apps/api/internal/mail"
	"github.com/hinata/track-talk/apps/api/internal/push"
	"github.com/jackc/pgx/v5"
)

// 18歳未満の月の上限で選べる金額。nil は上限なし。
var guardianLimitOptions = []int32{1000, 3000, 5000}

// 親が上限を選ばなかったときの金額。
const defaultGuardianLimitYen int32 = 3000

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
		out["guardian"] = guardianStatusJSON(consent)
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
		if err := s.mail.Send(r.Context(), guardianLinkMail(email, s.cfg.WebBaseURL+"/guardian/"+token), "guardian-link-"+hash); err != nil {
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
	JSON(w, http.StatusCreated, guardianStatusJSON(consent))
}

// guardianLinkMail は、保護者に送る同意のお願い。届く先を間違えていてもよいよう、子のアカウントの情報は書かない。
func guardianLinkMail(to, link string) mail.Message {
	return mail.Message{
		To:      to,
		Subject: "【TrackTalk】お子さんの利用について保護者の同意のお願い",
		Text: "TrackTalk（陸上競技のコミュニティサービス）をご利用中のお子さんが、オンラインコーチング" +
			"（フォーム添削・パーソナル指導）を使うために、保護者の方の同意を求めています。\n\n" +
			"次のページで内容をご確認のうえ、同意するかどうかを選んでください。\n" +
			link + "\n\n" +
			"・このリンクは保護者の方だけが使うものです。お子さんを含め、他の人には渡さないでください。\n" +
			"・お子さんが新しいリンクを送ると、このリンクは使えなくなります。\n" +
			"・心当たりがない場合は、このメールを破棄してください。同意しない限り何も始まりません。\n\n" +
			"TrackTalk\n",
	}
}

func (s *Server) guardianByToken(w http.ResponseWriter, r *http.Request) (string, db.GuardianConsent, bool) {
	token := chi.URLParam(r, "token")
	if len(token) != 43 {
		Error(w, http.StatusNotFound, "not_found", "このリンクは使えません。お子さんに頼んで、新しいリンクをメールで送ってもらってください")
		return "", db.GuardianConsent{}, false
	}
	hash := hashGuardianToken(token)
	consent, err := s.q.GetGuardianConsentByToken(r.Context(), hash)
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusNotFound, "not_found", "このリンクは使えません。お子さんに頼んで、新しいリンクをメールで送ってもらってください")
		return "", db.GuardianConsent{}, false
	}
	if err != nil {
		MapDBError(w, err)
		return "", db.GuardianConsent{}, false
	}
	return hash, consent, true
}

// guardianPublicJSON は保護者のページに返す内容。子のアカウントの情報は含めない。
func guardianPublicJSON(c db.GuardianConsent) map[string]any {
	options := make([]any, 0, len(guardianLimitOptions)+1)
	for _, v := range guardianLimitOptions {
		options = append(options, v)
	}
	options = append(options, nil)
	return map[string]any{
		"status":            c.Status,
		"guardian_name":     c.GuardianName,
		"relationship":      c.Relationship,
		"monthly_limit_yen": c.MonthlyLimitYen,
		"allow_publish":     c.AllowPublish,
		"consented_at":      c.ConsentedAt,
		"limit_options":     options,
		"default_limit_yen": defaultGuardianLimitYen,
	}
}

func (s *Server) getGuardian(w http.ResponseWriter, r *http.Request) {
	_, consent, ok := s.guardianByToken(w, r)
	if !ok {
		return
	}
	JSON(w, http.StatusOK, guardianPublicJSON(consent))
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

type guardianConsentReq struct {
	GuardianName    string          `json:"guardian_name"`
	Relationship    string          `json:"relationship"`
	MonthlyLimitYen json.RawMessage `json:"monthly_limit_yen"`
	AllowPublish    bool            `json:"allow_publish"`
	Agree           bool            `json:"agree"`
}

func (s *Server) consentGuardian(w http.ResponseWriter, r *http.Request) {
	hash, current, ok := s.guardianByToken(w, r)
	if !ok {
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
	if !req.Agree {
		fields["agree"] = "内容を確認して、同意にチェックを入れてください"
	}
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	relationship := req.Relationship
	consent, err := s.q.ConsentGuardian(r.Context(), db.ConsentGuardianParams{
		TokenHash:       hash,
		GuardianName:    &name,
		Relationship:    &relationship,
		MonthlyLimitYen: limit,
		AllowPublish:    req.AllowPublish,
	})
	if err != nil {
		MapDBError(w, err)
		return
	}
	if current.Status != "consented" {
		s.notifyGuardianConsented(r.Context(), consent)
	}
	JSON(w, http.StatusOK, guardianPublicJSON(consent))
}

// notifyGuardianConsented は、保護者が同意したことを本人の端末に知らせる。
// 届かなくても同意は済んでいるので、失敗は記録だけして返事を遅らせすぎない。
func (s *Server) notifyGuardianConsented(ctx context.Context, consent db.GuardianConsent) {
	if !s.push.Configured() {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_, err := s.sendPush(ctx, consent.MinorID, push.Message{
		Title: "保護者が同意しました",
		Body:  "フォーム添削を" + guardianLimitText(consent.MonthlyLimitYen) + "の範囲で頼めます",
		URL:   "/me/coaching",
		Tag:   "guardian-consent",
	}, 24*time.Hour)
	if err != nil {
		slog.Warn("guardian consent push", "err", err)
	}
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

type guardianSettingsReq struct {
	MonthlyLimitYen json.RawMessage `json:"monthly_limit_yen"`
	AllowPublish    bool            `json:"allow_publish"`
}

func (s *Server) updateGuardian(w http.ResponseWriter, r *http.Request) {
	hash, current, ok := s.guardianByToken(w, r)
	if !ok {
		return
	}
	if current.Status != "consented" {
		Error(w, http.StatusConflict, "conflict", "同意したあとに変更できます")
		return
	}
	var req guardianSettingsReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	limit, valid := parseGuardianLimit(req.MonthlyLimitYen)
	if !valid || len(req.MonthlyLimitYen) == 0 {
		FieldError(w, map[string]string{"monthly_limit_yen": "月の上限を選んでください"})
		return
	}
	consent, err := s.q.UpdateGuardianSettings(r.Context(), db.UpdateGuardianSettingsParams{
		TokenHash:       hash,
		MonthlyLimitYen: limit,
		AllowPublish:    req.AllowPublish,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusConflict, "conflict", "同意したあとに変更できます")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, guardianPublicJSON(consent))
}

func (s *Server) declineGuardian(w http.ResponseWriter, r *http.Request) {
	hash, _, ok := s.guardianByToken(w, r)
	if !ok {
		return
	}
	consent, err := s.q.DeclineGuardian(r.Context(), hash)
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusConflict, "conflict", "同意を取りやめる場合は「同意を取り消す」を選んでください")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, guardianPublicJSON(consent))
}

func (s *Server) revokeGuardian(w http.ResponseWriter, r *http.Request) {
	hash, _, ok := s.guardianByToken(w, r)
	if !ok {
		return
	}
	consent, err := s.q.RevokeGuardian(r.Context(), hash)
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusConflict, "conflict", "まだ同意していません")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, guardianPublicJSON(consent))
}

// signCoachingVideoUpload は、添削の動画を非公開でアップロードするための署名を返す。
func (s *Server) signCoachingVideoUpload(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireProfile(w, r); !ok {
		return
	}
	signed, err := s.cloud.SignedCoachingUpload(time.Now())
	if err != nil {
		writeAssetReadyError(w, err, "動画")
		return
	}
	JSON(w, http.StatusOK, signed)
}
