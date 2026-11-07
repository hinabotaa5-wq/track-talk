package httpx_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

func injectNotificationFailure(t *testing.T, table, event string) func() {
	t.Helper()
	// 呼び出し元はテスト内の固定テーブル名と操作だけを渡す。
	execSQLArgs(t, `CREATE OR REPLACE FUNCTION fail_notification_test() RETURNS trigger
		LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected failure'; END $$`)
	execSQLArgs(t, "CREATE TRIGGER fail_notification_test BEFORE "+event+" ON "+table+
		" FOR EACH ROW EXECUTE FUNCTION fail_notification_test()")
	remove := func() {
		execSQLArgs(t, "DROP TRIGGER IF EXISTS fail_notification_test ON "+table)
	}
	t.Cleanup(remove)
	return remove
}

func retryAllNotificationJobs(t *testing.T, h http.Handler) {
	t.Helper()
	execSQLArgs(t, "UPDATE notification_jobs SET next_attempt_at = now()")
	if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK {
		t.Fatalf("tick %d %s", rec.Code, rec.Body.String())
	}
}

func TestPostActionsCommitWhenNotificationInsertFails(t *testing.T) {
	h := setupConfig(t, func(cfg *config.Config) { cfg.CoachingTickSecret = "tick-secret" })
	hostID, guestID := uuid.New(), uuid.New()
	host, guest := token(t, hostID), token(t, guestID)
	if rec := doJSON(t, h, "GET", "/v1/me", guest, nil); rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}
	remove := injectNotificationFailure(t, "notifications", "INSERT")
	rec := doJSON(t, h, "POST", "/v1/posts", host, map[string]any{
		"kind": "practice", "body": "ジョグから入ります", "venue": "〇〇競技場", "event": "100m",
		"starts_at": time.Now().Add(72 * time.Hour).Format(time.RFC3339), "capacity": 2, "prefecture": "13",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("practice creation %d %s", rec.Code, rec.Body.String())
	}
	var post struct{ ID string }
	if err := json.Unmarshal(rec.Body.Bytes(), &post); err != nil {
		t.Fatal(err)
	}
	base := "/v1/posts/" + post.ID
	if rec := doJSON(t, h, "POST", base+"/comments", guest, map[string]string{"body": "参加したいです"}); rec.Code != http.StatusCreated {
		t.Fatalf("comment %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, "POST", base+"/practice/applications", guest, nil); rec.Code != http.StatusCreated {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
	var application string
	queryRowArgs(t, "SELECT id::text FROM practice_applications WHERE post_id = $1 AND user_id = $2", []any{post.ID, guestID}, &application)
	if rec := doJSON(t, h, "POST", base+"/practice/applications/"+application+"/accept", host, nil); rec.Code != http.StatusOK {
		t.Fatalf("accept %d %s", rec.Code, rec.Body.String())
	}
	var status string
	queryRowArgs(t, "SELECT status FROM practice_applications WHERE id = $1", []any{application}, &status)
	if status != "accepted" {
		t.Fatalf("application status %s", status)
	}
	practiceHeld(t, post.ID)
	if rec := doJSON(t, h, "POST", base+"/practice/dissolve", host, nil); rec.Code != http.StatusOK {
		t.Fatalf("dissolve %d %s", rec.Code, rec.Body.String())
	}
	var jobs int
	queryRowArgs(t, "SELECT count(*) FROM notification_jobs WHERE post_id = $1", []any{post.ID}, &jobs)
	if jobs != 6 {
		t.Fatalf("pending jobs %d", jobs)
	}

	remove()
	retryAllNotificationJobs(t, h)
	retryAllNotificationJobs(t, h)
	hostNotes, _ := listNotifications(t, h, host)
	guestNotes, _ := listNotifications(t, h, guest)
	for _, want := range []struct {
		notes []noteItem
		kind  string
	}{{hostNotes, "comment"}, {hostNotes, "practice_application"}, {guestNotes, "recruitment_post"}, {guestNotes, "practice_accepted"}, {hostNotes, "practice_rate"}, {guestNotes, "practice_rate"}} {
		if got := len(notesFor(want.notes, post.ID, want.kind)); got != 1 {
			t.Errorf("%s count = %d", want.kind, got)
		}
	}
}

func TestReviewOfferCommitsWhenNotificationInsertFails(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupCoachChoice(t)
	admin := adminToken(t, h)
	coach, coachToken := onDutyCoach(t, h, admin)
	_, requester := demoRequester(t, h, admin)
	id := chosenReview(t, h, requester, coach.String(), "card", false)
	remove := injectNotificationFailure(t, "notifications", "INSERT")
	payChosenReview(t, h, id, "card")
	if got := coachRequests(t, h, coachToken).Offers; len(got) != 1 || got[0].Request.ID != id {
		t.Fatalf("offer did not commit: %+v", got)
	}
	remove()
	retryAllNotificationJobs(t, h)
	retryAllNotificationJobs(t, h)
	var count int
	queryRowArgs(t, "SELECT count(*) FROM notifications WHERE review_request_id = $1 AND kind = 'review_offer'", []any{id}, &count)
	if count != 1 {
		t.Fatalf("offer notification count %d", count)
	}
}

func notificationPushRecorder(t *testing.T, record func()) (func(*config.Config), string) {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		record()
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(server.Close)
	private, public, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	return func(cfg *config.Config) {
		cfg.VAPIDPublicKey = public
		cfg.VAPIDPrivateKey = private
		cfg.VAPIDSubject = "mailto:test@example.com"
		cfg.PushHTTP = server.Client()
	}, server.URL
}

func subscribeNotificationTest(t *testing.T, h http.Handler, token, endpoint string) {
	t.Helper()
	p256dh, auth := testPushKeys(t)
	if rec := doJSON(t, h, "POST", "/v1/me/push-subscriptions", token, map[string]any{
		"endpoint": endpoint, "keys": map[string]string{"p256dh": p256dh, "auth": auth},
	}); rec.Code != http.StatusNoContent {
		t.Fatalf("subscribe %d %s", rec.Code, rec.Body.String())
	}
}

func fakeIdentityEnd(t *testing.T, status *atomic.Int32, calls *atomic.Int32, seenAuth *atomic.Value, seenBody *atomic.Value) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/coach-end" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		calls.Add(1)
		seenAuth.Store(r.Header.Get("Authorization"))
		raw, _ := io.ReadAll(r.Body)
		seenBody.Store(string(raw))
		w.WriteHeader(int(status.Load()))
	}))
	t.Cleanup(server.Close)
	return server
}

func endJobCount(t *testing.T, applicant uuid.UUID) int {
	t.Helper()
	var pending int
	queryRowArgs(t, "SELECT count(*) FROM notification_jobs WHERE kind = 'coach_identity_end' AND recipient_id = $1", []any{applicant}, &pending)
	return pending
}

func retiredCount(t *testing.T, applicant uuid.UUID) int {
	t.Helper()
	var retired int
	queryRowArgs(t, "SELECT count(*) FROM identity_boundary.retirement WHERE coach_id = $1", []any{applicant}, &retired)
	return retired
}

func TestCoachRejectionNotificationRecoversIndependentlyOfIdentity(t *testing.T) {
	var pushes atomic.Int32
	configurePush, endpoint := notificationPushRecorder(t, func() { pushes.Add(1) })
	endToken := strings.Repeat("e", 32)
	var endCalls atomic.Int32
	var endAuth atomic.Value
	var endBody atomic.Value
	endStatus := atomic.Int32{}
	endStatus.Store(500)
	fakeVault := fakeIdentityEnd(t, &endStatus, &endCalls, &endAuth, &endBody)
	h := setupConfig(t, func(cfg *config.Config) {
		configurePush(cfg)
		cfg.CoachingTickSecret = "tick-secret"
		cfg.IdentityReaderURL = fakeVault.URL
		cfg.IdentityEndToken = endToken
	})
	applicant := uuid.New()
	applicantToken := token(t, applicant)
	if rec := doJSON(t, h, "POST", "/v1/coach/applications", applicantToken, coachApplicationBody()); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	subscribeNotificationTest(t, h, applicantToken, endpoint)
	removeBellFailure := injectNotificationFailure(t, "notifications", "INSERT")
	admin := adminToken(t, h)
	path := "/v1/admin/coach-applications/" + applicant.String() + "/reject"
	reject := func() {
		t.Helper()
		if rec := doJSON(t, h, "POST", path, admin, nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"rejected"`) {
			t.Fatalf("reject %d %s", rec.Code, rec.Body.String())
		}
	}
	reject()
	if pushes.Load() != 0 {
		t.Fatal("push sent before notification recovery")
	}
	removeBellFailure()
	reject()
	reject()
	retryAllNotificationJobs(t, h)
	notes, _ := listNotifications(t, h, applicantToken)
	if pushes.Load() != 1 || len(notes) != 1 || notes[0].Kind != "message" {
		t.Fatalf("pushes = %d, bell count = %d", pushes.Load(), len(notes))
	}
	if got := endJobCount(t, applicant); got != 1 {
		t.Fatalf("pending end jobs = %d", got)
	}
	if got := retiredCount(t, applicant); got != 1 {
		t.Fatalf("trigger did not record synchronously: retired = %d", got)
	}
	if endCalls.Load() == 0 {
		t.Fatal("vault end was never contacted")
	}
	if got, _ := endAuth.Load().(string); got != "Bearer "+endToken {
		t.Fatalf("vault end auth = %q", got)
	}
	endStatus.Store(204)
	retryAllNotificationJobs(t, h)
	if got := endJobCount(t, applicant); got != 0 {
		t.Fatalf("pending end jobs after recovery = %d", got)
	}
	if got := retiredCount(t, applicant); got != 1 {
		t.Fatalf("retired rows after recovery = %d", got)
	}
	if got, _ := endBody.Load().(string); got != `{"coach_id":"`+applicant.String()+`"}` {
		t.Fatalf("vault end body = %s", got)
	}
	retryAllNotificationJobs(t, h)
	if got := retiredCount(t, applicant); got != 1 || pushes.Load() != 1 {
		t.Fatalf("duplicate effect: retired = %d, pushes = %d", got, pushes.Load())
	}
}

func TestCoachIdentityEndDuplicateJobsRecordOnce(t *testing.T) {
	endToken := strings.Repeat("e", 32)
	var endCalls atomic.Int32
	var endAuth atomic.Value
	var endBody atomic.Value
	endStatus := atomic.Int32{}
	endStatus.Store(204)
	fakeVault := fakeIdentityEnd(t, &endStatus, &endCalls, &endAuth, &endBody)
	h := setupConfig(t, func(cfg *config.Config) {
		cfg.CoachingTickSecret = "tick-secret"
		cfg.IdentityReaderURL = fakeVault.URL
		cfg.IdentityEndToken = endToken
	})
	applicant := uuid.New()
	applicantToken := token(t, applicant)
	if rec := doJSON(t, h, "POST", "/v1/coach/applications", applicantToken, coachApplicationBody()); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	admin := adminToken(t, h)
	path := "/v1/admin/coach-applications/" + applicant.String() + "/reject"
	if rec := doJSON(t, h, "POST", path, admin, nil); rec.Code != http.StatusOK {
		t.Fatalf("reject %d %s", rec.Code, rec.Body.String())
	}
	var reviewedAt time.Time
	queryRowArgs(t, "SELECT reviewed_at FROM coach_profiles WHERE profile_id = $1", []any{applicant}, &reviewedAt)
	for range 2 {
		execSQLArgs(t, `INSERT INTO notification_jobs (id, kind, recipient_id, reviewed_at) VALUES ($1, 'coach_identity_end', $2, $3)`,
			uuid.New(), applicant, reviewedAt)
	}
	retryAllNotificationJobs(t, h)
	if got := endCalls.Load(); got != 3 {
		t.Fatalf("vault end calls = %d", got)
	}
	if got := retiredCount(t, applicant); got != 1 {
		t.Fatalf("retired rows = %d", got)
	}
	if got := endJobCount(t, applicant); got != 0 {
		t.Fatalf("pending end jobs = %d", got)
	}
}

func TestCoachIdentityEndConflictFinishesWithoutRetry(t *testing.T) {
	endToken := strings.Repeat("e", 32)
	var endCalls atomic.Int32
	var endAuth atomic.Value
	var endBody atomic.Value
	endStatus := atomic.Int32{}
	endStatus.Store(409)
	fakeVault := fakeIdentityEnd(t, &endStatus, &endCalls, &endAuth, &endBody)
	h := setupConfig(t, func(cfg *config.Config) {
		cfg.CoachingTickSecret = "tick-secret"
		cfg.IdentityReaderURL = fakeVault.URL
		cfg.IdentityEndToken = endToken
	})
	applicant := uuid.New()
	applicantToken := token(t, applicant)
	if rec := doJSON(t, h, "POST", "/v1/coach/applications", applicantToken, coachApplicationBody()); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	admin := adminToken(t, h)
	path := "/v1/admin/coach-applications/" + applicant.String() + "/reject"
	if rec := doJSON(t, h, "POST", path, admin, nil); rec.Code != http.StatusOK {
		t.Fatalf("reject %d %s", rec.Code, rec.Body.String())
	}
	retryAllNotificationJobs(t, h)
	if got := endJobCount(t, applicant); got != 0 {
		t.Fatalf("pending end jobs after conflict = %d", got)
	}
	if got := retiredCount(t, applicant); got != 1 {
		t.Fatalf("retired rows changed by conflict = %d", got)
	}
	if got := endCalls.Load(); got != 1 {
		t.Fatalf("vault end calls = %d", got)
	}
}

func TestCoachIdentityEndGuardSkipsReapprovedCoach(t *testing.T) {
	endToken := strings.Repeat("e", 32)
	var endCalls atomic.Int32
	var endAuth atomic.Value
	var endBody atomic.Value
	endStatus := atomic.Int32{}
	endStatus.Store(204)
	fakeVault := fakeIdentityEnd(t, &endStatus, &endCalls, &endAuth, &endBody)
	h := setupConfig(t, func(cfg *config.Config) {
		cfg.CoachingTickSecret = "tick-secret"
		cfg.IdentityReaderURL = fakeVault.URL
		cfg.IdentityEndToken = endToken
	})
	applicant := uuid.New()
	applicantToken := token(t, applicant)
	if rec := doJSON(t, h, "POST", "/v1/coach/applications", applicantToken, coachApplicationBody()); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	admin := adminToken(t, h)
	path := "/v1/admin/coach-applications/" + applicant.String() + "/reject"
	if rec := doJSON(t, h, "POST", path, admin, nil); rec.Code != http.StatusOK {
		t.Fatalf("reject %d %s", rec.Code, rec.Body.String())
	}
	var reviewedAt time.Time
	queryRowArgs(t, "SELECT reviewed_at FROM coach_profiles WHERE profile_id = $1", []any{applicant}, &reviewedAt)
	execSQLArgs(t, "UPDATE coach_profiles SET status = 'approved', reviewed_at = now() WHERE profile_id = $1", applicant)
	execSQLArgs(t, `INSERT INTO notification_jobs (id, kind, recipient_id, reviewed_at) VALUES ($1, 'coach_identity_end', $2, $3)`,
		uuid.New(), applicant, reviewedAt)
	before := endCalls.Load()
	retryAllNotificationJobs(t, h)
	if got := endJobCount(t, applicant); got != 0 {
		t.Fatalf("pending end jobs after reapproval = %d", got)
	}
	if got := endCalls.Load(); got != before {
		t.Fatalf("vault contacted for reapproved coach: calls = %d", got)
	}
	if got := retiredCount(t, applicant); got != 0 {
		t.Fatalf("retired rows for reapproved coach = %d", got)
	}
}

func TestCoachIdentityEndSkippedWhenVaultUnconfigured(t *testing.T) {
	h := setupConfig(t, func(cfg *config.Config) {
		cfg.CoachingTickSecret = "tick-secret"
		cfg.IdentityReaderURL = ""
		cfg.IdentityEndToken = ""
	})
	applicant := uuid.New()
	applicantToken := token(t, applicant)
	if rec := doJSON(t, h, "POST", "/v1/coach/applications", applicantToken, coachApplicationBody()); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	admin := adminToken(t, h)
	path := "/v1/admin/coach-applications/" + applicant.String() + "/reject"
	if rec := doJSON(t, h, "POST", path, admin, nil); rec.Code != http.StatusOK {
		t.Fatalf("reject %d %s", rec.Code, rec.Body.String())
	}
	retryAllNotificationJobs(t, h)
	retryAllNotificationJobs(t, h)
	if got := endJobCount(t, applicant); got != 0 {
		t.Fatalf("pending end jobs without vault = %d", got)
	}
}

func TestCoachDutySaveFailureSendsOnlyConnectionTest(t *testing.T) {
	var pushes atomic.Int32
	configurePush, endpoint := notificationPushRecorder(t, func() { pushes.Add(1) })
	h := setupConfig(t, configurePush)
	coach := uuid.New()
	coachToken := token(t, coach)
	if rec := doJSON(t, h, "POST", "/v1/coach/applications", coachToken, coachApplicationBody()); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	withCoachPhoto(t, coach)
	if rec := doJSON(t, h, "POST", "/v1/admin/coach-applications/"+coach.String()+"/approve", adminToken(t, h), nil); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	subscribeNotificationTest(t, h, coachToken, endpoint)
	remove := injectNotificationFailure(t, "coach_profiles", "UPDATE OF on_duty")
	path := "/v1/coach/me"
	req := map[string]bool{"on_duty": true}
	if rec := doJSON(t, h, "PATCH", path, coachToken, req); rec.Code != http.StatusInternalServerError {
		t.Fatalf("failed save %d %s", rec.Code, rec.Body.String())
	}
	var onDuty bool
	queryRowArgs(t, "SELECT on_duty FROM coach_profiles WHERE profile_id = $1", []any{coach}, &onDuty)
	if onDuty || pushes.Load() != 1 {
		t.Fatalf("after failed save: on duty = %t, pushes = %d", onDuty, pushes.Load())
	}
	remove()
	if rec := doJSON(t, h, "PATCH", path, coachToken, req); rec.Code != http.StatusOK {
		t.Fatalf("successful save %d %s", rec.Code, rec.Body.String())
	}
	if pushes.Load() != 3 {
		t.Fatalf("expected another test and a start push, got total %d", pushes.Load())
	}
}

func TestReviewResubmissionNotificationMatchesPushSettingsAndDestination(t *testing.T) {
	for _, tc := range []struct {
		name                string
		accept, inApp, push bool
		wantBell, wantPush  bool
	}{
		{name: "before_accept_is_required", wantBell: true, wantPush: true},
		{name: "after_accept_both_disabled", accept: true},
		{name: "after_accept_bell_only", accept: true, inApp: true, wantBell: true},
		{name: "after_accept_push_only", accept: true, push: true, wantPush: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fakeCoachingCloudinary(t)
			var pushes atomic.Int32
			configurePush, endpoint := notificationPushRecorder(t, func() { pushes.Add(1) })
			h, _ := setupStripeConfig(t, func(cfg *config.Config) {
				configurePush(cfg)
				cfg.ReviewRandomAssign = false
			})
			admin := adminToken(t, h)
			coach := uuid.New()
			coachToken := token(t, coach)
			if rec := doJSON(t, h, "POST", "/v1/coach/applications", coachToken, coachApplicationBody()); rec.Code != 200 {
				t.Fatal(rec.Body.String())
			}
			withCoachPhoto(t, coach)
			if rec := doJSON(t, h, "POST", "/v1/admin/coach-applications/"+coach.String()+"/approve", admin, nil); rec.Code != 200 {
				t.Fatal(rec.Body.String())
			}
			subscribeNotificationTest(t, h, coachToken, endpoint)
			execSQLArgs(t, "UPDATE coach_profiles SET on_duty = true, stripe_account_id = $2, payouts_enabled = true WHERE profile_id = $1", coach, "acct_"+coach.String())
			_, requester := demoRequester(t, h, admin)
			id := chosenReview(t, h, requester, coach.String(), "card", false)
			payChosenReview(t, h, id, "card")
			kind, url := "review_resubmitted", "/coach"
			if tc.accept {
				offers := coachRequests(t, h, coachToken).Offers
				if len(offers) != 1 {
					t.Fatalf("offers %+v", offers)
				}
				if rec := doJSON(t, h, "POST", "/v1/coach/offers/"+offers[0].ID+"/accept", coachToken, nil); rec.Code != 200 {
					t.Fatal(rec.Body.String())
				}
				kind, url = "review_resubmitted_after_accept", "/coach/requests/"+id
			}
			if rec := doJSON(t, h, "PUT", "/v1/me/notification-settings/review", coachToken, map[string]bool{"in_app": tc.inApp, "push": tc.push}); rec.Code != 200 {
				t.Fatal(rec.Body.String())
			}
			askInfo(t, h, coachToken, id, map[string]any{"body": infoQuestion}, http.StatusOK)
			pushes.Store(0)
			if rec := doJSON(t, h, "POST", "/v1/coaching/requests/"+id+"/resubmit", requester, map[string]string{"question": "スタートから30mまでです"}); rec.Code != 200 {
				t.Fatal(rec.Body.String())
			}
			var count int
			var bellURL string
			queryRowArgs(t, "SELECT count(*), coalesce(max(url), '') FROM notifications WHERE review_request_id = $1 AND recipient_id = $2 AND kind = $3", []any{id, coach, kind}, &count, &bellURL)
			if (count == 1) != tc.wantBell || (tc.wantBell && bellURL != url) {
				t.Errorf("bell count %d, URL %q", count, bellURL)
			}
			if got := pushes.Load(); got > 1 || (got == 1) != tc.wantPush {
				t.Errorf("push count %d", got)
			}
		})
	}
}
