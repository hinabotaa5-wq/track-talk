package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

func TestNominatedReviewNotifications(t *testing.T) {
	fakeCoachingCloudinary(t)
	var coachPushes, requesterPushes atomic.Int32
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/coach":
			coachPushes.Add(1)
		case "/requester":
			requesterPushes.Add(1)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(provider.Close)
	private, public, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	h, _ := setupStripeConfig(t, func(cfg *config.Config) {
		cfg.ReviewRandomAssign = false
		cfg.VAPIDPublicKey = public
		cfg.VAPIDPrivateKey = private
		cfg.VAPIDSubject = "mailto:test@example.com"
		cfg.PushHTTP = provider.Client()
	})
	admin := adminToken(t, h)
	coach, tc := approvedReviewCoach(t, h, admin)
	requester, tu := demoRequester(t, h, admin)
	subscribe := func(tok, route string) {
		t.Helper()
		p256dh, auth := testPushKeys(t)
		rec := doJSON(t, h, http.MethodPost, "/v1/me/push-subscriptions", tok, map[string]any{
			"endpoint": provider.URL + route, "keys": map[string]string{"p256dh": p256dh, "auth": auth},
		})
		if rec.Code != http.StatusNoContent {
			t.Fatalf("subscribe: %d", rec.Code)
		}
	}
	subscribe(tc, "/coach")
	subscribe(tu, "/requester")
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me", tc, map[string]any{"on_duty": true}); rec.Code != http.StatusOK {
		t.Fatalf("on duty: %d %s", rec.Code, rec.Body)
	}
	start := func() string {
		t.Helper()
		id := chosenReview(t, h, tu, coach.String(), "card", false)
		payChosenReview(t, h, id, "card")
		return id
	}
	assertNotice := func(id, kind string, to uuid.UUID, want int) {
		t.Helper()
		var count int
		queryRowArgs(t, "SELECT count(*) FROM notifications WHERE review_request_id=$1 AND kind=$2 AND recipient_id=$3", []any{id, kind, to}, &count)
		if count != want {
			t.Fatalf("%s notices: got %d, want %d", kind, count, want)
		}
	}
	runTick := func() {
		t.Helper()
		if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK {
			t.Fatalf("tick: %d %s", rec.Code, rec.Body)
		}
	}

	id := start()
	assertNotice(id, "review_offer", coach, 1)
	before := coachPushes.Load()
	for _, lead := range []time.Duration{11 * time.Hour, 3 * time.Hour} {
		execSQLArgs(t, "UPDATE review_requests SET due_at=$2 WHERE id=$1", id, time.Now().Add(lead))
		runTick()
		runTick()
	}
	assertNotice(id, "review_offer_reminder", coach, 2)
	if coachPushes.Load()-before != 2 {
		t.Fatal("expected one push per reminder")
	}
	execSQLArgs(t, "UPDATE review_requests SET due_at=now()-interval '1 second' WHERE id=$1", id)
	beforeRequester := requesterPushes.Load()
	before = coachPushes.Load()
	runTick()
	runTick()
	assertNotice(id, "review_expired", requester, 1)
	assertNotice(id, "review_accepted", requester, 0)
	if coachPushes.Load()-before != 1 || requesterPushes.Load()-beforeRequester != 1 {
		t.Fatal("expected one expiry push to each recipient")
	}
	var onDuty bool
	queryRowArgs(t, "SELECT on_duty FROM coach_profiles WHERE profile_id=$1", []any{coach}, &onDuty)
	if onDuty {
		t.Fatal("coach still accepting new reviews after timeout")
	}
}
