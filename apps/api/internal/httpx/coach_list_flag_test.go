package httpx_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

// コーチ一覧の公開は COACHING_COACH_LIST_ENABLED で、コーチングの開放とは別に切り替える。
// 状態ごとに、一覧 API と動画添削の申込み（指名候補 → 申込み → 支払い）の動きを確かめる。

// コーチングを閉じている間は、一覧のフラグによらず一覧も指名候補も 404（現行どおり）。
func TestCoachListFlagCoachingOff(t *testing.T) {
	for _, list := range []bool{false, true} {
		h := setupConfig(t, func(c *config.Config) {
			c.CoachingEnabled = false
			c.CoachingCoachList = list
		})
		tu := token(t, uuid.New())
		if r := doJSON(t, h, http.MethodGet, "/v1/coaches", "", nil); r.Code != http.StatusNotFound {
			t.Fatalf("list=%v coaches %d %s", list, r.Code, r.Body.String())
		}
		if r := doJSON(t, h, http.MethodGet, "/v1/coaching/nominees?event=100m", tu, nil); r.Code != http.StatusNotFound {
			t.Fatalf("list=%v nominees %d %s", list, r.Code, r.Body.String())
		}
	}
}

// コーチングを開けて一覧を閉じている間は、一覧 API はコーチングを閉じたときと同じ 404 で、
// コーチの名前・写真などは一覧から取れない。動画添削の指名候補と申込み・支払いは従来どおり使える。
func TestCoachListFlagOffKeepsReviewRequests(t *testing.T) {
	closed := doJSON(t, setupNoCoaching(t), http.MethodGet, "/v1/coaches", "", nil)

	fakeCoachingCloudinary(t)
	h, _ := setupStripeConfig(t, func(c *config.Config) {
		c.ReviewRandomAssign = false
		c.CoachingCoachList = false
	})
	admin := adminToken(t, h)
	coach, _ := onDutyCoach(t, h, admin)
	_, tu := demoRequester(t, h, admin)

	for _, tok := range []string{"", tu} {
		r := doJSON(t, h, http.MethodGet, "/v1/coaches", tok, nil)
		if r.Code != closed.Code || r.Body.String() != closed.Body.String() {
			t.Fatalf("coaches %d %s, want %d %s", r.Code, r.Body.String(), closed.Code, closed.Body.String())
		}
		if strings.Contains(r.Body.String(), coach.String()) {
			t.Fatal("coach exposed through the list")
		}
	}

	r := doJSON(t, h, http.MethodGet, "/v1/coaching/nominees?event=100m", tu, nil)
	if r.Code != http.StatusOK || !strings.Contains(r.Body.String(), coach.String()) {
		t.Fatalf("nominees %d %s", r.Code, r.Body.String())
	}
	id := chosenReview(t, h, tu, coach.String(), "card", false)
	payChosenReview(t, h, id, "card")
	var status, requested string
	queryRowArgs(t, `SELECT p.status, r.requested_coach_id::text FROM review_payments p JOIN review_requests r ON r.id = p.request_id WHERE p.request_id=$1`, []any{id}, &status, &requested)
	if status != "authorized" || requested != coach.String() {
		t.Fatalf("payment %s coach %s", status, requested)
	}
}

// コーチングと一覧の両方を開けると、受付中のコーチが一覧に出る（現行どおり）。
func TestCoachListFlagOnShowsCoaches(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripeConfig(t, func(c *config.Config) {
		c.ReviewRandomAssign = false
		c.CoachingCoachList = true
	})
	admin := adminToken(t, h)
	coach, _ := onDutyCoach(t, h, admin)
	r := doJSON(t, h, http.MethodGet, "/v1/coaches", "", nil)
	if r.Code != http.StatusOK || !strings.Contains(r.Body.String(), coach.String()) {
		t.Fatalf("coaches %d %s", r.Code, r.Body.String())
	}
}
