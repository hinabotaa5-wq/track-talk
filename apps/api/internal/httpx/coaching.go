package httpx

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5"
)

// 18歳未満の月の上限で選べる金額。nil は上限なし。
var guardianLimitOptions = []int32{1000, 3000, 5000}

// 親が上限を選ばなかったときの金額。
const defaultGuardianLimitYen int32 = 3000

// isMinor は、生まれた年月から18歳未満かを判定する。
// 誕生日が分からないので、生まれた月の翌月1日を18歳の誕生日とみなす（大人として扱うのが遅れる側に倒す）。
func isMinor(birthMonth, now time.Time) bool {
	adult := time.Date(birthMonth.Year()+18, birthMonth.Month()+1, 1, 0, 0, 0, 0, tokyo)
	return now.In(tokyo).Before(adult)
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
	today := now.In(tokyo)
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

// issueGuardianLink は、18歳未満の本人が保護者に送る同意のリンクを発行する。
// トークンはこのときだけ返す。出し直すと前のリンクは使えなくなる。
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
	token, hash, err := newGuardianToken()
	if err != nil {
		Error(w, http.StatusInternalServerError, "internal_error", "サーバーエラーが発生しました")
		return
	}
	consent, err := s.q.IssueGuardianLink(r.Context(), db.IssueGuardianLinkParams{MinorID: p.ID, TokenHash: hash})
	if err != nil {
		MapDBError(w, err)
		return
	}
	out := guardianStatusJSON(consent)
	out["token"] = token
	JSON(w, http.StatusCreated, out)
}

func (s *Server) guardianByToken(w http.ResponseWriter, r *http.Request) (string, db.GuardianConsent, bool) {
	token := chi.URLParam(r, "token")
	if len(token) != 43 {
		Error(w, http.StatusNotFound, "not_found", "このリンクは使えません。お子さんに新しいリンクを送ってもらってください")
		return "", db.GuardianConsent{}, false
	}
	hash := hashGuardianToken(token)
	consent, err := s.q.GetGuardianConsentByToken(r.Context(), hash)
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusNotFound, "not_found", "このリンクは使えません。お子さんに新しいリンクを送ってもらってください")
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
	hash, _, ok := s.guardianByToken(w, r)
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
	JSON(w, http.StatusOK, guardianPublicJSON(consent))
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
