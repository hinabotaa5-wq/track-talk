package httpx_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// 県ページはクラブチームの募集だけを集める。県別の大会の一覧は #549 で廃止した。
func TestPrefecturePageGathersRecruitments(t *testing.T) {
	h := setup(t)
	tok := token(t, uuid.New())

	recruit := func(prefectures []string, title string) string {
		t.Helper()
		rec := doJSON(t, h, http.MethodPost, "/v1/posts", tok, map[string]any{
			"title": title, "body": "初心者も歓迎です", "kind": "recruitment",
			"prefectures": prefectures, "schedule": "毎週土曜",
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("recruitment %d %s", rec.Code, rec.Body.String())
		}
		var p struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &p)
		return p.ID
	}
	recruit([]string{"39"}, "高知のクラブ")
	recruit([]string{"38", "39"}, "四国のクラブ")
	recruit([]string{"13"}, "東京のクラブ")
	removed := recruit([]string{"39"}, "消した募集")
	if rec := doJSON(t, h, http.MethodDelete, "/v1/posts/"+removed, tok, nil); rec.Code != http.StatusNoContent && rec.Code != http.StatusOK {
		t.Fatalf("delete %d %s", rec.Code, rec.Body.String())
	}

	if rec := doJSON(t, h, http.MethodGet, "/v1/prefectures/39", "", nil); rec.Code != http.StatusOK {
		t.Fatalf("anonymous = %d", rec.Code)
	}
	for _, bad := range []string{"00", "48", "tokyo"} {
		if rec := doJSON(t, h, http.MethodGet, "/v1/prefectures/"+bad, tok, nil); rec.Code != http.StatusNotFound {
			t.Fatalf("prefecture %s = %d", bad, rec.Code)
		}
	}

	rec := doJSON(t, h, http.MethodGet, "/v1/prefectures/39", tok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("page %d %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Prefecture struct {
			Code string `json:"code"`
			Name string `json:"name"`
		} `json:"prefecture"`
		Recruitments []struct {
			Title    string `json:"title"`
			Schedule string `json:"schedule"`
		} `json:"recruitments"`
		RecruitmentTotal int `json:"recruitment_total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Prefecture.Code != "39" || got.Prefecture.Name != "高知県" {
		t.Fatalf("prefecture %+v", got.Prefecture)
	}
	// 消した募集と他県だけの募集は出ない。新しい順。
	if got.RecruitmentTotal != 2 || len(got.Recruitments) != 2 || got.Recruitments[0].Title != "四国のクラブ" || got.Recruitments[1].Title != "高知のクラブ" || got.Recruitments[0].Schedule != "毎週土曜" {
		t.Fatalf("recruitments %s", rec.Body.String())
	}

	// 募集のない県は空の一覧。
	empty := doJSON(t, h, http.MethodGet, "/v1/prefectures/47", tok, nil)
	if empty.Code != http.StatusOK {
		t.Fatalf("empty %d", empty.Code)
	}
	var none struct {
		Recruitments []any `json:"recruitments"`
	}
	_ = json.Unmarshal(empty.Body.Bytes(), &none)
	if none.Recruitments == nil || len(none.Recruitments) != 0 {
		t.Fatalf("empty page %s", empty.Body.String())
	}
}
