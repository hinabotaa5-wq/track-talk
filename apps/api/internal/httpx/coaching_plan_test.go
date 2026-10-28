package httpx_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
)

func planBody() map[string]any {
	return map[string]any{
		"kind":          "period",
		"title":         "1か月で100mのスタートを直す",
		"description":   "最初に動画を見て、1か月分の練習メニューを作ります。週に1回、動画に返答します。",
		"duration_days": 30,
		"price_yen":     8000,
		"capacity":      3,
	}
}

func TestCoachingPlans(t *testing.T) {
	h := setup(t)
	coach := uuid.New()
	tc := token(t, coach)

	// 承認されていないとプランは作れない。
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/me/plans", tc, planBody()); rec.Code != http.StatusForbidden {
		t.Fatalf("not coach %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", tc, coachApplicationBody()); rec.Code != http.StatusOK {
		t.Fatalf("apply %d", rec.Code)
	}
	withCoachPhoto(t, coach)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+coach.String()+"/approve", adminToken(t, h), nil); rec.Code != http.StatusOK {
		t.Fatalf("approve %d", rec.Code)
	}

	// 誓約するまでは作れない。
	rec := doJSON(t, h, http.MethodPost, "/v1/coach/me/plans", tc, planBody())
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "pledge_required") {
		t.Fatalf("no pledge %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/me/pledge", tc, map[string]any{"agree": false}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("pledge without agree %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/coach/me/pledge", tc, map[string]any{"agree": true})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"pledged":true`) {
		t.Fatalf("pledge %d %s", rec.Code, rec.Body.String())
	}
	registerCommerceIdentity(t, h, tc)

	// 入力の確認。
	bad := planBody()
	bad["kind"] = "single"
	bad["price_yen"] = 1999
	rec = doJSON(t, h, http.MethodPost, "/v1/coach/me/plans", tc, bad)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "duration_days") || !strings.Contains(rec.Body.String(), "price_yen") {
		t.Fatalf("bad %d %s", rec.Code, rec.Body.String())
	}

	// プランはコーチ一覧で誰でも見るので、名前と内容にも連絡先は書けない。
	contact := planBody()
	contact["title"] = "インスタ @run_coach で質問OK"
	contact["description"] = "詳しくは linktr.ee/run_coach を見てください"
	rec = doJSON(t, h, http.MethodPost, "/v1/coach/me/plans", tc, contact)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"title"`) || !strings.Contains(rec.Body.String(), `"description"`) {
		t.Fatalf("contact in plan %d %s", rec.Code, rec.Body.String())
	}

	// 30,000円を超える値段は、確かめてからでないと保存しない。
	high := planBody()
	high["price_yen"] = 300000
	rec = doJSON(t, h, http.MethodPost, "/v1/coach/me/plans", tc, high)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "30,000円") {
		t.Fatalf("high %d %s", rec.Code, rec.Body.String())
	}
	high["confirm_high_price"] = true
	rec = doJSON(t, h, http.MethodPost, "/v1/coach/me/plans", tc, high)
	if rec.Code != http.StatusCreated {
		t.Fatalf("high confirmed %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Plan struct {
			ID string `json:"id"`
		} `json:"plan"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	// 値段を変えずに直すときは、もう一度確かめなくてよい。
	edit := planBody()
	edit["price_yen"] = 300000
	edit["title"] = "1か月の個別メニュー"
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me/plans/"+created.Plan.ID, tc, edit); rec.Code != http.StatusOK {
		t.Fatalf("edit same price %d %s", rec.Code, rec.Body.String())
	}

	single := planBody()
	single["kind"] = "single"
	single["duration_days"] = 7
	single["price_yen"] = 3000
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/me/plans", tc, single); rec.Code != http.StatusCreated {
		t.Fatalf("single %d %s", rec.Code, rec.Body.String())
	}

	// コーチ一覧には公開中のプランが安い順に出る。
	var list struct {
		Coaches []struct {
			ID    string `json:"id"`
			Plans []struct {
				PriceYen int `json:"price_yen"`
			} `json:"plans"`
		} `json:"coaches"`
	}
	plansOf := func() []int {
		t.Helper()
		rec := doJSON(t, h, http.MethodGet, "/v1/coaches", "", nil)
		_ = json.Unmarshal(rec.Body.Bytes(), &list)
		for _, c := range list.Coaches {
			if c.ID == coach.String() {
				out := []int{}
				for _, p := range c.Plans {
					out = append(out, p.PriceYen)
				}
				return out
			}
		}
		t.Fatalf("coach not listed")
		return nil
	}
	if got := plansOf(); len(got) != 2 || got[0] != 3000 || got[1] != 300000 {
		t.Fatalf("public plans %v", got)
	}

	// 非公開にしたプランは出ない。
	edit["published"] = false
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me/plans/"+created.Plan.ID, tc, edit); rec.Code != http.StatusOK {
		t.Fatalf("unpublish %d", rec.Code)
	}
	if got := plansOf(); len(got) != 1 || got[0] != 3000 {
		t.Fatalf("after unpublish %v", got)
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/coach/me/plans", tc, nil)
	if rec.Code != http.StatusOK || strings.Count(rec.Body.String(), `"id"`) != 2 {
		t.Fatalf("my plans %d %s", rec.Code, rec.Body.String())
	}

	// 他のコーチのプランは直せない。
	other := uuid.New()
	to := token(t, other)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", to, coachApplicationBody()); rec.Code != http.StatusOK {
		t.Fatalf("apply other %d", rec.Code)
	}
	withCoachPhoto(t, other)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+other.String()+"/approve", adminToken(t, h), nil); rec.Code != http.StatusOK {
		t.Fatalf("approve other %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me/plans/"+created.Plan.ID, to, planBody()); rec.Code != http.StatusNotFound {
		t.Fatalf("other edit %d", rec.Code)
	}

	// 誓約の文面の版が古いと、一覧に出ない。
	execSQLArgs(t, `UPDATE coach_profiles SET pledge_version = 0 WHERE profile_id = $1`, coach)
	if got := plansOf(); len(got) != 0 {
		t.Fatalf("old pledge %v", got)
	}
}

// 同時に作っても、プランは10個まで。
func TestCoachingPlanLimitUnderConcurrency(t *testing.T) {
	h := setup(t)
	coach := uuid.New()
	tc := token(t, coach)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", tc, coachApplicationBody()); rec.Code != http.StatusOK {
		t.Fatalf("apply %d", rec.Code)
	}
	withCoachPhoto(t, coach)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+coach.String()+"/approve", adminToken(t, h), nil); rec.Code != http.StatusOK {
		t.Fatalf("approve %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/me/pledge", tc, map[string]any{"agree": true}); rec.Code != http.StatusOK {
		t.Fatalf("pledge %d", rec.Code)
	}
	registerCommerceIdentity(t, h, tc)
	var wg sync.WaitGroup
	for i := 0; i < 15; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			raw, _ := json.Marshal(planBody())
			req := httptest.NewRequest(http.MethodPost, "/v1/coach/me/plans", bytes.NewReader(raw))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+tc)
			req.RemoteAddr = fmt.Sprintf("203.0.113.%d:1234", i+1)
			h.ServeHTTP(httptest.NewRecorder(), req)
		}(i)
	}
	wg.Wait()
	var n int
	queryRowArgs(t, `SELECT count(*)::int FROM coaching_plans WHERE coach_id = $1`, []any{coach}, &n)
	if n != 10 {
		t.Fatalf("created %d plans, want 10", n)
	}
}
