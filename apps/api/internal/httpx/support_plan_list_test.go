package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// サポートプランの入口の一覧：大会まで（無料相談の受付）と対面（公開プラン）のコーチだけを出し、
// ★・件数などの人気の指標は出さない。大会までは手動の受付状況、対面はカレンダーの空きで受付状況が決まる。
func TestSupportPlanCoachList(t *testing.T) {
	h := setup(t)
	consultID, _ := consultCoach(t, h)
	inPersonCoach, _ := inPersonPlan(t, h)
	_, plain, _ := planCoach(t, h, 5) // 動画添削だけのコーチ（サポートプランの一覧に出ない）
	_ = plain

	r := doJSON(t, h, "GET", "/v1/coaches/support-plan", "", nil)
	if r.Code != 200 {
		t.Fatalf("list %d %s", r.Code, r.Body.String())
	}
	body := r.Body.String()
	for _, banned := range []string{"rating", "count", "capacity", "remaining", "full", "sales"} {
		if strings.Contains(body, `"`+banned) {
			t.Fatalf("popularity field %q leaked: %s", banned, body)
		}
	}
	var out struct {
		Coaches []struct {
			ID       string   `json:"id"`
			Kinds    []string `json:"kinds"`
			InPerson *struct {
				Areas       []string                `json:"areas"`
				MinPriceYen int                     `json:"min_price_yen"`
				Reception   struct{ Status string } `json:"reception"`
			} `json:"in_person"`
			Competition *struct {
				Reception struct{ Status string } `json:"reception"`
			} `json:"competition_support"`
		} `json:"coaches"`
	}
	if err := json.Unmarshal(r.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Coaches) != 2 {
		t.Fatalf("want 2 support coaches, got %s", body)
	}
	for _, c := range out.Coaches {
		switch {
		case c.ID == consultID.String():
			if c.Competition == nil || c.Competition.Reception.Status != "open" || len(c.Kinds) != 1 || c.Kinds[0] != "competition_support" {
				t.Fatalf("competition coach %s", body)
			}
		case c.InPerson != nil:
			// カレンダーに空きの日がまだ無いので受付停止中。場所と価格は出る。
			if c.InPerson.Reception.Status != "paused" || c.InPerson.MinPriceYen != 8000 || len(c.InPerson.Areas) != 1 || c.InPerson.Areas[0] != "東京都" {
				t.Fatalf("in-person coach %s", body)
			}
		default:
			t.Fatalf("unexpected coach %s", body)
		}
	}
	// カレンダーに空きの日を入れると、対面は自動で受付中になる。
	if r := doJSON(t, h, "PUT", "/v1/coach/me/availability", inPersonCoach, availabilityBody(day(2), "available", "")); r.Code != 200 {
		t.Fatalf("availability %d %s", r.Code, r.Body.String())
	}
	r = doJSON(t, h, "GET", "/v1/coaches/support-plan", "", nil)
	if !strings.Contains(r.Body.String(), `"in_person":{`) || !strings.Contains(r.Body.String(), `"reception":{"status":"open"}`) {
		t.Fatalf("in-person open %s", r.Body.String())
	}
	_ = http.MethodGet
}
