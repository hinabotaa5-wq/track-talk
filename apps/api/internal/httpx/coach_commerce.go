package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

var errIdentityUnavailable = errors.New("identity service unavailable")

type identityReader struct {
	url   string
	token string
	end   string
	http  *http.Client
}

func newIdentityReader(base, token, endToken string) identityReader {
	return identityReader{url: strings.TrimRight(base, "/"), token: token, end: endToken, http: &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}}
}

// notifyEnd は、見送り確定のコーチの終了だけを専用サービスへ知らせる。
// 409（承認済みで終了対象外）は再試行しても変わらないので完了扱いにする。
// 未設定（専用サービスを置かない開発時）は警告だけ残し、終了の記録自体は
// 審査確定時の退役トリガが担う。
func (c identityReader) notifyEnd(ctx context.Context, coachID uuid.UUID) error {
	if c.url == "" || c.end == "" {
		slog.Warn("identity end notifier is not configured")
		return nil
	}
	body, err := json.Marshal(map[string]string{"coach_id": coachID.String()})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url+"/internal/coach-end", bytes.NewReader(body))
	if err != nil {
		return errIdentityUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+c.end)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return errIdentityUnavailable
	}
	defer res.Body.Close()
	switch res.StatusCode {
	case http.StatusNoContent, http.StatusConflict:
		return nil
	default:
		return errIdentityUnavailable
	}
}

func (c identityReader) get(ctx context.Context, path string, dst any) (bool, error) {
	if c.url == "" || c.token == "" {
		return false, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url+path, nil)
	if err != nil {
		return false, errIdentityUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	res, err := c.http.Do(req)
	if err != nil {
		return false, errIdentityUnavailable
	}
	defer res.Body.Close()
	switch {
	case res.StatusCode == http.StatusNotFound:
		return false, nil
	case dst == nil && res.StatusCode == http.StatusNoContent:
		return true, nil
	case dst != nil && res.StatusCode == http.StatusOK:
		if err := json.NewDecoder(io.LimitReader(res.Body, 4096)).Decode(dst); err == nil {
			return true, nil
		}
	}
	return false, errIdentityUnavailable
}

type publicCommerce struct {
	Required   bool   `json:"required"`
	Registered bool   `json:"registered,omitempty"`
	LegalName  string `json:"legal_name,omitempty"`
	Address    string `json:"address,omitempty"`
	Phone      string `json:"phone,omitempty"`
}

func (c identityReader) disclosure(ctx context.Context, plan uuid.UUID) (publicCommerce, bool, error) {
	var out publicCommerce
	ok, err := c.get(ctx, "/internal/disclosures/"+url.PathEscape(plan.String()), &out)
	if err != nil || !ok {
		return publicCommerce{}, ok, err
	}
	if !out.Required {
		return publicCommerce{}, true, nil
	}
	if !out.Registered || out.LegalName == "" || out.Address == "" || out.Phone == "" {
		return publicCommerce{}, false, errIdentityUnavailable
	}
	return out, true, nil
}

func (s *Server) commerceReadyToPublish(w http.ResponseWriter, r *http.Request, coach uuid.UUID) bool {
	ok, err := s.identity.get(r.Context(), "/internal/registration/"+url.PathEscape(coach.String()), nil)
	if err != nil {
		identityServiceError(w)
		return false
	}
	if !ok {
		FieldError(w, map[string]string{"published": "先に身元情報の専用画面で氏名・住所・電話番号を登録してください"})
	}
	return ok
}

func identityServiceError(w http.ResponseWriter) {
	Error(w, http.StatusServiceUnavailable, "identity_unavailable", "特定商取引法の表記を確認できません。時間をおいて再度お試しください")
}

func (c identityReader) approvedPlans(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	out := make(map[uuid.UUID]bool)
	for start := 0; start < len(ids); start += 100 {
		end := min(start+100, len(ids))
		parts := make([]string, 0, end-start)
		for _, id := range ids[start:end] {
			parts = append(parts, id.String())
		}
		var approved []uuid.UUID
		if _, err := c.get(ctx, "/internal/disclosures?ids="+strings.Join(parts, ","), &approved); err != nil {
			return nil, err
		}
		for _, id := range approved {
			out[id] = true
		}
	}
	return out, nil
}
