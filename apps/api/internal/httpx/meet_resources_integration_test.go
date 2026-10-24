package httpx_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

func TestMeetResourcesAdminSavePublicReadAndClear(t *testing.T) {
	fixed := time.Date(2026, 10, 24, 12, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	h := setupConfig(t, func(cfg *config.Config) { cfg.Clock = func() time.Time { return fixed } })
	pool := openPool(t)
	ctx := context.Background()
	seedSystemProfile(t, pool)
	const key = "m:resources-check:2026"
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM meet_resources WHERE meet_key=$1`, key)
		pool.Exec(ctx, `DELETE FROM domestic_majors WHERE slug='resources-check'`)
	})
	_, err := pool.Exec(ctx, `INSERT INTO domestic_majors (slug,year,name,category,venue,starts_on,ends_on,schedule_label,site_url) VALUES ('resources-check',2026,'リンク確認大会','open','競技場','2026-10-24','2026-10-24','2026年','https://example.jp/original')`)
	if err != nil {
		t.Fatal(err)
	}
	admin, user := uuid.New(), uuid.New()
	if _, err = pool.Exec(ctx, `INSERT INTO profiles(id,is_admin) VALUES($1,true),($2,false)`, admin, user); err != nil {
		t.Fatal(err)
	}
	ta, tu := token(t, admin), token(t, user)
	payload := map[string]string{"meet_key": key, "official_url": "https://example.jp/official", "timetable_url": "https://example.jp/timetable.pdf", "youtube_url": "https://www.youtube.com/live/abcdefghijk"}
	put := func(tok string) int {
		return doJSON(t, h, http.MethodPut, "/v1/admin/meet-resources", tok, payload).Code
	}
	if got := put(""); got != 401 {
		t.Fatalf("anonymous %d", got)
	}
	if got := put(tu); got != 403 {
		t.Fatalf("user %d", got)
	}
	payload["official_url"] = "javascript:alert(1)"
	if got := put(ta); got != 422 {
		t.Fatalf("invalid URL %d", got)
	}
	payload["official_url"] = "https://example.jp/official"
	payload["youtube_url"] = "https://example.jp/live"
	if got := put(ta); got != 422 {
		t.Fatalf("invalid stream %d", got)
	}
	payload["youtube_url"] = "https://www.youtube.com/live/abcdefghijk"
	payload["meet_key"] = "unknown"
	if got := put(ta); got != 422 {
		t.Fatalf("unknown meet %d", got)
	}
	payload["meet_key"] = key
	if got := put(ta); got != 200 {
		t.Fatalf("save %d", got)
	}
	for _, path := range []string{"/v1/meet-resources", "/v1/admin/meet-sessions"} {
		rec := doJSON(t, h, http.MethodGet, path, ta, nil)
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), "https://example.jp/timetable.pdf") || !strings.Contains(rec.Body.String(), "abcdefghijk") {
			t.Fatalf("list %s: %d %s", path, rec.Code, rec.Body.String())
		}
	}
	rec := doJSON(t, h, http.MethodGet, "/v1/meet-threads/today", "", nil)
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var today struct {
		Meets []struct {
			MeetKey string `json:"meet_key"`
			PostID  string `json:"post_id"`
		} `json:"meets"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &today); err != nil {
		t.Fatal(err)
	}
	id := ""
	for _, m := range today.Meets {
		if m.MeetKey == key {
			id = m.PostID
		}
	}
	if id == "" {
		t.Fatal("missing room")
	}
	path := "/v1/posts/" + id + "/meet-resources"
	rec = doJSON(t, h, http.MethodGet, path, "", nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "abcdefghijk") {
		t.Fatalf("public detail %d %s", rec.Code, rec.Body.String())
	}
	payload["official_url"] = ""
	payload["timetable_url"] = ""
	payload["youtube_url"] = ""
	if got := put(ta); got != 200 {
		t.Fatalf("clear %d", got)
	}
	rec = doJSON(t, h, http.MethodGet, path, "", nil)
	if !strings.Contains(rec.Body.String(), `"timetable_url":""`) || !strings.Contains(rec.Body.String(), "https://example.jp/original") {
		t.Fatal(rec.Body.String())
	}
	fixed = fixed.AddDate(0, 0, 1)
	if got := doJSON(t, h, http.MethodGet, path, "", nil).Code; got != 200 {
		t.Fatalf("past room %d", got)
	}
	if _, err = pool.Exec(ctx, `UPDATE posts SET hidden_at=now() WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if got := doJSON(t, h, http.MethodGet, path, "", nil).Code; got != 404 {
		t.Fatalf("hidden room %d", got)
	}
}
