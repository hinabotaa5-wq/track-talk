package httpx_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

// arrivalBellCounts は、割当後のコーチ宛ての到着系ベルの内訳を返す。
// workOrderは取引条件側（URL が #work-order で終わる message）、offer は到着通知側（kind review_offer）。
func arrivalBellCounts(t *testing.T, h http.Handler, coachToken string, coach uuid.UUID, requestID string) (workOrder, offer int) {
	t.Helper()
	queryRowArgs(t, `SELECT count(*) FROM notifications WHERE review_request_id=$1 AND kind='review_offer' AND recipient_id=$2`,
		[]any{requestID, coach}, &offer)
	items := coachingNotifications(t, h, coachToken)
	for _, n := range items {
		if n.Title == "スタンダードプランの依頼が届きました" {
			workOrder++
			if !strings.HasSuffix(n.URL, "#work-order") {
				t.Fatalf("arrival bell URL %q does not end with #work-order", n.URL)
			}
		}
	}
	return workOrder, offer
}

func arrivalPushCount(t *testing.T, coach uuid.UUID, requestID string) int {
	t.Helper()
	var n int
	queryRowArgs(t, `SELECT count(*) FROM push_outbox WHERE profile_id=$1 AND dedup_key=$2`,
		[]any{coach, "review_assigned:" + requestID}, &n)
	return n
}

func setupArrivalPush(t *testing.T) (configure func(*config.Config), endpoint string) {
	t.Helper()
	return notificationPushRecorder(t, func() {})
}

// TestReviewSingleArrivalCard は本体 #745 の項目1〜3：カード決済の完了で割り当てたとき、
// コーチのベルは取引条件側の1件だけで、kind review_offer のベルは0件、
// プッシュ outbox は DedupKey review_assigned:{依頼ID} で1件、交付記録も従来どおり残る。
func TestReviewSingleArrivalCard(t *testing.T) {
	fakeCoachingCloudinary(t)
	configurePush, endpoint := setupArrivalPush(t)
	h, _ := setupStripeConfig(t, func(cfg *config.Config) {
		configurePush(cfg)
		cfg.ReviewRandomAssign = false
	})
	admin := adminToken(t, h)
	coach, coachToken := onDutyCoachTo(t, h, admin, endpoint)
	_, requester := demoRequester(t, h, admin)

	id := chosenReview(t, h, requester, coach.String(), "card", false)
	payChosenReview(t, h, id, "card")

	if got := coachRequests(t, h, coachToken).Reviewing; len(got) != 1 || got[0].ID != id {
		t.Fatalf("not assigned: %+v", got)
	}
	workOrder, offer := arrivalBellCounts(t, h, coachToken, coach, id)
	if workOrder != 1 {
		t.Fatalf("work-order bells = %d, want 1", workOrder)
	}
	if offer != 0 {
		t.Fatalf("review_offer bells = %d, want 0", offer)
	}
	if n := arrivalPushCount(t, coach, id); n != 1 {
		t.Fatalf("arrival push rows = %d, want 1", n)
	}
	assertWorkOrderDelivered(t, h, coachToken, "review_request_id", id, "/coach/requests/"+id+"#work-order", "スタンダードプランの依頼が届きました")
}

// TestReviewSingleArrivalTicket は本体 #745 の項目4：チケット利用の申込でも項目1と同じく
// ベルは取引条件側の1件だけで、kind review_offer のベルは0件。
func TestReviewSingleArrivalTicket(t *testing.T) {
	fakeCoachingCloudinary(t)
	configurePush, endpoint := setupArrivalPush(t)
	// チケット申込はコーチ無指定のため、ランダム割当は既定のまま（受付中は1名だけ）。
	h, _ := setupStripeConfig(t, func(cfg *config.Config) {
		configurePush(cfg)
	})
	admin := adminToken(t, h)
	coach, coachToken := onDutyCoachTo(t, h, admin, endpoint)
	_, owner := demoRequester(t, h, admin)
	buySixTickets(t, h, owner)

	id := ticketReview(t, h, owner)
	if got := coachRequests(t, h, coachToken).Reviewing; len(got) != 1 || got[0].ID != id {
		t.Fatalf("not assigned: %+v", got)
	}
	workOrder, offer := arrivalBellCounts(t, h, coachToken, coach, id)
	if workOrder != 1 {
		t.Fatalf("work-order bells = %d, want 1", workOrder)
	}
	if offer != 0 {
		t.Fatalf("review_offer bells = %d, want 0", offer)
	}
	if n := arrivalPushCount(t, coach, id); n != 1 {
		t.Fatalf("arrival push rows = %d, want 1", n)
	}
	assertWorkOrderDelivered(t, h, coachToken, "review_request_id", id, "/coach/requests/"+id+"#work-order", "スタンダードプランの依頼が届きました")
}

// TestReviewSingleArrivalRequeue は本体 #745 の項目5：待機キューからの再割当（offerReview 経由）でも
// ベルは取引条件側の1件のままで、kind review_offer のベルは0件、プッシュも増えない。
func TestReviewSingleArrivalRequeue(t *testing.T) {
	fakeCoachingCloudinary(t)
	configurePush, endpoint := setupArrivalPush(t)
	h, _ := setupStripeConfig(t, func(cfg *config.Config) {
		configurePush(cfg)
		cfg.ReviewRandomAssign = false
	})
	admin := adminToken(t, h)
	coach, coachToken := onDutyCoachTo(t, h, admin, endpoint)
	_, requester := demoRequester(t, h, admin)

	id := chosenReview(t, h, requester, coach.String(), "card", false)
	payChosenReview(t, h, id, "card")

	// 割り当て済みを待機に戻し、tick の再割当に回す（指名は残す）。
	execSQLArgs(t, `UPDATE review_requests SET status='queued', coach_id=NULL WHERE id=$1`, id)
	if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK {
		t.Fatalf("tick: %d %s", rec.Code, rec.Body.String())
	}
	if got := coachRequests(t, h, coachToken).Reviewing; len(got) != 1 || got[0].ID != id {
		t.Fatalf("not re-offered: %+v", got)
	}
	workOrder, offer := arrivalBellCounts(t, h, coachToken, coach, id)
	if workOrder != 1 {
		t.Fatalf("work-order bells after requeue = %d, want 1", workOrder)
	}
	if offer != 0 {
		t.Fatalf("review_offer bells after requeue = %d, want 0", offer)
	}
	if n := arrivalPushCount(t, coach, id); n != 1 {
		t.Fatalf("arrival push rows after requeue = %d, want 1", n)
	}
}

// TestReviewSingleArrivalNotificationsOff は本体 #745 の項目6（フォローアップの項目1）：
// コーチがレビュー通知をオフにしていても、取引条件のベルは1件届き、交付記録も残る。
// 到着プッシュは積まない（2026-11-15 運営者判断）。
func TestReviewSingleArrivalNotificationsOff(t *testing.T) {
	fakeCoachingCloudinary(t)
	configurePush, endpoint := setupArrivalPush(t)
	h, _ := setupStripeConfig(t, func(cfg *config.Config) {
		configurePush(cfg)
		cfg.ReviewRandomAssign = false
	})
	admin := adminToken(t, h)
	coach, coachToken := onDutyCoachTo(t, h, admin, endpoint)
	if rec := doJSON(t, h, http.MethodPut, "/v1/me/notification-settings/review", coachToken, map[string]bool{"in_app": false, "push": false}); rec.Code != 200 {
		t.Fatalf("setting %d %s", rec.Code, rec.Body.String())
	}
	_, requester := demoRequester(t, h, admin)

	id := chosenReview(t, h, requester, coach.String(), "card", false)
	payChosenReview(t, h, id, "card")

	workOrder, offer := arrivalBellCounts(t, h, coachToken, coach, id)
	if workOrder != 1 {
		t.Fatalf("work-order bells when off = %d, want 1", workOrder)
	}
	if offer != 0 {
		t.Fatalf("review_offer bells when off = %d, want 0", offer)
	}
	if n := arrivalPushCount(t, coach, id); n != 0 {
		t.Fatalf("arrival push rows when off = %d, want 0", n)
	}
	assertWorkOrderDelivered(t, h, coachToken, "review_request_id", id, "/coach/requests/"+id+"#work-order", "スタンダードプランの依頼が届きました")
}

// TestReviewSingleArrivalTicketNotificationsOff はフォローアップの項目2：通知オフ＋
// チケット利用の割当でも、ベルは取引条件側の1件だけでプッシュは積まない。
func TestReviewSingleArrivalTicketNotificationsOff(t *testing.T) {
	fakeCoachingCloudinary(t)
	configurePush, endpoint := setupArrivalPush(t)
	// チケット申込はコーチ無指定のため、ランダム割当は既定のまま（受付中は1名だけ）。
	h, _ := setupStripeConfig(t, func(cfg *config.Config) {
		configurePush(cfg)
	})
	admin := adminToken(t, h)
	coach, coachToken := onDutyCoachTo(t, h, admin, endpoint)
	if rec := doJSON(t, h, http.MethodPut, "/v1/me/notification-settings/review", coachToken, map[string]bool{"in_app": false, "push": false}); rec.Code != 200 {
		t.Fatalf("setting %d %s", rec.Code, rec.Body.String())
	}
	_, owner := demoRequester(t, h, admin)
	buySixTickets(t, h, owner)

	id := ticketReview(t, h, owner)
	if got := coachRequests(t, h, coachToken).Reviewing; len(got) != 1 || got[0].ID != id {
		t.Fatalf("not assigned: %+v", got)
	}
	workOrder, offer := arrivalBellCounts(t, h, coachToken, coach, id)
	if workOrder != 1 {
		t.Fatalf("work-order bells when off = %d, want 1", workOrder)
	}
	if offer != 0 {
		t.Fatalf("review_offer bells when off = %d, want 0", offer)
	}
	if n := arrivalPushCount(t, coach, id); n != 0 {
		t.Fatalf("arrival push rows when off = %d, want 0", n)
	}
	assertWorkOrderDelivered(t, h, coachToken, "review_request_id", id, "/coach/requests/"+id+"#work-order", "スタンダードプランの依頼が届きました")
}

// TestReviewSingleArrivalRequeueNotificationsOff はフォローアップの項目3：通知オフ＋
// 待機キューからの再割当でも、ベルは取引条件側の1件のままでプッシュは新たに積まない。
func TestReviewSingleArrivalRequeueNotificationsOff(t *testing.T) {
	fakeCoachingCloudinary(t)
	configurePush, endpoint := setupArrivalPush(t)
	h, _ := setupStripeConfig(t, func(cfg *config.Config) {
		configurePush(cfg)
		cfg.ReviewRandomAssign = false
	})
	admin := adminToken(t, h)
	coach, coachToken := onDutyCoachTo(t, h, admin, endpoint)
	if rec := doJSON(t, h, http.MethodPut, "/v1/me/notification-settings/review", coachToken, map[string]bool{"in_app": false, "push": false}); rec.Code != 200 {
		t.Fatalf("setting %d %s", rec.Code, rec.Body.String())
	}
	_, requester := demoRequester(t, h, admin)

	id := chosenReview(t, h, requester, coach.String(), "card", false)
	payChosenReview(t, h, id, "card")

	// 割り当て済みを待機に戻し、tick の再割当に回す（指名は残す）。
	execSQLArgs(t, `UPDATE review_requests SET status='queued', coach_id=NULL WHERE id=$1`, id)
	if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK {
		t.Fatalf("tick: %d %s", rec.Code, rec.Body.String())
	}
	if got := coachRequests(t, h, coachToken).Reviewing; len(got) != 1 || got[0].ID != id {
		t.Fatalf("not re-offered: %+v", got)
	}
	workOrder, offer := arrivalBellCounts(t, h, coachToken, coach, id)
	if workOrder != 1 {
		t.Fatalf("work-order bells after requeue when off = %d, want 1", workOrder)
	}
	if offer != 0 {
		t.Fatalf("review_offer bells after requeue when off = %d, want 0", offer)
	}
	if n := arrivalPushCount(t, coach, id); n != 0 {
		t.Fatalf("arrival push rows after requeue when off = %d, want 0", n)
	}
}
