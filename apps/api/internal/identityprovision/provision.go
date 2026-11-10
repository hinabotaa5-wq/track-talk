package identityprovision

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Querier は pgxpool.Pool と pgx.Conn の共通部分。
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Runner は合成プロビジョニングの各操作。Main は本体 DB（身元専用サービスの本体読み取りロール）、
// Vault は金庫 DB（身元専用サービスのアプリ用ロール）。
type Runner struct {
	Main        Querier
	Vault       Querier
	Auth        *authClient
	Base        string
	IdentityURL string // 身元専用サービス（walkthrough だけ）
	PublicURL   string // 公開 API（walkthrough だけ）
	HTTP        *http.Client
}

// NewAuth は金庫 Auth のクライアントを作る。base は …/auth/v1 まで。
func NewAuth(base, adminKey, publicKey string) (*authClient, error) {
	if !strings.HasPrefix(base, "https://") && !strings.HasPrefix(base, "http://") || adminKey == "" || publicKey == "" || adminKey == publicKey {
		return nil, errors.New("auth_configuration_invalid")
	}
	return &authClient{base: base, adminKey: adminKey, publicKey: publicKey}, nil
}

// Result は1行の JSON として出す結果。値は操作名・分類名・真偽・件数・HTTP ステータスだけ。
type Result map[string]any

// 拒否・失敗の分類名。
const (
	errEmail      = "synthetic_email_rejected"
	errMainGuard  = "coach_not_approved_or_email_mismatch"
	errDeleted    = "synthetic_user_deleted"
	errSelfCheck  = "self_check_failed"
	errNotBound   = "synthetic_binding_missing"
	errNotSynth   = "binding_not_synthetic"
	errDatabase   = "database_error"
	errAuthPrefix = "auth_"
)

// vault の関数が出す固定の分類名（それ以外の DB エラーは database_error に丸める）。
var vaultCodes = map[string]bool{
	"synthetic_invalid_input": true, "synthetic_subject_required": true,
	"synthetic_subject_bound_elsewhere": true, "synthetic_coach_bound_elsewhere": true,
}

func classify(err error) string {
	var ae *authError
	if errors.As(err, &ae) {
		return ae.Error()
	}
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe.Code == "P0001" && vaultCodes[pe.Message] {
		return pe.Message
	}
	if err != nil && strings.HasPrefix(err.Error(), errAuthPrefix) {
		return err.Error()
	}
	return errDatabase
}

func fail(r Result, code string) Result {
	r["result"] = "failed"
	r["error"] = code
	return r
}

// guard は、合成メールの形と本体側のガード（承認済み・メール一致）を確かめる。
func (rn *Runner) guard(ctx context.Context, coach uuid.UUID, tag string) (string, bool, error) {
	email, err := SyntheticEmail(rn.Base, tag)
	if err != nil || !IsSynthetic(rn.Base, email) {
		return "", false, errors.New(errEmail)
	}
	var matched bool
	if err := rn.Main.QueryRow(ctx, "SELECT identity_boundary.coach_matches_email($1, $2)", coach, email).Scan(&matched); err != nil {
		return email, false, err
	}
	return email, matched, nil
}

type bindingStatus struct {
	MemberRows, AdminRows, IdentityRows, CommerceRows, DisclosureRows, AccessLogRows int
	SubjectMatches, SubjectSynthetic                                                 bool
}

func (rn *Runner) status(ctx context.Context, coach, subject uuid.UUID) (bindingStatus, error) {
	var s bindingStatus
	err := rn.Vault.QueryRow(ctx, `SELECT member_rows, subject_matches, subject_synthetic, admin_rows, identity_rows,
		commerce_rows, disclosure_rows, access_log_rows FROM identity_vault.synthetic_status($1, $2)`, coach, subject).
		Scan(&s.MemberRows, &s.SubjectMatches, &s.SubjectSynthetic, &s.AdminRows, &s.IdentityRows, &s.CommerceRows, &s.DisclosureRows, &s.AccessLogRows)
	return s, err
}

func (s bindingStatus) bound() bool {
	return s.MemberRows == 1 && s.SubjectMatches && s.SubjectSynthetic && s.AdminRows == 0
}

func (s bindingStatus) put(r Result) {
	r["member_rows"] = s.MemberRows
	r["subject_matches"] = s.SubjectMatches
	r["subject_synthetic"] = s.SubjectSynthetic
	r["admin_rows"] = s.AdminRows
	r["identity_rows"] = s.IdentityRows
	r["commerce_rows"] = s.CommerceRows
	r["disclosure_rows"] = s.DisclosureRows
	r["access_log_rows"] = s.AccessLogRows
}

// Provision は、金庫 Auth の合成ユーザー（無ければ作る）と、その ID を subject とする members 行
// （is_admin=false、coach_id＝入力）を用意し、自己検証まで行う。冪等。
func (rn *Runner) Provision(ctx context.Context, coach uuid.UUID, tag string) Result {
	r := Result{"op": "provision"}
	email, matched, err := rn.guard(ctx, coach, tag)
	if err != nil {
		if err.Error() == errEmail {
			return fail(r, errEmail)
		}
		return fail(r, classify(err))
	}
	if !matched {
		return fail(r, errMainGuard)
	}
	user, err := rn.Auth.findUser(ctx, email)
	if err != nil {
		return fail(r, classify(err))
	}
	r["created_user"] = false
	var subject uuid.UUID
	switch {
	case user == nil:
		if subject, err = rn.Auth.createUser(ctx, email); err != nil {
			return fail(r, classify(err))
		}
		r["created_user"] = true
	case user.DeletedAt != nil:
		return fail(r, errDeleted)
	default:
		subject = user.ID
	}
	var binding string
	if err := rn.Vault.QueryRow(ctx, "SELECT identity_vault.synthetic_bind($1, $2)", subject, coach).Scan(&binding); err != nil {
		return fail(r, classify(err))
	}
	r["binding"] = binding
	s, err := rn.status(ctx, coach, subject)
	if err != nil {
		return fail(r, classify(err))
	}
	s.put(r)
	if !s.bound() {
		return fail(r, errSelfCheck)
	}
	r["result"] = "ok"
	return r
}

// Verify は、members 行の有無・subject 一致・coach_id 一致・is_admin=false を、真偽と件数だけで返す。
func (rn *Runner) Verify(ctx context.Context, coach uuid.UUID, tag string) Result {
	r := Result{"op": "verify"}
	email, matched, err := rn.guard(ctx, coach, tag)
	if err != nil {
		if err.Error() == errEmail {
			return fail(r, errEmail)
		}
		return fail(r, classify(err))
	}
	r["main_guard"] = matched
	user, err := rn.Auth.findUser(ctx, email)
	if err != nil {
		return fail(r, classify(err))
	}
	subject := uuid.Nil
	r["auth_user_exists"] = user != nil && user.DeletedAt == nil
	if user != nil && user.DeletedAt == nil {
		subject = user.ID
	}
	s, err := rn.status(ctx, coach, subject)
	if err != nil {
		return fail(r, classify(err))
	}
	s.put(r)
	// coach_id 一致は、synthetic_status が coach_id で行を引いているので member_rows=1 が意味する。
	if matched && s.bound() && subject != uuid.Nil {
		r["result"] = "match"
	} else {
		r["result"] = "mismatch"
	}
	return r
}

// Cleanup は、合成の割り当てと、そのコーチの合成の身元行、合成の金庫 Auth ユーザーを消し、件数だけを返す。
// 割り当ての subject が合成でない・管理者のときは何も消さずに止まる（金庫側の関数でも同じ条件を強制する）。
func (rn *Runner) Cleanup(ctx context.Context, coach uuid.UUID, tag string) Result {
	r := Result{"op": "cleanup"}
	email, err := SyntheticEmail(rn.Base, tag)
	if err != nil || !IsSynthetic(rn.Base, email) {
		return fail(r, errEmail)
	}
	user, err := rn.Auth.findUser(ctx, email)
	if err != nil {
		return fail(r, classify(err))
	}
	subject := uuid.Nil
	if user != nil {
		subject = user.ID
	}
	before, err := rn.status(ctx, coach, subject)
	if err != nil {
		return fail(r, classify(err))
	}
	if before.AdminRows != 0 || (before.MemberRows > 0 && !before.SubjectSynthetic) {
		return fail(r, errNotSynth)
	}
	var members, identities, commerce, disclosures int
	var unbound *uuid.UUID
	if err := rn.Vault.QueryRow(ctx, `SELECT member_rows, identity_rows, commerce_rows, disclosure_rows, unbound_subject
		FROM identity_vault.synthetic_unbind($1)`, coach).Scan(&members, &identities, &commerce, &disclosures, &unbound); err != nil {
		return fail(r, classify(err))
	}
	r["deleted_member_rows"] = members
	r["deleted_identity_rows"] = identities
	r["deleted_commerce_rows"] = commerce
	r["deleted_disclosure_rows"] = disclosures
	deleted := 0
	targets := []uuid.UUID{}
	if user != nil {
		targets = append(targets, user.ID)
	}
	if unbound != nil && (user == nil || *unbound != user.ID) {
		targets = append(targets, *unbound)
	}
	for _, id := range targets {
		u, err := rn.Auth.getUser(ctx, id)
		if err != nil {
			return fail(r, classify(err))
		}
		// 消すのは合成の標識（@ より前の +synthetic）を持つユーザーだけ。管理者・実在の利用者には触れない。
		if u == nil || !syntheticLocalPart(u.Email) {
			continue
		}
		var admin bool
		if err := rn.Vault.QueryRow(ctx, "SELECT admin_rows > 0 FROM identity_vault.synthetic_status($1, $2)", uuid.Nil, id).Scan(&admin); err != nil {
			return fail(r, classify(err))
		}
		if admin {
			return fail(r, errNotSynth)
		}
		ok, err := rn.Auth.deleteUser(ctx, id)
		if err != nil {
			return fail(r, classify(err))
		}
		if ok {
			deleted++
		}
	}
	r["deleted_auth_users"] = deleted
	after, err := rn.status(ctx, coach, uuid.Nil)
	if err != nil {
		return fail(r, classify(err))
	}
	r["remaining_member_rows"] = after.MemberRows
	r["remaining_identity_rows"] = after.IdentityRows
	if after.MemberRows != 0 || after.IdentityRows != 0 {
		return fail(r, errSelfCheck)
	}
	r["result"] = "ok"
	return r
}

func syntheticLocalPart(email string) bool {
	local, _, ok := strings.Cut(strings.ToLower(email), "@")
	return ok && strings.Contains(local, "+synthetic")
}

// 架空の身元。毎回ランダムな印を入れ、公開 API の応答に含まれないことを部分一致で確かめる（値は出力しない）。
func fictitiousIdentity() (map[string]string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return nil, err
	}
	mark := fmt.Sprintf("%06d", n.Int64())
	return map[string]string{
		"legal_name": "架空 合成" + mark,
		"address":    "架空県架空市合成町" + mark + "番地",
		"phone":      "000-" + mark[:4] + "-" + mark[2:],
	}, nil
}

func (rn *Runner) request(ctx context.Context, method, url, bearer string, body any) (int, []byte) {
	var reader io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return 0, nil
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := rn.HTTP
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	return res.StatusCode, raw
}

func containsAny(body []byte, values map[string]string) bool {
	for _, v := range values {
		if bytes.Contains(body, []byte(v)) {
			return true
		}
	}
	return false
}

// Walkthrough は、合成コーチで本番経路を通す（#62 手順4）：二段階認証まで済ませたセッション→身元画面の
// 読み取り→架空の身元の登録→読み戻しの一致→公開 API に氏名・住所・電話が出ないこと→管理者側の前提（件数）。
// 結果は真偽・件数・HTTP ステータスだけ。架空の値・トークンは出力しない。
func (rn *Runner) Walkthrough(ctx context.Context, coach uuid.UUID, tag string) Result {
	r := Result{"op": "walkthrough"}
	if rn.IdentityURL == "" || rn.PublicURL == "" {
		return fail(r, "walkthrough_configuration_missing")
	}
	email, matched, err := rn.guard(ctx, coach, tag)
	if err != nil {
		if err.Error() == errEmail {
			return fail(r, errEmail)
		}
		return fail(r, classify(err))
	}
	if !matched {
		return fail(r, errMainGuard)
	}
	user, err := rn.Auth.findUser(ctx, email)
	if err != nil {
		return fail(r, classify(err))
	}
	if user == nil || user.DeletedAt != nil {
		return fail(r, errNotBound)
	}
	before, err := rn.status(ctx, coach, user.ID)
	if err != nil {
		return fail(r, classify(err))
	}
	if !before.bound() {
		return fail(r, errNotBound)
	}
	token, err := rn.Auth.session(ctx, user.ID, email)
	if err != nil {
		return fail(r, classify(err))
	}
	r["aal2_session"] = true
	identityBase := strings.TrimRight(rn.IdentityURL, "/")
	status, _ := rn.request(ctx, http.MethodGet, identityBase+"/v1/coach", token, nil)
	r["coach_view_status"] = status
	fake, err := fictitiousIdentity()
	if err != nil {
		return fail(r, errDatabase)
	}
	status, _ = rn.request(ctx, http.MethodPut, identityBase+"/v1/coach/identity", token, fake)
	r["identity_save_status"] = status
	status, body := rn.request(ctx, http.MethodGet, identityBase+"/v1/coach", token, nil)
	var view struct {
		Identity map[string]string `json:"identity"`
		Revision *uuid.UUID        `json:"identity_revision"`
	}
	readBack := status == http.StatusOK && json.Unmarshal(body, &view) == nil && view.Revision != nil
	if readBack {
		for k, v := range fake {
			if view.Identity[k] != v {
				readBack = false
			}
		}
	}
	r["identity_read_back_matches"] = readBack
	// 公開 API：コーチの公開ページで使う経路。どの応答にも架空の氏名・住所・電話が含まれないこと。
	publicBase := strings.TrimRight(rn.PublicURL, "/")
	exposed := false
	statuses := map[string]int{}
	for name, path := range map[string]string{
		"coaches":            "/v1/coaches",
		"coaches_support":    "/v1/coaches/support-plan",
		"coach_availability": "/v1/coaches/" + coach.String() + "/availability",
		"coach_consult":      "/v1/coaches/" + coach.String() + "/support-consult",
	} {
		status, body := rn.request(ctx, http.MethodGet, publicBase+path, "", nil)
		statuses[name] = status
		if containsAny(body, fake) {
			exposed = true
		}
	}
	r["public_statuses"] = statuses
	r["public_api_exposes_identity"] = exposed
	var admins, adminsMFA int
	if err := rn.Vault.QueryRow(ctx, "SELECT admin_rows, admins_with_verified_mfa FROM identity_vault.admin_readiness()").Scan(&admins, &adminsMFA); err != nil {
		return fail(r, classify(err))
	}
	r["admin_member_rows"] = admins
	r["admins_with_verified_mfa"] = adminsMFA
	after, err := rn.status(ctx, coach, user.ID)
	if err != nil {
		return fail(r, classify(err))
	}
	r["identity_rows"] = after.IdentityRows
	r["access_log_rows"] = after.AccessLogRows
	r["subject_matches"] = after.SubjectMatches
	ok := r["coach_view_status"] == http.StatusOK && r["identity_save_status"] == http.StatusNoContent && readBack &&
		!exposed && after.IdentityRows == 1 && admins >= 1 && adminsMFA == admins && after.bound()
	if ok {
		r["result"] = "ok"
	} else {
		r["result"] = "failed"
		r["error"] = "walkthrough_check_failed"
	}
	return r
}
