package httpx_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPracticeUpdateRejectsPastStart(t *testing.T) {
	h := setup(t)
	host := token(t, uuid.New())
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	starts := time.Now().In(loc).Add(48 * time.Hour).Truncate(time.Minute)
	payload := map[string]any{
		"kind": "practice", "body": "ジョグから入ります", "venue": "駒沢競技場",
		"starts_at": starts.Format(time.RFC3339), "event": "100m", "capacity": 3,
	}
	rec := doJSON(t, h, http.MethodPost, "/v1/posts", host, payload)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID       string `json:"id"`
		Practice struct {
			StartsAt string `json:"starts_at"`
		} `json:"practice"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	past := time.Now().Add(-time.Hour).Format(time.RFC3339)
	payload["starts_at"] = past
	rec = doJSON(t, h, http.MethodPatch, "/v1/posts/"+created.ID, host, payload)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("past update %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/posts/"+created.ID, host, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get %d %s", rec.Code, rec.Body.String())
	}
	var kept struct {
		Practice struct {
			StartsAt string `json:"starts_at"`
		} `json:"practice"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &kept)
	if kept.Practice.StartsAt != created.Practice.StartsAt {
		t.Fatalf("start changed %s -> %s", created.Practice.StartsAt, kept.Practice.StartsAt)
	}

	payload["starts_at"] = starts.Add(24 * time.Hour).Format(time.RFC3339)
	rec = doJSON(t, h, http.MethodPatch, "/v1/posts/"+created.ID, host, payload)
	if rec.Code != http.StatusOK {
		t.Fatalf("future update %d %s", rec.Code, rec.Body.String())
	}

	if rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/practice/dissolve", host, nil); rec.Code != http.StatusOK {
		t.Fatalf("dissolve %d %s", rec.Code, rec.Body.String())
	}
	payload["starts_at"] = past
	payload["venue"] = "解散後の場所"
	rec = doJSON(t, h, http.MethodPatch, "/v1/posts/"+created.ID, host, payload)
	if rec.Code != http.StatusOK {
		t.Fatalf("dissolved past update %d %s", rec.Code, rec.Body.String())
	}
}
