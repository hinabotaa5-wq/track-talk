package httpx_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

// 口座ゲート（本体 #1063）：Stripe 設定済みの環境では、承認済みでも受け取り口座の手続きが済んでいない
// （payouts_enabled = false）コーチを公開面（一覧・詳細・継続プランの一覧）に出さない。

func setCoachPayoutsEnabled(t *testing.T, coach uuid.UUID, enabled bool) {
	t.Helper()
	execSQLArgs(t, `UPDATE coach_profiles SET payouts_enabled = $2 WHERE profile_id = $1`, coach, enabled)
}

// A1・A2：Stripe 設定あり。口座未完了は一覧に出ず詳細は 404、口座済みは従来どおり出る。
func TestPublicCoachesHideCoachWithoutPayouts(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripeConfig(t, func(c *config.Config) { c.CoachingCoachList = true })
	admin := adminToken(t, h)
	ready, _ := onDutyCoach(t, h, admin)
	notReady, _ := onDutyCoach(t, h, admin)
	setCoachPayoutsEnabled(t, notReady, false)

	r := doJSON(t, h, http.MethodGet, "/v1/coaches", "", nil)
	if r.Code != http.StatusOK || !strings.Contains(r.Body.String(), ready.String()) || strings.Contains(r.Body.String(), notReady.String()) {
		t.Fatalf("coaches %d %s", r.Code, r.Body.String())
	}
	if r := doJSON(t, h, http.MethodGet, "/v1/coaches/"+notReady.String(), "", nil); r.Code != http.StatusNotFound || !strings.Contains(r.Body.String(), "coach_not_public") {
		t.Fatalf("not ready detail %d %s", r.Code, r.Body.String())
	}
	if r := doJSON(t, h, http.MethodGet, "/v1/coaches/"+ready.String(), "", nil); r.Code != http.StatusOK || !strings.Contains(r.Body.String(), ready.String()) {
		t.Fatalf("ready detail %d %s", r.Code, r.Body.String())
	}
	// A8：指名候補の既存ゲート（口座未完了は候補に出ない）も従来どおり。
	_, tu := demoRequester(t, h, admin)
	if r := doJSON(t, h, http.MethodGet, "/v1/coaching/nominees?event=100m", tu, nil); r.Code != http.StatusOK || !strings.Contains(r.Body.String(), ready.String()) || strings.Contains(r.Body.String(), notReady.String()) {
		t.Fatalf("nominees %d %s", r.Code, r.Body.String())
	}
	// 口座の手続きが済めば（webhook で payouts_enabled が立てば）次から出る。
	setCoachPayoutsEnabled(t, notReady, true)
	if r := doJSON(t, h, http.MethodGet, "/v1/coaches/"+notReady.String(), "", nil); r.Code != http.StatusOK {
		t.Fatalf("detail after payouts %d %s", r.Code, r.Body.String())
	}
}

// A3：Stripe 未設定（デモ・ローカル）では、payouts_enabled = false でも従来どおり出る。
func TestPublicCoachesWithoutStripeIgnorePayouts(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setupConfig(t, func(c *config.Config) { c.CoachingCoachList = true })
	admin := adminToken(t, h)
	coach, _ := onDutyCoach(t, h, admin)
	setCoachPayoutsEnabled(t, coach, false)
	if r := doJSON(t, h, http.MethodGet, "/v1/coaches", "", nil); r.Code != http.StatusOK || !strings.Contains(r.Body.String(), coach.String()) {
		t.Fatalf("coaches %d %s", r.Code, r.Body.String())
	}
	if r := doJSON(t, h, http.MethodGet, "/v1/coaches/"+coach.String(), "", nil); r.Code != http.StatusOK {
		t.Fatalf("detail %d %s", r.Code, r.Body.String())
	}
}

// A7：継続プランの一覧（support-plan）でも、口座未完了のコーチは出ない。
func TestSupportPlanListHidesCoachWithoutPayouts(t *testing.T) {
	h, _ := setupStripeConfig(t, nil)
	coach, _ := consultCoach(t, h)
	if body := doJSON(t, h, http.MethodGet, "/v1/coaches/support-plan", "", nil).Body.String(); !strings.Contains(body, coach.String()) {
		t.Fatalf("ready coach missing: %s", body)
	}
	setCoachPayoutsEnabled(t, coach, false)
	if body := doJSON(t, h, http.MethodGet, "/v1/coaches/support-plan", "", nil).Body.String(); strings.Contains(body, coach.String()) {
		t.Fatalf("coach without payouts listed: %s", body)
	}
}
