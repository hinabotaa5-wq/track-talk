package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
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

// 日時を過ぎた練習には、解散していなくても申請も承認もできない。
func TestPracticeClosesApplicationsAfterStart(t *testing.T) {
	h := setup(t)
	host := token(t, uuid.New())
	rec := doJSON(t, h, http.MethodPost, "/v1/posts", host, map[string]any{
		"kind": "practice", "body": "ジョグから入ります", "venue": "駒沢競技場",
		"starts_at": time.Now().Add(48 * time.Hour).Format(time.RFC3339), "event": "100m", "capacity": 3,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/practice/applications", token(t, uuid.New()), nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
	var applied struct {
		Practice struct {
			MyApplicationID string `json:"my_application_id"`
		} `json:"practice"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &applied)

	execSQLArgs(t, `UPDATE practice_details SET starts_at = now() - interval '1 hour' WHERE post_id = $1`, created.ID)
	if rec := doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/practice/applications", token(t, uuid.New()), nil); rec.Code != http.StatusConflict {
		t.Fatalf("apply after start %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/practice/applications/"+applied.Practice.MyApplicationID+"/accept", host, nil); rec.Code != http.StatusConflict {
		t.Fatalf("accept after start %d %s", rec.Code, rec.Body.String())
	}
}

// practiceHeld は、練習の日時を過ぎたことにする（開かれた練習として解散・評価できるように）。
func practiceHeld(t *testing.T, postID string) {
	t.Helper()
	execSQLArgs(t, `UPDATE practice_details SET starts_at = now() - interval '1 hour' WHERE post_id = $1`, postID)
}

// 練習の日時より前に解散した練習は中止として扱い、評価もそのお願いの通知もない。
func TestPracticeDissolvedBeforeStartCannotBeRated(t *testing.T) {
	h := setup(t)
	host := token(t, uuid.New())
	guest := token(t, uuid.New())
	rec := doJSON(t, h, http.MethodPost, "/v1/posts", host, map[string]any{
		"kind": "practice", "body": "ジョグから入ります", "venue": "駒沢競技場",
		"starts_at": time.Now().Add(48 * time.Hour).Format(time.RFC3339), "event": "100m", "capacity": 3,
	})
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/practice/applications", guest, nil)
	var applied struct {
		Practice struct {
			MyApplicationID string `json:"my_application_id"`
		} `json:"practice"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &applied)
	app := applied.Practice.MyApplicationID
	if rec := doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/practice/applications/"+app+"/accept", host, nil); rec.Code != http.StatusOK {
		t.Fatalf("accept %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/practice/dissolve", host, nil)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), `"ratable"`) {
		t.Fatalf("dissolve %d %s", rec.Code, rec.Body.String())
	}
	if notes, _ := listNotifications(t, h, guest); len(notesFor(notes, created.ID, "practice_rate")) != 0 {
		t.Fatalf("rate notification for a canceled practice %+v", notes)
	}
	// あとで日時を過ぎても、中止した練習は評価できない。
	practiceHeld(t, created.ID)
	if rec := doJSON(t, h, http.MethodPost, "/v1/posts/"+created.ID+"/practice/applications/"+app+"/rating", host, map[string]any{"score": "no_show"}); rec.Code != http.StatusConflict {
		t.Fatalf("rate canceled practice %d %s", rec.Code, rec.Body.String())
	}
}
