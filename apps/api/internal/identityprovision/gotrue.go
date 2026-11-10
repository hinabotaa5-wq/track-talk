package identityprovision

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

// authClient は金庫の Supabase Auth（GoTrue）への呼び出し。応答の本文・エラーの原文は呼び出し側へ返さない
// （設定キーやトークンを含みうるため）。失敗は固定の分類名とステータスだけにする。
type authClient struct {
	base      string // 例：https://<金庫>.supabase.co/auth/v1
	adminKey  string // 金庫 Auth の管理用キー（ジョブの秘密参照からだけ）
	publicKey string // 金庫 Auth の公開キー
	http      *http.Client
	now       func() time.Time
}

type authError struct {
	Op     string
	Status int
}

func (e *authError) Error() string { return fmt.Sprintf("auth_%s_failed_%d", e.Op, e.Status) }

func (c *authClient) do(ctx context.Context, op, method, path string, bearer, apikey string, in, out any) (int, error) {
	var body io.Reader
	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			return 0, err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.base, "/")+path, body)
	if err != nil {
		return 0, &authError{Op: op}
	}
	req.Header.Set("apikey", apikey)
	req.Header.Set("Authorization", "Bearer "+bearer)
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := c.http
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, &authError{Op: op}
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		return res.StatusCode, &authError{Op: op, Status: res.StatusCode}
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return res.StatusCode, &authError{Op: op, Status: res.StatusCode}
		}
	}
	return res.StatusCode, nil
}

func (c *authClient) admin(ctx context.Context, op, method, path string, in, out any) (int, error) {
	return c.do(ctx, op, method, path, c.adminKey, c.adminKey, in, out)
}

type authFactor struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	FactorType string `json:"factor_type"`
}

type authUser struct {
	ID        uuid.UUID    `json:"id"`
	Email     string       `json:"email"`
	DeletedAt *time.Time   `json:"deleted_at"`
	Factors   []authFactor `json:"factors"`
}

// findUser は、メールが完全一致する金庫 Auth のユーザーを探す（大文字小文字は区別しない）。無ければ nil。
func (c *authClient) findUser(ctx context.Context, email string) (*authUser, error) {
	want := strings.ToLower(strings.TrimSpace(email))
	for page := 1; page <= 50; page++ {
		var out struct {
			Users []authUser `json:"users"`
		}
		q := url.Values{"page": {fmt.Sprint(page)}, "per_page": {"100"}, "filter": {want}}
		if _, err := c.admin(ctx, "list_users", http.MethodGet, "/admin/users?"+q.Encode(), nil, &out); err != nil {
			return nil, err
		}
		for i := range out.Users {
			if strings.ToLower(out.Users[i].Email) == want {
				return &out.Users[i], nil
			}
		}
		if len(out.Users) < 100 {
			return nil, nil
		}
	}
	return nil, &authError{Op: "list_users"}
}

func (c *authClient) getUser(ctx context.Context, id uuid.UUID) (*authUser, error) {
	var out authUser
	status, err := c.admin(ctx, "get_user", http.MethodGet, "/admin/users/"+id.String(), nil, &out)
	if status == http.StatusNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func randomPassword() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// createUser は、メール確認済みの合成ユーザーを作る。パスワードはこの実行の中でだけ使う乱数。
func (c *authClient) createUser(ctx context.Context, email string) (uuid.UUID, error) {
	password, err := randomPassword()
	if err != nil {
		return uuid.Nil, err
	}
	var out authUser
	if _, err := c.admin(ctx, "create_user", http.MethodPost, "/admin/users",
		map[string]any{"email": email, "password": password, "email_confirm": true}, &out); err != nil {
		return uuid.Nil, err
	}
	return out.ID, nil
}

func (c *authClient) deleteUser(ctx context.Context, id uuid.UUID) (bool, error) {
	status, err := c.admin(ctx, "delete_user", http.MethodDelete, "/admin/users/"+id.String(), nil, nil)
	if status == http.StatusNotFound {
		return false, nil
	}
	return err == nil, err
}

// session は、合成ユーザーで二段階認証（aal2）まで済ませたアクセストークンを返す。
// 毎回パスワードを乱数で付け直し、既存の要素を消して TOTP を登録し直す。設定キー・コード・パスワードは
// この関数の中だけにあり、返すのはアクセストークンだけ（呼び出し側も出力しない）。
func (c *authClient) session(ctx context.Context, id uuid.UUID, email string) (string, error) {
	password, err := randomPassword()
	if err != nil {
		return "", err
	}
	if _, err := c.admin(ctx, "set_password", http.MethodPut, "/admin/users/"+id.String(), map[string]any{"password": password}, nil); err != nil {
		return "", err
	}
	user, err := c.getUser(ctx, id)
	if err != nil || user == nil {
		return "", &authError{Op: "get_user"}
	}
	for _, f := range user.Factors {
		if _, err := c.admin(ctx, "delete_factor", http.MethodDelete, "/admin/users/"+id.String()+"/factors/"+url.PathEscape(f.ID), nil, nil); err != nil {
			return "", err
		}
	}
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	if _, err := c.do(ctx, "password_login", http.MethodPost, "/token?grant_type=password", c.publicKey, c.publicKey,
		map[string]string{"email": email, "password": password}, &tok); err != nil {
		return "", err
	}
	var enrolled struct {
		ID   string `json:"id"`
		TOTP struct {
			Secret string `json:"secret"`
		} `json:"totp"`
	}
	if _, err := c.do(ctx, "totp_enroll", http.MethodPost, "/factors", tok.AccessToken, c.publicKey,
		map[string]string{"factor_type": "totp", "friendly_name": "synthetic-" + uuid.NewString()[:8]}, &enrolled); err != nil {
		return "", err
	}
	if enrolled.ID == "" || enrolled.TOTP.Secret == "" {
		return "", &authError{Op: "totp_enroll"}
	}
	var challenge struct {
		ID string `json:"id"`
	}
	if _, err := c.do(ctx, "totp_challenge", http.MethodPost, "/factors/"+url.PathEscape(enrolled.ID)+"/challenge", tok.AccessToken, c.publicKey, map[string]string{}, &challenge); err != nil {
		return "", err
	}
	now := time.Now
	if c.now != nil {
		now = c.now
	}
	code, err := totpCode(enrolled.TOTP.Secret, now())
	enrolled.TOTP.Secret = ""
	if err != nil {
		return "", &authError{Op: "totp_code"}
	}
	var verified struct {
		AccessToken string `json:"access_token"`
	}
	if _, err := c.do(ctx, "totp_verify", http.MethodPost, "/factors/"+url.PathEscape(enrolled.ID)+"/verify", tok.AccessToken, c.publicKey,
		map[string]string{"challenge_id": challenge.ID, "code": code}, &verified); err != nil {
		return "", err
	}
	if verified.AccessToken == "" {
		return "", errors.New("auth_totp_verify_failed")
	}
	return verified.AccessToken, nil
}
