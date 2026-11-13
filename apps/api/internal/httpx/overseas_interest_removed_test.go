package httpx_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// 海外コーチの指導希望のアンケート（本体 #542）は #615 でコードごと撤去した。
// 旧 API はルート自体が無く（404 か 405）、ログイン済みでも管理者でも登録・閲覧できない。
// テーブル overseas_interests と既存の行は残す（migration は足さない）。
func TestOverseasInterestRoutesRemoved(t *testing.T) {
	h := setup(t)
	pool := openPool(t)
	ctx := context.Background()

	userID := uuid.New()
	user := tokenWithProfile(t, userID, "撤去確認", true)
	adminID := uuid.New()
	admin := tokenWithProfile(t, adminID, "撤去確認管理者", false)
	// 自分の取得でプロフィールを作ってから管理者にする。
	for _, tok := range []string{user, admin} {
		if rec := doJSON(t, h, http.MethodGet, "/v1/me", tok, nil); rec.Code != http.StatusOK {
			t.Fatalf("me %d %s", rec.Code, rec.Body.String())
		}
	}
	setAdmin(t, adminID)

	for _, tok := range []string{"", user, admin} {
		for _, tc := range []struct {
			method, path string
		}{
			{http.MethodGet, "/v1/me/overseas-interest"},
			{http.MethodPut, "/v1/me/overseas-interest"},
			{http.MethodDelete, "/v1/me/overseas-interest"},
			{http.MethodGet, "/v1/admin/overseas-interests"},
		} {
			rec := doJSON(t, h, tc.method, tc.path, tok, map[string]string{"kind": "both"})
			if rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("%s %s = %d %s", tc.method, tc.path, rec.Code, rec.Body.String())
			}
		}
	}

	// テーブルは残っていて、旧 API を叩いても行は増えない。
	var n int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM overseas_interests WHERE profile_id = ANY($1)`, []uuid.UUID{userID, adminID}).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("rows = %d", n)
	}
}
