package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

type coachRequestsView struct {
	Offers []struct {
		ID      string            `json:"id"`
		Request reviewRequestView `json:"request"`
	} `json:"offers"`
	Reviewing []reviewRequestView `json:"reviewing"`
}

// approvedReviewCoach は、動画添削に必要な承認と振込先を設定する。
func approvedReviewCoach(t *testing.T, h http.Handler, tadmin string) (uuid.UUID, string) {
	t.Helper()
	id := uuid.New()
	tc := token(t, id)
	body := coachApplicationBody()
	body["tier"] = "coach"
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", tc, body); rec.Code != http.StatusOK {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
	withCoachPhoto(t, id)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+id.String()+"/approve", tadmin, nil); rec.Code != http.StatusOK {
		t.Fatalf("approve %d", rec.Code)
	}
	// 支払いのあるテストでも受付を始められるように、受け取り口座の登録を済ませておく。
	execSQLArgs(t, `UPDATE coach_profiles SET stripe_account_id = $2, payouts_enabled = true WHERE profile_id = $1`, id, "acct_"+id.String())
	return id, tc
}

// onDutyCoach は、承認済みで受付中のコーチ（枠はコーチ、種目は110mハードルを含む）を作る。
func onDutyCoach(t *testing.T, h http.Handler, tadmin string) (uuid.UUID, string) {
	t.Helper()
	id, tc := approvedReviewCoach(t, h, tadmin)
	execSQLArgs(t, `INSERT INTO push_subscriptions (id, profile_id, endpoint, p256dh, auth) VALUES ($1, $2, $3, 'key', 'auth')`,
		uuid.New(), id, "https://push.example.com/"+uuid.NewString())
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me", tc, map[string]any{"on_duty": true}); rec.Code != http.StatusOK {
		t.Fatalf("on duty %d %s", rec.Code, rec.Body.String())
	}
	return id, tc
}

func coachRequests(t *testing.T, h http.Handler, tc string) coachRequestsView {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/coach/requests", tc, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("coach requests %d %s", rec.Code, rec.Body.String())
	}
	var out coachRequestsView
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}

func reviewStatus(t *testing.T, h http.Handler, tu, id string) string {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/coaching/requests/"+id, tu, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get %d %s", rec.Code, rec.Body.String())
	}
	var out reviewRequestView
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out.Status
}

func tick(t *testing.T, h http.Handler, secret string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/internal/coaching/tick", nil)
	if secret != "" {
		req.Header.Set("X-Coaching-Tick-Secret", secret)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func dueIn24h(due, before, after time.Time) bool {
	return !due.Before(before.Add(24*time.Hour).Truncate(time.Second)) && !due.After(after.Add(24*time.Hour))
}

func TestReviewStartLocksCancellation(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupCoachChoice(t)
	admin := adminToken(t, h)
	coachID, coach := onDutyCoach(t, h, admin)
	_, athlete := demoRequester(t, h, admin)

	id := chosenReview(t, h, athlete, coachID.String(), "card", false)
	payChosenReview(t, h, id, "card")
	if got := coachRequests(t, h, coach).Reviewing; len(got) != 1 || got[0].ID != id || got[0].VideoURL == "" {
		t.Fatalf("request must immediately appear with its video: %+v", got)
	}
	var assigned, due time.Time
	queryRowArgs(t, "SELECT assigned_at, due_at FROM review_requests WHERE id=$1", []any{id}, &assigned, &due)
	if due.Sub(assigned) != 24*time.Hour {
		t.Fatalf("due %s, assigned %s", due, assigned)
	}
	athletePath := "/v1/coaching/requests/" + id
	coachPath := "/v1/coach/requests/" + id
	if rec := doJSON(t, h, "POST", athletePath+"/response", coach, responseBody()); rec.Code != http.StatusConflict {
		t.Fatalf("response before start: %d %s", rec.Code, rec.Body)
	}
	if rec := doJSON(t, h, "POST", coachPath+"/start", coach, nil); rec.Code != http.StatusOK {
		t.Fatalf("start: %d %s", rec.Code, rec.Body)
	}
	if rec := doJSON(t, h, "POST", athletePath+"/cancel", athlete, nil); rec.Code != http.StatusConflict {
		t.Fatalf("cancel after start: %d %s", rec.Code, rec.Body)
	}
	if rec := doJSON(t, h, "POST", athletePath+"/response", coach, responseBody()); rec.Code != http.StatusOK {
		t.Fatalf("response after start: %d %s", rec.Code, rec.Body)
	}
}
