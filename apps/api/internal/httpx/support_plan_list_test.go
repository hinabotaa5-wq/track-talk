package httpx_test

import (
	"encoding/json"
	"strings"
	"testing"
)

// サポートプランの入口の一覧：無料相談を受け付けているコーチ（1ヶ月プラン）だけを出し、
// ★・件数などの人気の指標は出さない。受け入れ状況はコーチの手動入力だけ。並びは公平なローテーション。
// 対面プランのコーチは表示しない（コード・データは残し、無効のまま）。
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
	// 無料相談のコーチだけ。対面だけのコーチは出ない。
	if len(out.Coaches) != 1 {
		t.Fatalf("want 1 support coach, got %s", body)
	}
	c := out.Coaches[0]
	if c.ID != consultID.String() || c.Competition == nil || c.Competition.Reception.Status != "open" || len(c.Kinds) != 1 || c.Kinds[0] != "competition_support" || c.InPerson != nil {
		t.Fatalf("competition coach %s", body)
	}
	// カレンダーに空きの日を入れても、対面は出ないまま。
	if r := doJSON(t, h, "PUT", "/v1/coach/me/availability", inPersonCoach, availabilityBody(day(2), "available", "")); r.Code != 200 {
		t.Fatalf("availability %d %s", r.Code, r.Body.String())
	}
	r = doJSON(t, h, "GET", "/v1/coaches/support-plan", "", nil)
	if strings.Contains(r.Body.String(), `"in_person":{`) {
		t.Fatalf("in-person leaked %s", r.Body.String())
	}
}
