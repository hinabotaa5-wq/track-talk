package httpx_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPrefecturePageGathersMeetsAndRecruitments(t *testing.T) {
	h := setup(t)
	pool := openPool(t)
	ctx := context.Background()
	tok := token(t, uuid.New())

	// 高知県(39)の大会を入れ直す: 終わった大会、開催中の大会、これからの大会。
	if _, err := pool.Exec(ctx, `DELETE FROM competitions WHERE prefecture IN ('39', '38')`); err != nil {
		t.Fatal(err)
	}
	today := time.Now().In(time.FixedZone("Asia/Tokyo", 9*60*60))
	day := func(offset int) string { return today.AddDate(0, 0, offset).Format(time.DateOnly) }
	insert := func(prefecture, name, starts string, ends *string) {
		t.Helper()
		if _, err := pool.Exec(ctx, `
			INSERT INTO competitions (id, name, starts_on, ends_on, location, prefecture, season_year, organizer_url, source_url)
			VALUES ($1, $2, $3::date, $4::date, '春野', $5, 2026, 'https://example.com', 'https://example.com')`,
			uuid.New(), name, starts, ends, prefecture); err != nil {
			t.Fatal(err)
		}
	}
	tomorrow := day(1)
	insert("39", "終わった記録会", day(-10), nil)
	insert("39", "開催中の選手権", day(-1), &tomorrow)
	insert("39", "これからの記録会", day(7), nil)
	insert("38", "隣の県の大会", day(3), nil)

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
		t.Fatalf("delete %d", rec.Code)
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
		Competitions []struct {
			Name string `json:"name"`
		} `json:"competitions"`
		CompetitionTotal int `json:"competition_total"`
		Recruitments     []struct {
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
	// 終わった大会と隣の県の大会は出ない。開催中の大会は出る。日付の近い順。
	if got.CompetitionTotal != 2 || len(got.Competitions) != 2 || got.Competitions[0].Name != "開催中の選手権" || got.Competitions[1].Name != "これからの記録会" {
		t.Fatalf("competitions %s", rec.Body.String())
	}
	// 消した募集と他県だけの募集は出ない。新しい順。
	if got.RecruitmentTotal != 2 || len(got.Recruitments) != 2 || got.Recruitments[0].Title != "四国のクラブ" || got.Recruitments[1].Title != "高知のクラブ" || got.Recruitments[0].Schedule != "毎週土曜" {
		t.Fatalf("recruitments %s", rec.Body.String())
	}

	// 大会も募集もない県は空の一覧。
	empty := doJSON(t, h, http.MethodGet, "/v1/prefectures/47", tok, nil)
	if empty.Code != http.StatusOK {
		t.Fatalf("empty %d", empty.Code)
	}
	var none struct {
		Competitions []any `json:"competitions"`
		Recruitments []any `json:"recruitments"`
	}
	_ = json.Unmarshal(empty.Body.Bytes(), &none)
	if none.Recruitments == nil || len(none.Recruitments) != 0 {
		t.Fatalf("empty page %s", empty.Body.String())
	}
}
