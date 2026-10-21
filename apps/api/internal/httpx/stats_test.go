package httpx_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

func TestAdminStatsCountsWithoutNames(t *testing.T) {
	fixed := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	h := setupConfig(t, func(cfg *config.Config) {
		cfg.Clock = func() time.Time { return fixed }
	})
	adminID := uuid.New()
	admin := token(t, adminID)
	if rec := doJSON(t, h, http.MethodGet, "/v1/me", admin, nil); rec.Code != http.StatusOK {
		t.Fatalf("admin profile %d", rec.Code)
	}
	member := token(t, uuid.New())
	if rec := doJSON(t, h, http.MethodGet, "/v1/admin/stats", member, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("member stats = %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/admin/stats/coaching", member, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("member coaching stats = %d", rec.Code)
	}
	setAdmin(t, adminID)

	if rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes", member, map[string]any{"body": "接地が軽かった", "event": "100m"}); rec.Code != http.StatusCreated {
		t.Fatalf("sense note %d %s", rec.Code, rec.Body.String())
	}

	rec := doJSON(t, h, http.MethodGet, "/v1/admin/stats", admin, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("stats %d %s", rec.Code, rec.Body.String())
	}
	var stats struct {
		Weeks []struct {
			WeekStart   string `json:"week_start"`
			Signups     int    `json:"signups"`
			ActiveUsers int    `json:"active_users"`
			SenseNotes  int    `json:"sense_notes"`
		} `json:"weeks"`
		Features []struct {
			Feature     string `json:"feature"`
			Recent      int    `json:"recent"`
			RecentUsers int    `json:"recent_users"`
		} `json:"features"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatal(err)
	}
	if len(stats.Weeks) != 12 {
		t.Fatalf("weeks %d", len(stats.Weeks))
	}
	// 週は日本時間の月曜始まりで、新しい週が先頭。
	if stats.Weeks[0].WeekStart != "2026-06-22" || stats.Weeks[11].WeekStart != "2026-04-06" {
		t.Fatalf("week range %s..%s", stats.Weeks[11].WeekStart, stats.Weeks[0].WeekStart)
	}
	// 本文やアカウントは返さない。
	if strings.Contains(rec.Body.String(), "接地") || strings.Contains(rec.Body.String(), adminID.String()) {
		t.Fatalf("stats leaked content %s", rec.Body.String())
	}
	found := false
	for _, f := range stats.Features {
		if f.Feature == "sense_note" {
			found = f.Recent >= 1 && f.RecentUsers >= 1
		}
	}
	if !found {
		t.Fatalf("features %s", rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/admin/stats/coaching", admin, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("coaching stats %d %s", rec.Code, rec.Body.String())
	}
}

func TestPBStatsNeedFivePeople(t *testing.T) {
	fixed := time.Date(2026, 6, 30, 3, 0, 0, 0, time.UTC)
	h := setupConfig(t, func(cfg *config.Config) {
		cfg.Clock = func() time.Time { return fixed }
	})
	if rec := doJSON(t, h, http.MethodGet, "/v1/race-records/pb-stats", "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous = %d", rec.Code)
	}
	add := func(tok, day, event string, mark int, wind *int) {
		t.Helper()
		body := map[string]any{"raced_on": day, "event": event, "mark": mark}
		if wind != nil {
			body["wind"] = *wind
		}
		if rec := doJSON(t, h, http.MethodPost, "/v1/me/race-records", tok, body); rec.Code != http.StatusCreated {
			t.Fatalf("record %v = %d %s", body, rec.Code, rec.Body.String())
		}
	}
	aided := 25
	// 100m は5人が更新。走幅跳は4人だけなので出さない。
	for i := 0; i < 5; i++ {
		tok := token(t, uuid.New())
		add(tok, "2026-04-01", "100m", 1200, nil)
		add(tok, "2026-06-20", "100m", 1180-i, nil)
	}
	for i := 0; i < 4; i++ {
		tok := token(t, uuid.New())
		add(tok, "2026-04-01", "long-jump", 600, nil)
		add(tok, "2026-06-20", "long-jump", 620+i, nil)
	}
	// 数えない記録: 最初の記録、追い風参考、前の記録より悪い、30日より前の更新。
	notCounted := token(t, uuid.New())
	add(notCounted, "2026-06-01", "200m", 2400, nil)
	add(notCounted, "2026-04-01", "100m", 1150, nil)
	add(notCounted, "2026-06-10", "100m", 1100, &aided)
	add(notCounted, "2026-06-15", "100m", 1160, nil)
	add(notCounted, "2026-05-01", "100m", 1140, nil)

	me := token(t, uuid.New())
	rec := doJSON(t, h, http.MethodGet, "/v1/race-records/pb-stats", me, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("pb stats %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Days   int `json:"days"`
		Events []struct {
			Event      string `json:"event"`
			EventLabel string `json:"event_label"`
			Authors    int    `json:"authors"`
		} `json:"events"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Days != 30 || len(out.Events) != 1 || out.Events[0].Event != "100m" || out.Events[0].Authors != 5 || out.Events[0].EventLabel != "100m" {
		t.Fatalf("pb stats %s", rec.Body.String())
	}
}

func TestProductRankingByEvent(t *testing.T) {
	h := setup(t)
	adminID := uuid.New()
	admin := token(t, adminID)
	if rec := doJSON(t, h, http.MethodGet, "/v1/me", admin, nil); rec.Code != http.StatusOK {
		t.Fatalf("admin profile %d", rec.Code)
	}
	setAdmin(t, adminID)
	brand := "Rank" + strings.ReplaceAll(uuid.NewString()[:8], "-", "")
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
	review := func(productID, event string, rating int) {
		t.Helper()
		body := map[string]any{"rating": rating, "body": fmt.Sprintf("%sで%d", event, rating), "event": event}
		if rec := doJSON(t, h, http.MethodPost, "/v1/products/"+productID+"/reviews", token(t, uuid.New()), body); rec.Code != http.StatusCreated {
			t.Fatalf("review %d %s", rec.Code, rec.Body.String())
		}
	}
	high := newProduct("高評価")
	mid := newProduct("中評価")
	few := newProduct("レビュー少")
	for _, rating := range []int{5, 5, 4} {
		review(high, "100m", rating)
	}
	for _, rating := range []int{4, 3, 3, 3} {
		review(mid, "100m", rating)
	}
	// 他の種目の★は混ぜない。
	review(mid, "5000m", 5)
	review(mid, "5000m", 5)
	review(few, "100m", 5)
	review(few, "100m", 5)

	rec := doJSON(t, h, http.MethodGet, "/v1/products/ranking?kind=spike&event=100m", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("ranking %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		EventLabel string `json:"event_label"`
		Products   []struct {
			ID          string  `json:"id"`
			ReviewCount int     `json:"review_count"`
			AvgRating   float64 `json:"avg_rating"`
		} `json:"products"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, p := range out.Products {
		if p.ID == high || p.ID == mid || p.ID == few {
			ids = append(ids, p.ID)
		}
	}
	if len(ids) != 2 || ids[0] != high || ids[1] != mid {
		t.Fatalf("ranking %s", rec.Body.String())
	}
	if out.Products[1].ReviewCount != 4 || out.Products[1].AvgRating != 3.25 {
		t.Fatalf("mid only counts 100m reviews: %s", rec.Body.String())
	}
	for _, bad := range []string{"?kind=spike", "?kind=spike&event=marathon", "?kind=boots&event=100m"} {
		if rec := doJSON(t, h, http.MethodGet, "/v1/products/ranking"+bad, "", nil); rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("ranking %s = %d", bad, rec.Code)
		}
	}
}
