package httpx_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// 海外コーチの指導希望（本体 #542）は1ユーザー1件。選び直しは変更になり、行は増えない。
// 未認証は 401、種別が3択以外は 422。管理側は総数・内訳・一覧が見られる。
func TestOverseasInterestFlow(t *testing.T) {
	h := setup(t)
	pool := openPool(t)
	ctx := context.Background()

	for _, tc := range []struct {
		method, path string
	}{
		{http.MethodGet, "/v1/me/overseas-interest"},
		{http.MethodPut, "/v1/me/overseas-interest"},
		{http.MethodDelete, "/v1/me/overseas-interest"},
		{http.MethodGet, "/v1/admin/overseas-interests"},
	} {
		if rec := doJSON(t, h, tc.method, tc.path, "", nil); rec.Code != http.StatusUnauthorized {
			t.Fatalf("anonymous %s %s = %d", tc.method, tc.path, rec.Code)
		}
	}

	meID := uuid.New()
	me := token(t, meID)
	other := tokenWithProfile(t, uuid.New(), "希望者", true)

	type interest struct {
		Kind string `json:"kind"`
	}
	read := func(tok string) *interest {
		t.Helper()
		rec := doJSON(t, h, http.MethodGet, "/v1/me/overseas-interest", tok, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("get %d %s", rec.Code, rec.Body.String())
		}
		var body struct {
			Interest *interest `json:"interest"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		return body.Interest
	}
	put := func(tok, kind string) (int, string) {
		t.Helper()
		rec := doJSON(t, h, http.MethodPut, "/v1/me/overseas-interest", tok, map[string]string{"kind": kind})
		return rec.Code, rec.Body.String()
	}
	count := func(id string) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM overseas_interests WHERE profile_id = $1`, id).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	if got := read(me); got != nil {
		t.Fatalf("initial %+v", got)
	}
	if code, body := put(me, "plan"); code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid kind %d %s", code, body)
	}
	if code, body := put(me, "monthly"); code != http.StatusOK {
		t.Fatalf("put monthly %d %s", code, body)
	}
	if got := read(me); got == nil || got.Kind != "monthly" {
		t.Fatalf("after monthly %+v", got)
	}
	// 同じ種別の登録を繰り返しても行は増えない（重複登録不可）。
	if code, body := put(me, "monthly"); code != http.StatusOK {
		t.Fatalf("put monthly again %d %s", code, body)
	}
	// 別の種別を選ぶと変更として置き換わる。
	if code, body := put(me, "both"); code != http.StatusOK {
		t.Fatalf("put both %d %s", code, body)
	}
	if got := read(me); got == nil || got.Kind != "both" {
		t.Fatalf("after both %+v", got)
	}
	if n := count(meID.String()); n != 1 {
		t.Fatalf("rows for me = %d", n)
	}
	// 取り消しができる。取り消し済みの取り消しも 200。
	if rec := doJSON(t, h, http.MethodDelete, "/v1/me/overseas-interest", me, nil); rec.Code != http.StatusOK {
		t.Fatalf("delete %d %s", rec.Code, rec.Body.String())
	}
	if got := read(me); got != nil {
		t.Fatalf("after delete %+v", got)
	}
	if rec := doJSON(t, h, http.MethodDelete, "/v1/me/overseas-interest", me, nil); rec.Code != http.StatusOK {
		t.Fatalf("delete again %d %s", rec.Code, rec.Body.String())
	}

	if code, body := put(other, "single"); code != http.StatusOK {
		t.Fatalf("other put single %d %s", code, body)
	}

	// 管理側：管理者でなければ見られない。
	adminID := uuid.New()
	admin := token(t, adminID)
	if rec := doJSON(t, h, http.MethodGet, "/v1/admin/overseas-interests", admin, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin %d %s", rec.Code, rec.Body.String())
	}
	// 管理者の準備（自分の取得でプロフィールを作ってから管理者にする）。
	if got := read(admin); got != nil {
		t.Fatalf("admin initial %+v", got)
	}
	setAdmin(t, adminID)
	rec := doJSON(t, h, http.MethodGet, "/v1/admin/overseas-interests", admin, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin %d %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Total     int64 `json:"total"`
		Breakdown struct {
			Monthly int64 `json:"monthly"`
			Single  int64 `json:"single"`
			Both    int64 `json:"both"`
		} `json:"breakdown"`
		Interests []struct {
			Kind      string `json:"kind"`
			Nickname  string `json:"nickname"`
			CreatedAt string `json:"created_at"`
		} `json:"interests"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	// 自分は取り消し済み・管理者は未登録なので、残りは other の1件だけ。
	if body.Total != 1 || body.Breakdown.Monthly != 0 || body.Breakdown.Single != 1 || body.Breakdown.Both != 0 {
		t.Fatalf("stats %+v", body)
	}
	if len(body.Interests) != 1 || body.Interests[0].Kind != "single" || body.Interests[0].Nickname != "希望者" || body.Interests[0].CreatedAt == "" {
		t.Fatalf("list %+v", body.Interests)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM overseas_interests`)
	})
}
