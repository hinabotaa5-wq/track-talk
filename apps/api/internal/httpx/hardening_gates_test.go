package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

// セッション照合の設定（サービスキー）が欠けているときは、書き込みを通さない。読み取りは通す。
// 設定済みのときと、自前の JWT 鍵で動くローカル実行（URL もキーも無い）は従来どおり。
func TestWriteSessionFailsClosedWhenServiceKeyMissing(t *testing.T) {
	f := &fakeAuth{status: http.StatusOK, body: `{}`}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	h := setupConfig(t, func(cfg *config.Config) {
		cfg.SupabaseURL = srv.URL
		cfg.SupabaseServiceRoleKey = ""
	})
	id := uuid.New()
	tok := tokenWithSession(t, id, uuid.NewString())
	if rec := doJSON(t, h, http.MethodGet, "/v1/me", tok, nil); rec.Code != http.StatusOK {
		t.Fatalf("read %d %s", rec.Code, rec.Body.String())
	}
	rec := doJSON(t, h, http.MethodDelete, "/v1/me", tok, map[string]string{"confirm": "退会する"})
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("write %d %s", rec.Code, rec.Body.String())
	}
	if profileCount(t, id) != 1 {
		t.Fatal("profile changed")
	}

	// 設定済みなら従来どおり通る。
	h2, _ := setupFakeAuth(t, http.StatusOK, `{}`)
	id2 := uuid.New()
	tok2 := tokenWithSession(t, id2, uuid.NewString())
	if rec := doJSON(t, h2, http.MethodGet, "/v1/me", tok2, nil); rec.Code != http.StatusOK {
		t.Fatalf("configured read %d", rec.Code)
	}
	if rec := doJSON(t, h2, http.MethodDelete, "/v1/me", tok2, map[string]string{"confirm": "退会する"}); rec.Code != http.StatusOK {
		t.Fatalf("configured write %d %s", rec.Code, rec.Body.String())
	}
}

// 通話の予約と対面の作成も、ほかの作成経路と同じくメッセージのフラグが閉じている間は受け付けない。
func TestCallAndInPersonCreationFollowMessagingFlag(t *testing.T) {
	for _, open := range []bool{false, true} {
		h := setupConfig(t, func(cfg *config.Config) { cfg.CoachingMessaging = open })
		tok := token(t, uuid.New())
		for _, path := range []string{"/calls", "/in-person"} {
			rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts/"+uuid.NewString()+path, tok, map[string]any{})
			closed := rec.Code == http.StatusConflict && strings.Contains(rec.Body.String(), "messaging_closed")
			if closed == open {
				t.Fatalf("messaging=%v %s: %d %s", open, path, rec.Code, rec.Body.String())
			}
		}
	}
}
