package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type filteredReviews struct {
	Reviews []struct {
		ID            string  `json:"id"`
		Rating        int     `json:"rating"`
		Event         *string `json:"event"`
		EventLabel    *string `json:"event_label"`
		LevelLabel    *string `json:"level_label"`
		FootSizeLabel *string `json:"foot_size_label"`
		FootWidth     *string `json:"foot_width"`
		UsageLabel    *string `json:"usage_label"`
	} `json:"reviews"`
	Summary struct {
		ReviewCount int     `json:"review_count"`
		AvgRating   float64 `json:"avg_rating"`
		Filtered    bool    `json:"filtered"`
	} `json:"summary"`
}

type filteredProducts struct {
	Products []struct {
		ID          string  `json:"id"`
		ReviewCount int     `json:"review_count"`
		AvgRating   float64 `json:"avg_rating"`
	} `json:"products"`
}

func TestReviewsFilterByReviewerAttributes(t *testing.T) {
	h := setup(t)
	adminID := uuid.New()
	admin := token(t, adminID)
	if rec := doJSON(t, h, http.MethodGet, "/v1/me", admin, nil); rec.Code != http.StatusOK {
		t.Fatalf("admin profile %d", rec.Code)
	}
	setAdmin(t, adminID)

	brand := "Filter" + strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	newProduct := func(name string) string {
		t.Helper()
		rec := doJSON(t, h, http.MethodPost, "/v1/admin/products", admin, map[string]string{"kind": "spike", "brand": brand, "name": name})
		if rec.Code != http.StatusCreated {
			t.Fatalf("create product %d %s", rec.Code, rec.Body.String())
		}
		var p struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &p)
		return p.ID
	}
	sprint := newProduct("短距離向け")
	distance := newProduct("長距離向け")

	review := func(productID string, body map[string]any) {
		t.Helper()
		rec := doJSON(t, h, http.MethodPost, "/v1/products/"+productID+"/reviews", token(t, uuid.New()), body)
		if rec.Code != http.StatusCreated {
			t.Fatalf("review %d %s", rec.Code, rec.Body.String())
		}
	}
	// 属性を書かない以前のようなレビュー。
	review(sprint, map[string]any{"rating": 2, "body": "ふつう"})
	review(sprint, map[string]any{"rating": 5, "body": "100mで使った", "event": "100m", "level": "high-school", "foot_size": 25.5, "foot_width": "wide", "usage": "race"})
	review(sprint, map[string]any{"rating": 3, "body": "練習で", "event": "100m", "level": "university", "foot_size": 27.0, "foot_width": "regular", "usage": "practice"})
	review(distance, map[string]any{"rating": 4, "body": "5000mで", "event": "5000m", "level": "high-school", "foot_size": 25.0, "usage": "race"})

	// 入力の検証。
	for _, bad := range []map[string]any{
		{"rating": 4, "body": "x", "level": "pro"},
		{"rating": 4, "body": "x", "foot_size": 25.3},
		{"rating": 4, "body": "x", "foot_size": 31},
		{"rating": 4, "body": "x", "foot_width": "huge"},
		{"rating": 4, "body": "x", "usage": "daily"},
		{"rating": 4, "body": "x", "event": "marathon"},
	} {
		if rec := doJSON(t, h, http.MethodPost, "/v1/products/"+sprint+"/reviews", token(t, uuid.New()), bad); rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("invalid %v = %d", bad, rec.Code)
		}
	}

	listReviews := func(query string) filteredReviews {
		t.Helper()
		rec := doJSON(t, h, http.MethodGet, "/v1/products/"+sprint+"/reviews"+query, "", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("reviews %s = %d %s", query, rec.Code, rec.Body.String())
		}
		var out filteredReviews
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return out
	}

	all := listReviews("")
	if len(all.Reviews) != 3 || all.Summary.ReviewCount != 3 || all.Summary.Filtered {
		t.Fatalf("all reviews %+v", all)
	}
	race := listReviews("?usage=race")
	if len(race.Reviews) != 1 || race.Summary.ReviewCount != 1 || race.Summary.AvgRating != 5 || !race.Summary.Filtered {
		t.Fatalf("race reviews %+v", race)
	}
	r := race.Reviews[0]
	// 足のサイズは正確な値ではなく 1cm 幅で返す。
	if r.EventLabel == nil || *r.EventLabel != "100m" || r.LevelLabel == nil || *r.LevelLabel != "高校" ||
		r.FootSizeLabel == nil || *r.FootSizeLabel != "25.0〜25.5cm" || r.UsageLabel == nil || *r.UsageLabel != "試合用" {
		t.Fatalf("race review attributes %+v", r)
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/products/"+sprint+"/reviews?usage=race", "", nil); strings.Contains(rec.Body.String(), `"foot_size":`) {
		t.Fatalf("exact foot size leaked: %s", rec.Body.String())
	}
	sized := listReviews("?foot_size_min=26.5&foot_size_max=27.5")
	if len(sized.Reviews) != 1 || sized.Summary.AvgRating != 3 {
		t.Fatalf("sized reviews %+v", sized)
	}
	if got := listReviews("?event=100m&level=university&foot_width=regular"); len(got.Reviews) != 1 {
		t.Fatalf("combined filter %+v", got)
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/products/"+sprint+"/reviews?level=pro", "", nil); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad filter = %d", rec.Code)
	}

	listProducts := func(query string) filteredProducts {
		t.Helper()
		rec := doJSON(t, h, http.MethodGet, "/v1/products?kind=spike&q="+brand+query, "", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("products %s = %d %s", query, rec.Code, rec.Body.String())
		}
		var out filteredProducts
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return out
	}
	// 絞り込みなしは、これまでどおり全商品と全レビューの件数。
	if got := listProducts(""); len(got.Products) != 2 || got.Products[0].ID != sprint || got.Products[0].ReviewCount != 3 {
		t.Fatalf("unfiltered products %+v", got)
	}
	// 高校生のレビューだけで数え直す。どちらも1件ずつで、平均★の高い短距離向けが先。
	hs := listProducts("&level=high-school")
	if len(hs.Products) != 2 || hs.Products[0].ID != sprint || hs.Products[0].ReviewCount != 1 || hs.Products[0].AvgRating != 5 || hs.Products[1].AvgRating != 4 {
		t.Fatalf("high school products %+v", hs)
	}
	// 合うレビューのない商品は出さない。
	if got := listProducts("&event=5000m"); len(got.Products) != 1 || got.Products[0].ID != distance {
		t.Fatalf("5000m products %+v", got)
	}
}
