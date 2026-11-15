package httpx_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// 失効したセッションの書き込みは次のリクエストから止まり、読み取りは通る。
// 保護者リンク・/internal・Webhook の書き込みにはセッションの確認が掛からない。
func TestWriteSessionEnforcedOnAPI(t *testing.T) {
	h, f := setupFakeAuth(t, http.StatusForbidden, `{"code":403,"error_code":"session_not_found"}`)
	id := uuid.New()
	tok := tokenWithSession(t, id, uuid.NewString())

	// 対照：書き込みは 401 session_inactive で止まる(代役が1回だけ呼ばれる)。
	rec := doJSON(t, h, http.MethodPost, "/v1/posts", tok, map[string]any{"kind": "text", "body": "hello"})
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), `"session_inactive"`) {
		t.Fatalf("write %d %s", rec.Code, rec.Body.String())
	}
	if len(f.checked) != 1 {
		t.Fatalf("checked %d times", len(f.checked))
	}

	// 読み取りは確認なしで通る。
	if rec := doJSON(t, h, http.MethodGet, "/v1/me", tok, nil); rec.Code != http.StatusOK {
		t.Fatalf("read %d %s", rec.Code, rec.Body.String())
	}
	if len(f.checked) != 1 {
		t.Fatalf("checked %d times", len(f.checked))
	}

	// 保護者の同意(書き込み)は、失効した利用者トークンを付けても session_inactive にならない。
	rec = doJSON(t, h, http.MethodPost, "/v1/guardian/"+uuid.NewString()+"/consent", tok, map[string]any{"agree": true})
	if strings.Contains(rec.Body.String(), `"session_inactive"`) {
		t.Fatalf("guardian %d %s", rec.Code, rec.Body.String())
	}
	// Stripe の Webhook(書き込み)も session_inactive にならない。
	rec = doJSON(t, h, http.MethodPost, "/webhooks/stripe", tok, map[string]any{"id": "evt_test"})
	if strings.Contains(rec.Body.String(), `"session_inactive"`) {
		t.Fatalf("webhook %d %s", rec.Code, rec.Body.String())
	}
	// 内部の定期実行(書き込み)も session_inactive にならない。
	rec = doJSON(t, h, http.MethodPost, "/internal/monthly-reminder", tok, nil)
	if strings.Contains(rec.Body.String(), `"session_inactive"`) {
		t.Fatalf("internal %d %s", rec.Code, rec.Body.String())
	}
	if len(f.checked) != 1 {
		t.Fatalf("checked %d times", len(f.checked))
	}
}

// 有効なセッションの書き込みは従来どおり通る。
func TestWriteSessionActivePasses(t *testing.T) {
	h, f := setupFakeAuth(t, http.StatusOK, `{}`)
	id := uuid.New()
	tok := tokenWithSession(t, id, uuid.NewString())
	rec := doJSON(t, h, http.MethodPatch, "/v1/me/badge", tok, map[string]any{"post_master_badge_public": false})
	if rec.Code != http.StatusOK {
		t.Fatalf("write %d %s", rec.Code, rec.Body.String())
	}
	if len(f.checked) != 1 {
		t.Fatalf("checked %d times", len(f.checked))
	}
}
