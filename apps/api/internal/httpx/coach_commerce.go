package httpx

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5"
)

// コーチごとの特定商取引法の表記。サポートプランはコーチが販売者なので(ココナラ回答1)、
// プランのページにコーチの表記を出す。コーチ側は原則フル表記(住所・電話の省略は使わない。Q5 回答待ち)。
//
// 表記の要否はコーチの自己申告(既定は表記要)。表記不要にするには、下のチェックリストに全部当てはまると申告する。
// 運営は事業の実態が見えたら表記要に戻せる(戻したら本人は申告し直せない)。
//
// 本名・住所・電話番号は全コーチがプランの公開前に登録する(表記不要でも)。表示用のデータとは別のテーブルに、
// アプリで暗号化して入れ、読み書きはこのファイルの関数だけで行う。公開の API に出すのは表記要のコーチの分だけ。
// 運営が見るのは管理画面からだけで、見るたびに目的と一緒に記録する。

// 登録情報の保持期間(年)。最後の取引が終わった時とコーチをやめた時の遅いほうから数え、過ぎたら日次の処理で消す。
const coachIdentityRetentionYears = 5

// 表記不要のチェックリスト。継続的な事業かどうかの基準は弁護士の確認(Q5系)のあとに差し替える。
// 文面を変えたら版を上げる。上げると、前の版で申告したコーチはそのまま(申告し直しは求めない)。
const commerceChecklistVersion = 1

var commerceExemptChecklist = []string{
	"有料の指導を、TrackTalk の外で事業として続けていない",
	"TrackTalk のプランも、繰り返し収入を得ることを目的にしていない",
	"ホームページや SNS で、有料の指導を宣伝・募集していない",
}

const identityKeyID = "v1"

var errIdentityKeyMissing = errors.New("coach identity key is not configured")

func (s *Server) identityAEAD() (cipher.AEAD, error) {
	if s.cfg.CoachIdentityKey == "" {
		return nil, errIdentityKeyMissing
	}
	key, err := base64.StdEncoding.DecodeString(s.cfg.CoachIdentityKey)
	if err != nil || len(key) != 32 {
		return nil, errors.New("coach identity key must be 32 bytes in base64")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// sealIdentity は、コーチと項目の名前を追加データにして暗号化する(別のコーチ・別の欄への付け替えを防ぐ)。
func sealIdentity(aead cipher.AEAD, coach uuid.UUID, field, value string) ([]byte, error) {
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return aead.Seal(nonce, nonce, []byte(value), []byte(coach.String()+"/"+field)), nil
}

func openIdentity(aead cipher.AEAD, coach uuid.UUID, field string, sealed []byte) (string, error) {
	n := aead.NonceSize()
	if len(sealed) < n {
		return "", errors.New("sealed identity is too short")
	}
	plain, err := aead.Open(nil, sealed[:n], sealed[n:], []byte(coach.String()+"/"+field))
	return string(plain), err
}

type coachIdentity struct {
	LegalName string `json:"legal_name"`
	Address   string `json:"address"`
	Phone     string `json:"phone"`
}

var phonePattern = regexp.MustCompile(`^[0-9+()\- ]{10,20}$`)

func (c *coachIdentity) normalize() map[string]string {
	c.LegalName = strings.TrimSpace(c.LegalName)
	c.Address = strings.TrimSpace(c.Address)
	c.Phone = strings.TrimSpace(c.Phone)
	fields := map[string]string{}
	if e := coachTextError("氏名", c.LegalName, 80); e != "" {
		fields["legal_name"] = e
	}
	if e := coachTextError("住所", c.Address, 200); e != "" {
		fields["address"] = e
	}
	if !phonePattern.MatchString(c.Phone) {
		fields["phone"] = "電話番号を数字とハイフンで入力してください"
	}
	return fields
}

// loadIdentity は、登録情報を復号する。登録が無ければ nil。呼ぶのは本人・運営の閲覧・表記要の公開表示だけ。
func (s *Server) loadIdentity(ctx context.Context, q *db.Queries, coach uuid.UUID) (*coachIdentity, error) {
	row, err := q.GetCoachLegalIdentity(ctx, coach)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	aead, err := s.identityAEAD()
	if err != nil {
		return nil, err
	}
	var out coachIdentity
	for _, f := range []struct {
		name   string
		sealed []byte
		dst    *string
	}{{"legal_name", row.LegalNameEnc, &out.LegalName}, {"address", row.AddressEnc, &out.Address}, {"phone", row.PhoneEnc, &out.Phone}} {
		v, err := openIdentity(aead, coach, f.name, f.sealed)
		if err != nil {
			return nil, err
		}
		*f.dst = v
	}
	return &out, nil
}

// commerceRequired は、コーチの表記が要るか。設定が無ければ既定の「表記要」。
func commerceRequired(ctx context.Context, q *db.Queries, coach uuid.UUID) (bool, db.CoachCommerceSetting, error) {
	st, err := q.GetCoachCommerceSettings(ctx, coach)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, db.CoachCommerceSetting{}, nil
	}
	return st.DisclosureRequired, st, err
}

// publicCommerceJSON は、プランのページに出すコーチの表記。表記不要のコーチの登録情報は返さない。
func (s *Server) publicCommerceJSON(ctx context.Context, q *db.Queries, coach uuid.UUID) (map[string]any, error) {
	required, _, err := commerceRequired(ctx, q, coach)
	if err != nil {
		return nil, err
	}
	if !required {
		return map[string]any{"required": false}, nil
	}
	id, err := s.loadIdentity(ctx, q, coach)
	if err != nil {
		return nil, err
	}
	if id == nil {
		return map[string]any{"required": true, "registered": false}, nil
	}
	return map[string]any{"required": true, "registered": true, "legal_name": id.LegalName, "address": id.Address, "phone": id.Phone}, nil
}

// commerceReadyToPublish は、プランを公開してよいか(全コーチ、公開前に登録情報が要る)。
func (s *Server) commerceReadyToPublish(w http.ResponseWriter, r *http.Request, q *db.Queries, coach uuid.UUID) bool {
	ok, err := q.HasCoachLegalIdentity(r.Context(), coach)
	if err != nil {
		MapDBError(w, err)
		return false
	}
	if !ok {
		FieldError(w, map[string]string{"published": "公開する前に、コーチ専用画面の「特定商取引法の表記」で氏名・住所・電話番号を登録してください"})
		return false
	}
	return true
}

func (s *Server) coachCommerceJSON(ctx context.Context, coach uuid.UUID) (map[string]any, error) {
	required, st, err := commerceRequired(ctx, s.q, coach)
	if err != nil {
		return nil, err
	}
	id, err := s.loadIdentity(ctx, s.q, coach)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"required":           required,
		"forced":             st.ForcedRequiredAt != nil,
		"exempt_declared_at": st.ExemptDeclaredAt,
		"checklist_version":  commerceChecklistVersion,
		"checklist":          commerceExemptChecklist,
		"identity":           id,
	}, nil
}

func (s *Server) writeIdentityKeyError(w http.ResponseWriter, err error) bool {
	if errors.Is(err, errIdentityKeyMissing) {
		Error(w, http.StatusServiceUnavailable, "identity_unavailable", "特定商取引法の表記の登録は準備中です")
		return true
	}
	return false
}

// getCoachCommerce は、コーチ本人が見る自分の表記の設定と登録情報。
func (s *Server) getCoachCommerce(w http.ResponseWriter, r *http.Request) {
	coach, ok := s.approvedCoach(w, r)
	if !ok {
		return
	}
	out, err := s.coachCommerceJSON(r.Context(), coach.ProfileID)
	if err != nil {
		if !s.writeIdentityKeyError(w, err) {
			MapDBError(w, err)
		}
		return
	}
	JSON(w, http.StatusOK, out)
}

// saveCoachIdentity は、本名・住所・電話番号を暗号化して保存する。
func (s *Server) saveCoachIdentity(w http.ResponseWriter, r *http.Request) {
	coach, ok := s.approvedCoach(w, r)
	if !ok {
		return
	}
	var req coachIdentity
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	if fields := req.normalize(); len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	aead, err := s.identityAEAD()
	if err != nil {
		if !s.writeIdentityKeyError(w, err) {
			MapDBError(w, err)
		}
		return
	}
	params := db.UpsertCoachLegalIdentityParams{CoachID: coach.ProfileID, KeyID: identityKeyID}
	for _, f := range []struct {
		name  string
		value string
		dst   *[]byte
	}{{"legal_name", req.LegalName, &params.LegalNameEnc}, {"address", req.Address, &params.AddressEnc}, {"phone", req.Phone, &params.PhoneEnc}} {
		sealed, err := sealIdentity(aead, coach.ProfileID, f.name, f.value)
		if err != nil {
			MapDBError(w, err)
			return
		}
		*f.dst = sealed
	}
	if err := s.q.UpsertCoachLegalIdentity(r.Context(), params); err != nil {
		MapDBError(w, err)
		return
	}
	s.getCoachCommerce(w, r)
}

// declareCoachExemption は、表記の要否の申告。不要にするときは、その版のチェックリストに全部当てはまること。
func (s *Server) declareCoachExemption(w http.ResponseWriter, r *http.Request) {
	coach, ok := s.approvedCoach(w, r)
	if !ok {
		return
	}
	var req struct {
		Exempt           bool   `json:"exempt"`
		ChecklistVersion int    `json:"checklist_version"`
		Checked          []bool `json:"checked"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	if !req.Exempt {
		if _, err := s.q.DeclareCoachRequired(r.Context(), coach.ProfileID); err != nil {
			MapDBError(w, err)
			return
		}
		s.getCoachCommerce(w, r)
		return
	}
	allChecked := req.ChecklistVersion == commerceChecklistVersion && len(req.Checked) == len(commerceExemptChecklist)
	for _, c := range req.Checked {
		allChecked = allChecked && c
	}
	if !allChecked {
		FieldError(w, map[string]string{"checked": "表記を不要にできるのは、すべての項目に当てはまるときだけです"})
		return
	}
	version := int16(commerceChecklistVersion)
	_, err := s.q.DeclareCoachExempt(r.Context(), db.DeclareCoachExemptParams{CoachID: coach.ProfileID, ExemptChecklistVersion: &version})
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusConflict, "commerce_forced", "運営が表記要にしたため、表記不要には変えられません。お問い合わせください")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	s.getCoachCommerce(w, r)
}

// adminViewCoachIdentity は、運営が開示請求・紛争対応のためにコーチの登録情報を見る。見るたびに目的を記録する。
func (s *Server) adminViewCoachIdentity(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	coach, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		Purpose string `json:"purpose"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	req.Purpose = strings.TrimSpace(req.Purpose)
	if e := coachTextError("見る目的", req.Purpose, 200); e != "" {
		FieldError(w, map[string]string{"purpose": e})
		return
	}
	id, err := s.loadIdentity(r.Context(), s.q, coach)
	if err != nil {
		if !s.writeIdentityKeyError(w, err) {
			MapDBError(w, err)
		}
		return
	}
	if id == nil {
		Error(w, http.StatusNotFound, "not_found", "このコーチは登録していません")
		return
	}
	if err := s.q.InsertCoachIdentityAccessLog(r.Context(), db.InsertCoachIdentityAccessLogParams{ID: uuid.New(), CoachID: coach, AdminID: &admin.ID, Purpose: req.Purpose}); err != nil {
		MapDBError(w, err)
		return
	}
	logs, err := s.q.ListCoachIdentityAccessLogs(r.Context(), coach)
	if err != nil {
		MapDBError(w, err)
		return
	}
	required, _, err := commerceRequired(r.Context(), s.q, coach)
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"identity": id, "required": required, "access_logs": logs})
}

// adminForceCoachRequired は、事業の実態が見えたコーチを表記要に戻す(規約に定める運営の権限)。
func (s *Server) adminForceCoachRequired(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	coach, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	if _, err := s.q.ForceCoachRequired(r.Context(), db.ForceCoachRequiredParams{CoachID: coach, ForcedBy: &admin.ID}); err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"required": true})
}
