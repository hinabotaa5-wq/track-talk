package httpx_test

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// 最初の動画から数えるプランは、始まった時に終了日を7日先に延ばしておき、
// 利用者の最初の動画が届いたら、その時から期間を数え直す。
func TestCoachingClockStartsOnFirstVideo(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setup(t)
	_, tc, planID := planCoach(t, h, 3)
	body := planBody()
	body["start_on_first_video"] = true
	rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me/plans/"+planID, tc, body)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"start_on_first_video":true`) {
		t.Fatalf("plan %d %s", rec.Code, rec.Body.String())
	}

	_, tu := demoUser(t, h, time.Date(1995, 4, 1, 0, 0, 0, 0, time.UTC))
	rec = doJSON(t, h, http.MethodPost, "/v1/coaching/contracts", tu, contractBody(planID))
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"start_on_first_video":true`) {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
	id := decodeContract(t, rec.Body.Bytes()).ID
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/contracts/"+id+"/accept", tc, map[string]any{"agree_report": true, "agree_detection": true}); rec.Code != http.StatusOK {
		t.Fatalf("accept %d %s", rec.Code, rec.Body.String())
	}
	var days float64
	lengthDays := func() float64 {
		t.Helper()
		queryRowArgs(t, `SELECT extract(epoch FROM ends_at - starts_at) / 86400 FROM coaching_contracts WHERE id = $1`, []any{id}, &days)
		return days
	}
	// 30日のプランに、待つ7日を足しておく。
	if d := lengthDays(); d < 36.99 || d > 37.01 {
		t.Fatalf("provisional length %v", d)
	}

	// 文章やコーチのメッセージでは数え始めない。
	path := "/v1/coaching/contracts/" + id + "/messages"
	if rec := doJSON(t, h, http.MethodPost, path, tu, map[string]any{"body": "よろしくお願いします。"}); rec.Code != http.StatusCreated {
		t.Fatalf("text %d", rec.Code)
	}
	if d := lengthDays(); d < 36.99 {
		t.Fatalf("text started the clock %v", d)
	}

	// 3日たってから最初の動画が届いたら、その時から30日にする。
	execSQLArgs(t, `UPDATE coaching_contracts SET starts_at = starts_at - interval '3 days', ends_at = ends_at - interval '3 days' WHERE id = $1`, id)
	if rec := doJSON(t, h, http.MethodPost, path, tu, map[string]any{"video_public_id": signedCoachingVideo(t, h, tu)}); rec.Code != http.StatusCreated {
		t.Fatalf("video %d %s", rec.Code, rec.Body.String())
	}
	if d := lengthDays(); d < 32.99 || d > 33.01 {
		t.Fatalf("after first video %v", d)
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/me/coaching/contracts", tu, nil)
	if strings.Contains(rec.Body.String(), `"clock_started_at":null`) {
		t.Fatalf("clock not reported %s", rec.Body.String())
	}

	// 2本目では数え直さない。
	execSQLArgs(t, `UPDATE coaching_contracts SET starts_at = starts_at - interval '1 day', ends_at = ends_at - interval '1 day', clock_started_at = clock_started_at - interval '1 day' WHERE id = $1`, id)
	if rec := doJSON(t, h, http.MethodPost, path, tu, map[string]any{"video_public_id": signedCoachingVideo(t, h, tu)}); rec.Code != http.StatusCreated {
		t.Fatalf("second video %d %s", rec.Code, rec.Body.String())
	}
	if d := lengthDays(); d < 32.99 || d > 33.01 {
		t.Fatalf("second video moved the clock %v", d)
	}
}

// 7日のあいだに動画が届かなければ、8日目から数え始めたのと同じで、そのあとの動画では数え直さない。
func TestCoachingClockDoesNotRestartAfterWaiting(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setup(t)
	_, tc, planID := planCoach(t, h, 3)
	body := planBody()
	body["start_on_first_video"] = true
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me/plans/"+planID, tc, body); rec.Code != http.StatusOK {
		t.Fatalf("plan %d", rec.Code)
	}
	_, tu := demoUser(t, h, time.Date(1995, 4, 1, 0, 0, 0, 0, time.UTC))
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts", tu, contractBody(planID))
	id := decodeContract(t, rec.Body.Bytes()).ID
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/contracts/"+id+"/accept", tc, map[string]any{"agree_report": true, "agree_detection": true}); rec.Code != http.StatusOK {
		t.Fatalf("accept %d", rec.Code)
	}
	execSQLArgs(t, `UPDATE coaching_contracts SET starts_at = starts_at - interval '8 days', ends_at = ends_at - interval '8 days' WHERE id = $1`, id)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts/"+id+"/messages", tu, map[string]any{"video_public_id": signedCoachingVideo(t, h, tu)}); rec.Code != http.StatusCreated {
		t.Fatalf("video %d %s", rec.Code, rec.Body.String())
	}
	var days float64
	var started *time.Time
	queryRowArgs(t, `SELECT extract(epoch FROM ends_at - starts_at) / 86400, clock_started_at FROM coaching_contracts WHERE id = $1`, []any{id}, &days, &started)
	if days < 36.99 || days > 37.01 || started != nil {
		t.Fatalf("late video %v %v", days, started)
	}
}

// ふつうのプランは、これまでどおり始まった時から数える。
func TestCoachingClockIsUnchangedForOrdinaryPlans(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setup(t)
	_, tu, id := activeContract(t, h)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts/"+id+"/messages", tu, map[string]any{"video_public_id": signedCoachingVideo(t, h, tu)}); rec.Code != http.StatusCreated {
		t.Fatalf("video %d %s", rec.Code, rec.Body.String())
	}
	var days float64
	queryRowArgs(t, `SELECT extract(epoch FROM ends_at - starts_at) / 86400 FROM coaching_contracts WHERE id = $1`, []any{id}, &days)
	if days < 29.99 || days > 30.01 {
		t.Fatalf("ordinary length %v", days)
	}
}
