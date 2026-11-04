package auth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrSessionInactive は、トークンのセッションが Supabase Auth で有効でないことを表す。
var ErrSessionInactive = errors.New("session inactive")

// Admin は、service role キーで Supabase Auth のユーザーを削除し、セッションの状態を確かめる。
type Admin struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

func (a Admin) Enabled() bool {
	return strings.TrimSpace(a.BaseURL) != "" && strings.TrimSpace(a.APIKey) != ""
}

func (a Admin) DeleteUser(ctx context.Context, id uuid.UUID) error {
	if !a.Enabled() {
		return nil
	}
	endpoint := trimSlash(a.BaseURL) + "/auth/v1/admin/users/" + id.String()
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+a.APIKey)
	req.Header.Set("apikey", a.APIKey)
	res, err := a.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("supabase delete user: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusOK || res.StatusCode == http.StatusNoContent || res.StatusCode == http.StatusNotFound {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
	return fmt.Errorf("supabase delete user: status %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
}

// CheckSession は、アクセストークンのセッションがいまも有効かを Supabase Auth に確かめる。
// /auth/v1/user は、JWT の session_id のセッションが残っていること、ユーザーがいて Ban されていないことを
// 毎回確かめ、欠けていれば 403 を返す。session_id の無いトークンはセッションを確かめないまま通すので、
// 呼び出し側で session_id があることも確かめること。
// 有効でなければ ErrSessionInactive を返す。確かめられなかったときは別のエラーを返す。
func (a Admin) CheckSession(ctx context.Context, accessToken string) error {
	if !a.Enabled() {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, trimSlash(a.BaseURL)+"/auth/v1/user", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("apikey", a.APIKey)
	res, err := a.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("supabase check session: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusOK {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		return fmt.Errorf("%w: status %d: %s", ErrSessionInactive, res.StatusCode, strings.TrimSpace(string(body)))
	}
	return fmt.Errorf("supabase check session: status %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
}

func (a Admin) httpClient() *http.Client {
	if a.HTTP != nil {
		return a.HTTP
	}
	return &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}
