package httpx_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

// 特集モード（本体 #985）：COACHING_FEATURED_COACH_ID の間は、一覧・詳細・継続プランの一覧・指名候補・申込みの指名を
// 特集コーチだけに絞る。
func TestFeaturedCoachModeHidesOtherCoaches(t *testing.T) {
	fakeCoachingCloudinary(t)
	featured := uuid.New()
	h, _ := setupStripeConfig(t, func(c *config.Config) {
		c.ReviewRandomAssign = false
		c.CoachingCoachList = true
		c.FeaturedCoachID = featured.String()
	})
	admin := adminToken(t, h)
	onDutyCoachID(t, h, admin, featured)
	other, _ := onDutyCoach(t, h, admin)
	requester, tu := demoRequester(t, h, admin)

	for _, tok := range []string{"", tu} {
		r := doJSON(t, h, http.MethodGet, "/v1/coaches", tok, nil)
		if r.Code != http.StatusOK || !strings.Contains(r.Body.String(), featured.String()) || strings.Contains(r.Body.String(), other.String()) {
			t.Fatalf("featured coaches %d %s", r.Code, r.Body.String())
		}
	}
	// 直URLの詳細も、外したコーチは非公開と同じ 404。
	if r := doJSON(t, h, http.MethodGet, "/v1/coaches/"+other.String(), "", nil); r.Code != http.StatusNotFound || !strings.Contains(r.Body.String(), "coach_not_public") {
		t.Fatalf("other detail %d %s", r.Code, r.Body.String())
	}
	if r := doJSON(t, h, http.MethodGet, "/v1/coaches/"+featured.String(), "", nil); r.Code != http.StatusOK {
		t.Fatalf("featured detail %d %s", r.Code, r.Body.String())
	}
	for _, path := range []string{"/v1/coaches/support-plan", "/v1/coaches/" + other.String() + "/support-consult", "/v1/coaches/" + other.String() + "/availability"} {
		if r := doJSON(t, h, http.MethodGet, path, "", nil); strings.Contains(r.Body.String(), other.String()) {
			t.Fatalf("%s exposes other coach: %d %s", path, r.Code, r.Body.String())
		}
	}
	// 指名候補も特集コーチだけ。
	r := doJSON(t, h, http.MethodGet, "/v1/coaching/nominees?event=100m", tu, nil)
	if r.Code != http.StatusOK || !strings.Contains(r.Body.String(), featured.String()) || strings.Contains(r.Body.String(), other.String()) {
		t.Fatalf("nominees %d %s", r.Code, r.Body.String())
	}
	// 外したコーチを指名した申込みは、受付していないコーチと同じく断る。
	body := reviewBodyFor(t, h, tu)
	body["method"] = "card"
	body["requested_coach_id"] = other.String()
	if r := createReviewWith(t, h, tu, "card", body); r.Code != http.StatusUnprocessableEntity || !strings.Contains(r.Body.String(), "requested_coach_id") {
		t.Fatalf("nominate other %d %s", r.Code, r.Body.String())
	}
	// コーチを選ばずに送った申込みは、特集コーチへの指名になる。
	body = reviewBodyFor(t, h, tu)
	body["method"] = "card"
	if r := createReviewWith(t, h, tu, "card", body); r.Code >= 300 {
		t.Fatalf("no nomination %d %s", r.Code, r.Body.String())
	}
	var requested string
	queryRowArgs(t, `SELECT requested_coach_id::text FROM review_requests WHERE requester_id=$1 ORDER BY created_at DESC LIMIT 1`, []any{requester}, &requested)
	if requested != featured.String() {
		t.Fatalf("requested %s, want %s", requested, featured)
	}
}

// 特集を切ると、従来どおり全員が出る。
func TestFeaturedCoachModeOffShowsAll(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripeConfig(t, func(c *config.Config) { c.CoachingCoachList = true })
	admin := adminToken(t, h)
	a, _ := onDutyCoach(t, h, admin)
	b, _ := onDutyCoach(t, h, admin)
	if r := doJSON(t, h, http.MethodGet, "/v1/coaches", "", nil); r.Code != http.StatusOK || !strings.Contains(r.Body.String(), a.String()) || !strings.Contains(r.Body.String(), b.String()) {
		t.Fatalf("coaches %d %s", r.Code, r.Body.String())
	}
}

// 特集の値が ID として読めないときは、誰も出さない（他のコーチが出る側には倒さない）。
func TestFeaturedCoachModeInvalidIDShowsNobody(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripeConfig(t, func(c *config.Config) {
		c.CoachingCoachList = true
		c.FeaturedCoachID = "not-a-uuid"
	})
	coach, _ := onDutyCoach(t, h, adminToken(t, h))
	if r := doJSON(t, h, http.MethodGet, "/v1/coaches", "", nil); r.Code != http.StatusOK || strings.Contains(r.Body.String(), coach.String()) {
		t.Fatalf("coaches %d %s", r.Code, r.Body.String())
	}
}
