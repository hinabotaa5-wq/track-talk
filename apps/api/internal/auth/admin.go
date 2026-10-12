package auth

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Admin deletes Supabase Auth users with the service role key.
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
