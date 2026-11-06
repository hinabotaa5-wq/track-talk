package httpx_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func coachSuspendPath(id uuid.UUID, action string) string {
	return "/v1/admin/coaching/coaches/" + id.String() + "/" + action
}

// 運営が止めたコーチは、ログインしたままのトークンでも、次のリクエストからコーチの操作ができない。
// アカウントは残り、ふつうの利用はできる。戻すとまたコーチとして使える。
func TestAdminSuspendCoach(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setup(t)
	tadmin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, tadmin)
	other, to := onDutyCoach(t, h, tadmin)

	rec := doJSON(t, h, http.MethodPost, coachSuspendPath(coach, "suspend"), tadmin, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"suspended"`) {
		t.Fatalf("suspend %d %s", rec.Code, rec.Body.String())
	}
	var onDuty bool
	queryRowArgs(t, `SELECT on_duty FROM coach_profiles WHERE profile_id = $1`, []any{coach}, &onDuty)
	if onDuty {
		t.Fatal("suspended coach still on duty")
	}
	if body := doJSON(t, h, http.MethodGet, "/v1/coaches", "", nil).Body.String(); strings.Contains(body, coach.String()) || !strings.Contains(body, other.String()) {
		t.Fatalf("coach list after suspend %s", body)
	}

	// 止める前に発行されたトークンのまま、コーチの操作をしてみる。
	for _, op := range []struct {
		name, method, path string
		body               any
	}{
		{"requests", http.MethodGet, "/v1/coach/requests", nil},
		{"on_duty", http.MethodPatch, "/v1/coach/me", map[string]any{"on_duty": true}},
		{"payout_onboarding", http.MethodPost, "/v1/coach/stripe/onboarding", nil},
	} {
		if rec := doJSON(t, h, op.method, op.path, tc, op.body); rec.Code != http.StatusForbidden {
			t.Errorf("%s: suspended coach got %d, want 403: %s", op.name, rec.Code, rec.Body.String())
		}
	}
	// 本人がやめた扱いにして応募し直すことはできない。
	if rec := doJSON(t, h, http.MethodPost, coachWithdrawPath, tc, withdrawCoachBody()); rec.Code != http.StatusConflict {
		t.Errorf("suspended coach withdraw %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", tc, coachApplicationBody()); rec.Code == http.StatusOK {
		t.Errorf("suspended coach reapplied %s", rec.Body.String())
	}
	// アカウントは残る。
	if rec := doJSON(t, h, http.MethodGet, "/v1/me", tc, nil); rec.Code != http.StatusOK {
		t.Errorf("me after suspend %d", rec.Code)
	}
	// 止めていないコーチは、これまでどおり使える。
	if rec := doJSON(t, h, http.MethodGet, "/v1/coach/requests", to, nil); rec.Code != http.StatusOK {
		t.Errorf("other coach requests %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, coachSuspendPath(coach, "suspend"), tadmin, nil); rec.Code != http.StatusConflict {
		t.Errorf("suspend twice %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodPost, coachSuspendPath(coach, "resume"), tadmin, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"approved"`) {
		t.Fatalf("resume %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/coach/requests", tc, nil); rec.Code != http.StatusOK {
		t.Errorf("resumed coach requests %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me", tc, map[string]any{"on_duty": true}); rec.Code != http.StatusOK {
		t.Errorf("resumed coach on duty %d %s", rec.Code, rec.Body.String())
	}
	if body := doJSON(t, h, http.MethodGet, "/v1/coaches", "", nil).Body.String(); !strings.Contains(body, coach.String()) {
		t.Errorf("resumed coach not listed %s", body)
	}
	if rec := doJSON(t, h, http.MethodPost, coachSuspendPath(coach, "resume"), tadmin, nil); rec.Code != http.StatusConflict {
		t.Errorf("resume twice %d %s", rec.Code, rec.Body.String())
	}
}

// 止める・戻すのは運営だけ。承認済みでない応募は止められない。
func TestAdminSuspendCoachAuthorization(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setup(t)
	tadmin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, tadmin)

	for _, action := range []string{"suspend", "resume"} {
		if rec := doJSON(t, h, http.MethodPost, coachSuspendPath(coach, action), tc, nil); rec.Code != http.StatusForbidden {
			t.Errorf("%s by coach %d, want 403", action, rec.Code)
		}
		if rec := doJSON(t, h, http.MethodPost, coachSuspendPath(uuid.New(), action), tadmin, nil); rec.Code != http.StatusNotFound {
			t.Errorf("%s unknown %d, want 404", action, rec.Code)
		}
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/coach/requests", tc, nil); rec.Code != http.StatusOK {
		t.Errorf("coach after denied suspend %d", rec.Code)
	}

	applicant := uuid.New()
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", token(t, applicant), coachApplicationBody()); rec.Code != http.StatusOK {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, coachSuspendPath(applicant, "suspend"), tadmin, nil); rec.Code != http.StatusConflict {
		t.Errorf("suspend pending %d, want 409", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, coachSuspendPath(applicant, "resume"), tadmin, nil); rec.Code != http.StatusConflict {
		t.Errorf("resume pending %d, want 409", rec.Code)
	}
}

// 止めたコーチに返事待ちの打診があれば、断った扱いにして依頼を待ちに戻す。
func TestAdminSuspendCoachHandsOffOpenOffers(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setup(t)
	tadmin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, tadmin)
	_, tu := demoRequester(t, h, tadmin)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, reviewBodyFor(t, h, tu)); rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	offers := coachRequests(t, h, tc).Offers
	if len(offers) != 1 {
		t.Fatalf("offers %+v", offers)
	}
	if rec := doJSON(t, h, http.MethodPost, coachSuspendPath(coach, "suspend"), tadmin, nil); rec.Code != http.StatusOK {
		t.Fatalf("suspend %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/offers/"+offers[0].ID+"/accept", tc, nil); rec.Code != http.StatusForbidden {
		t.Errorf("suspended coach accepted offer %d %s", rec.Code, rec.Body.String())
	}
	if status := reviewStatus(t, h, tu, offers[0].Request.ID); status != "queued" {
		t.Errorf("request after suspend %s", status)
	}
	var result string
	queryRowArgs(t, `SELECT result FROM review_offers WHERE id = $1`, []any{offers[0].ID}, &result)
	if result != "declined" {
		t.Errorf("offer result %s", result)
	}
}

// 止めているあいだは、報酬を送金しない。戻すと次の送金でまとめて送る。止めていないコーチには、これまでどおり送る。
func TestAdminSuspendCoachHoldsPayouts(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupStripe(t)
	tadmin := adminToken(t, h)
	coach, _ := onDutyCoach(t, h, tadmin)
	other, _ := onDutyCoach(t, h, tadmin)
	for _, id := range []uuid.UUID{coach, other} {
		execSQLArgs(t, `INSERT INTO coach_earnings (id, coach_id, source, gross_yen, share_percent, amount_yen, confirmed_at)
			VALUES (gen_random_uuid(), $1, 'review', 6000, 70, 4200, `+lastMonthJST+`)`, id)
	}
	run := func() string {
		rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/payouts/run", tadmin, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("run %d %s", rec.Code, rec.Body.String())
		}
		return rec.Body.String()
	}
	if rec := doJSON(t, h, http.MethodPost, coachSuspendPath(coach, "suspend"), tadmin, nil); rec.Code != http.StatusOK {
		t.Fatalf("suspend %d %s", rec.Code, rec.Body.String())
	}
	if out := run(); !strings.Contains(out, `"created":1,"paid":1`) {
		t.Fatalf("run while suspended %s", out)
	}
	if got := f.transferList(); len(got) != 1 || got[0].Get("destination") != "acct_"+other.String() {
		t.Fatalf("transfers while suspended %v", got)
	}
	if rec := doJSON(t, h, http.MethodPost, coachSuspendPath(coach, "resume"), tadmin, nil); rec.Code != http.StatusOK {
		t.Fatalf("resume %d %s", rec.Code, rec.Body.String())
	}
	if out := run(); !strings.Contains(out, `"created":1,"paid":1`) {
		t.Fatalf("run after resume %s", out)
	}
	if got := f.transferList(); len(got) != 2 || got[1].Get("destination") != "acct_"+coach.String() || got[1].Get("amount") != "4200" {
		t.Fatalf("transfers after resume %v", got)
	}
}

// 止めたまま登録を終えると、留保していた報酬が最低額に届かなくても次の送金で送られる（最終精算）。
// 終えなければ、止めているあいだは送らないまま。誰がいつ終えたかが残る。
func TestAdminEndSuspendedCoachSettlesPayouts(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupStripe(t)
	tadmin := adminToken(t, h)
	ended, tc := onDutyCoach(t, h, tadmin)
	held, _ := onDutyCoach(t, h, tadmin)
	// どちらも最低額（3,000円）に届かない報酬。
	for _, id := range []uuid.UUID{ended, held} {
		execSQLArgs(t, `INSERT INTO coach_earnings (id, coach_id, source, gross_yen, share_percent, amount_yen, confirmed_at)
			VALUES (gen_random_uuid(), $1, 'review', 1500, 70, 1050, `+lastMonthJST+`)`, id)
		if rec := doJSON(t, h, http.MethodPost, coachSuspendPath(id, "suspend"), tadmin, nil); rec.Code != http.StatusOK {
			t.Fatalf("suspend %d %s", rec.Code, rec.Body.String())
		}
	}
	run := func() string {
		rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/payouts/run", tadmin, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("run %d %s", rec.Code, rec.Body.String())
		}
		return rec.Body.String()
	}
	if out := run(); !strings.Contains(out, `"created":0,"paid":0`) {
		t.Fatalf("run while both suspended %s", out)
	}

	rec := doJSON(t, h, http.MethodPost, coachSuspendPath(ended, "end"), tadmin, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"ended"`) {
		t.Fatalf("end %d %s", rec.Code, rec.Body.String())
	}
	var endedBy uuid.UUID
	var recorded int
	queryRowArgs(t, `SELECT count(*), max(ended_by::text)::uuid FROM coach_registration_endings WHERE coach_id = $1 AND ended_at IS NOT NULL`, []any{ended}, &recorded, &endedBy)
	if recorded != 1 || endedBy == uuid.Nil {
		t.Fatalf("ending record count=%d by=%s", recorded, endedBy)
	}
	if me := doJSON(t, h, http.MethodGet, "/v1/me", tadmin, nil).Body.String(); !strings.Contains(me, endedBy.String()) {
		t.Errorf("ended_by %s is not the admin who ended it: %s", endedBy, me)
	}

	if out := run(); !strings.Contains(out, `"created":1,"paid":1`) {
		t.Fatalf("run after end %s", out)
	}
	if got := f.transferList(); len(got) != 1 || got[0].Get("destination") != "acct_"+ended.String() || got[0].Get("amount") != "1050" {
		t.Fatalf("transfers after end %v", got)
	}
	var heldUnpaid int
	queryRowArgs(t, `SELECT count(*) FROM coach_earnings WHERE coach_id = $1 AND payout_id IS NULL`, []any{held}, &heldUnpaid)
	if heldUnpaid != 1 {
		t.Errorf("held coach earnings unpaid=%d, want 1 (kept, not paid)", heldUnpaid)
	}

	// 終えたコーチは、コーチの操作も、本人のやめる・応募し直す・戻すもできない。
	if rec := doJSON(t, h, http.MethodGet, "/v1/coach/requests", tc, nil); rec.Code != http.StatusForbidden {
		t.Errorf("ended coach requests %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, coachWithdrawPath, tc, withdrawCoachBody()); rec.Code != http.StatusConflict {
		t.Errorf("ended coach withdraw %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", tc, coachApplicationBody()); rec.Code == http.StatusOK {
		t.Errorf("ended coach reapplied %s", rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, coachSuspendPath(ended, "resume"), tadmin, nil); rec.Code != http.StatusConflict {
		t.Errorf("resume ended %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/admin/coach-applications?status=ended", tadmin, nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), ended.String()) {
		t.Errorf("admin list ended %d %s", rec.Code, rec.Body.String())
	}
	// 二度目は 409 で、記録も増えない。
	if rec := doJSON(t, h, http.MethodPost, coachSuspendPath(ended, "end"), tadmin, nil); rec.Code != http.StatusConflict {
		t.Errorf("end twice %d %s", rec.Code, rec.Body.String())
	}
	queryRowArgs(t, `SELECT count(*) FROM coach_registration_endings WHERE coach_id = $1`, []any{ended}, &recorded)
	if recorded != 1 {
		t.Errorf("ending records after twice %d", recorded)
	}
}

// 登録を終えるのは運営だけで、止めているコーチだけが対象。
func TestAdminEndCoachAuthorization(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setup(t)
	tadmin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, tadmin)

	// 承認済み（止めていない）コーチは終えられない。
	if rec := doJSON(t, h, http.MethodPost, coachSuspendPath(coach, "end"), tadmin, nil); rec.Code != http.StatusConflict {
		t.Errorf("end approved %d, want 409", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, coachSuspendPath(coach, "suspend"), tadmin, nil); rec.Code != http.StatusOK {
		t.Fatalf("suspend %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, coachSuspendPath(coach, "end"), tc, nil); rec.Code != http.StatusForbidden {
		t.Errorf("end by coach %d, want 403", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, coachSuspendPath(coach, "end"), token(t, uuid.New()), nil); rec.Code != http.StatusForbidden {
		t.Errorf("end by user %d, want 403", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, coachSuspendPath(uuid.New(), "end"), tadmin, nil); rec.Code != http.StatusNotFound {
		t.Errorf("end unknown %d, want 404", rec.Code)
	}
	var status string
	queryRowArgs(t, `SELECT status FROM coach_profiles WHERE profile_id = $1`, []any{coach}, &status)
	if status != "suspended" {
		t.Errorf("status after denied end %s", status)
	}
}

// 終えたあとはコーチが口座を登録し直せないので、未送金の報酬があるのに口座がなければ終えない。
// 利用者とのやりとりが残っているあいだも終えない。どちらも止めたまま（報酬は残る）。
func TestAdminEndCoachBlockers(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setup(t)
	tadmin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, tadmin)
	_, tu := demoRequester(t, h, tadmin)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, reviewBodyFor(t, h, tu)); rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	offers := coachRequests(t, h, tc).Offers
	if len(offers) != 1 {
		t.Fatalf("offers %+v", offers)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/offers/"+offers[0].ID+"/accept", tc, nil); rec.Code != http.StatusOK {
		t.Fatalf("accept %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, coachSuspendPath(coach, "suspend"), tadmin, nil); rec.Code != http.StatusOK {
		t.Fatalf("suspend %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, coachSuspendPath(coach, "end"), tadmin, nil); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "coaching_in_progress") {
		t.Errorf("end with review in progress %d %s", rec.Code, rec.Body.String())
	}

	var status string
	queryRowArgs(t, `SELECT status FROM coach_profiles WHERE profile_id = $1`, []any{coach}, &status)
	if status != "suspended" {
		t.Errorf("status after blocked end %s", status)
	}
}

func TestAdminEndCoachNeedsPayoutAccount(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	tadmin := adminToken(t, h)
	other, _ := onDutyCoach(t, h, tadmin)
	execSQLArgs(t, `UPDATE coach_profiles SET payouts_enabled = false, stripe_account_id = NULL WHERE profile_id = $1`, other)
	execSQLArgs(t, `INSERT INTO coach_earnings (id, coach_id, source, gross_yen, share_percent, amount_yen, confirmed_at) VALUES (gen_random_uuid(), $1, 'review', 1000, 70, 700, now())`, other)
	if rec := doJSON(t, h, http.MethodPost, coachSuspendPath(other, "suspend"), tadmin, nil); rec.Code != http.StatusOK {
		t.Fatalf("suspend other %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, coachSuspendPath(other, "end"), tadmin, nil); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "payouts_not_ready") {
		t.Errorf("end without payout account %d %s", rec.Code, rec.Body.String())
	}
	var status string
	var endings int
	queryRowArgs(t, `SELECT status, (SELECT count(*) FROM coach_registration_endings WHERE coach_id = $1) FROM coach_profiles WHERE profile_id = $1`, []any{other}, &status, &endings)
	if status != "suspended" || endings != 0 {
		t.Errorf("after blocked end: status=%s endings=%d", status, endings)
	}
}
