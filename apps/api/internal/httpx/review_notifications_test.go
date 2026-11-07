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
	reviewSetting := func(tok string, enabled bool) {
		t.Helper()
		rec := doJSON(t, h, http.MethodPut, "/v1/me/notification-settings/review", tok, map[string]bool{"in_app": enabled, "push": enabled})
		if rec.Code != http.StatusOK {
			t.Fatalf("setting: %d", rec.Code)
		}
	}

	id := start()
	reviewSetting(tc, false)
	beforeCoach := coachPushes.Load()
	// The offer is still open with at most 30 countable minutes remaining.
	execSQLArgs(t, "UPDATE review_offers SET expires_at=$2 WHERE request_id=$1", id, time.Now().Add(20*time.Minute))
	runTick()
	runTick()
	assertNotice(id, "review_offer_reminder", coach, 1)
	assertNotice(id, "review_offer_reminder", requester, 0)
	if got := coachPushes.Load() - beforeCoach; got != 1 {
		t.Fatalf("reminder push attempts: %d", got)
	}
	beforeRequester := requesterPushes.Load()
	acceptChosen(t, h, tc, id)
	assertNotice(id, "review_accepted", requester, 1)
	assertNotice(id, "review_accepted", coach, 0)
	if got := requesterPushes.Load() - beforeRequester; got != 1 {
		t.Fatalf("acceptance push attempts: %d", got)
	}

	reviewSetting(tu, false)
	silent := start()
	offers := coachRequests(t, h, tc).Offers
	if len(offers) != 1 || offers[0].Request.ID != silent {
		t.Fatalf("expected one open offer, got %+v", offers)
	}
	beforeRequester = requesterPushes.Load()
	acceptChosen(t, h, tc, silent)
	assertNotice(silent, "review_accepted", requester, 0)
	if requesterPushes.Load() != beforeRequester {
		t.Fatal("acceptance push bypassed requester preferences")
	}
	// Free a slot for the remaining offers.
	execSQLArgs(t, "UPDATE review_requests SET status='expired' WHERE id=$1", silent)
	reviewSetting(tu, true)
	reviewSetting(tc, true)

	expired := start()
	beforeCoach = coachPushes.Load()
	beforeRequester = requesterPushes.Load()
	execSQLArgs(t, "UPDATE review_offers SET expires_at=now() - interval '1 second' WHERE request_id=$1", expired)
	runTick()
	runTick()
	assertNotice(expired, "review_expired", requester, 1)
	assertNotice(expired, "review_nomination_ended", coach, 1)
	assertNotice(expired, "review_offer_reminder", coach, 0)
	if coachPushes.Load()-beforeCoach != 1 || requesterPushes.Load()-beforeRequester != 1 {
		t.Fatal("expected one expiry push to each recipient")
	}

	declined := start()
	offers = coachRequests(t, h, tc).Offers
	if len(offers) != 1 || offers[0].Request.ID != declined {
		t.Fatalf("expected one open offer, got %+v", offers)
	}
	beforeCoach = coachPushes.Load()
	rec := doJSON(t, h, http.MethodPost, "/v1/coach/offers/"+offers[0].ID+"/decline", tc, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("decline: %d", rec.Code)
	}
	assertNotice(declined, "review_expired", requester, 1)
	assertNotice(declined, "review_nomination_ended", coach, 0)
	if coachPushes.Load() != beforeCoach {
		t.Fatal("coach was notified of their own decline")
	}
}
