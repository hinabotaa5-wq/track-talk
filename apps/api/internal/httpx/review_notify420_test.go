package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/hinata/track-talk/apps/api/internal/jst"
)

// countableAhead は、from からカウント可能な時間（23時〜翌7時を除く）で d だけ後の時刻。
// リマインドのテストで、壁時計によらず残り時間を固定するために使う。
func countableAhead(from time.Time, d time.Duration) time.Time {
	t := from.In(jst.Zone)
	remaining := d
	for remaining > 0 {
		day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, jst.Zone)
		quietStart := day.Add(23 * time.Hour)
		quietEnd := day.Add(7 * time.Hour)
		switch {
		case t.Before(quietEnd):
			t = quietEnd
		case !t.Before(quietStart):
			t = quietEnd.AddDate(0, 0, 1)
		default:
			if seg := quietStart.Sub(t); seg >= remaining {
				return t.Add(remaining)
			} else {
				remaining -= seg
				t = quietEnd.AddDate(0, 0, 1)
			}
		}
	}
	return t
}

func notificationKinds(t *testing.T, h http.Handler, tok string) string {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/me/notifications", tok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("notifications %d %s", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

func countKind(body, kind string) int {
	return strings.Count(body, `"kind":"`+kind+`"`)
}

// 引受が決まったら依頼者に通知が届き、タップ先は依頼の画面。review カテゴリの設定に従う。
func TestReviewAcceptNotifiesRequester(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setup(t)
	tadmin := adminToken(t, h)
	_, tc := onDutyCoach(t, h, tadmin)
	_, tu := demoRequester(t, h, tadmin)
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, reviewBodyFor(t, h, tu))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct{ ID string }
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	offers := coachRequests(t, h, tc).Offers
	if len(offers) != 1 {
		t.Fatalf("offers %+v", offers)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/offers/"+offers[0].ID+"/accept", tc, nil); rec.Code != http.StatusOK {
		t.Fatalf("accept %d %s", rec.Code, rec.Body.String())
	}
	body := notificationKinds(t, h, tu)
	if countKind(body, "review_accepted") != 1 || !strings.Contains(body, `"review_request_id":"`+created.ID+`"`) {
		t.Fatalf("requester notifications %s", body)
	}
}

func TestReviewAcceptRespectsReviewCategory(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setup(t)
	tadmin := adminToken(t, h)
	_, tc := onDutyCoach(t, h, tadmin)
	_, tu := demoRequester(t, h, tadmin)
	if rec := doJSON(t, h, http.MethodPut, "/v1/me/notification-settings/review", tu,
		map[string]any{"in_app": false, "push": false}); rec.Code != http.StatusOK {
		t.Fatalf("settings %d %s", rec.Code, rec.Body.String())
	}
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, reviewBodyFor(t, h, tu))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	offers := coachRequests(t, h, tc).Offers
	if len(offers) != 1 {
		t.Fatalf("offers %+v", offers)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/offers/"+offers[0].ID+"/accept", tc, nil); rec.Code != http.StatusOK {
		t.Fatalf("accept %d %s", rec.Code, rec.Body.String())
	}
	if body := notificationKinds(t, h, tu); countKind(body, "review_accepted") != 0 {
		t.Fatalf("category off but notified %s", body)
	}
}

// 指名の打診が残り30分（カウント可能な時間）になったら、コーチにリマインドが1回だけ届く。
func TestReviewReminderOncePerOffer(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripeConfig(t, func(c *config.Config) {
		c.ReviewRandomAssign = false
		c.CoachingTickSecret = "tick-secret"
	})
	tadmin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, tadmin)
	_, tu := demoRequester(t, h, tadmin)
	id := chosenReview(t, h, tu, coach.String(), "card", false)
	payChosenReview(t, h, id, "card")

	expiry := countableAhead(time.Now(), 20*time.Minute)
	execSQLArgs(t, `UPDATE review_offers SET expires_at=$1 WHERE request_id=$2`, expiry, uuid.MustParse(id))
	rec := tick(t, h, "tick-secret")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"reminded":1`) {
		t.Fatalf("tick %d %s", rec.Code, rec.Body.String())
	}
	body := notificationKinds(t, h, tc)
	if countKind(body, "review_offer_reminder") != 1 || !strings.Contains(body, `"review_request_id":"`+id+`"`) {
		t.Fatalf("coach notifications %s", body)
	}
	rec = tick(t, h, "tick-secret")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"reminded":0`) {
		t.Fatalf("second tick %d %s", rec.Code, rec.Body.String())
	}
	if body := notificationKinds(t, h, tc); countKind(body, "review_offer_reminder") != 1 {
		t.Fatalf("reminded twice %s", body)
	}
}

// 指名の打診が時間切れで終わったら、依頼者に加えて指名コーチにも期限切れが届く。
// コーチ自身が見送った場合は、コーチへの通知は増えない。
func TestReviewNominationTimeoutNotifiesCoach(t *testing.T) {
	for _, action := range []string{"timeout", "decline"} {
		t.Run(action, func(t *testing.T) {
			fakeCoachingCloudinary(t)
			h, _ := setupStripeConfig(t, func(c *config.Config) {
				c.ReviewRandomAssign = false
				c.CoachingTickSecret = "tick-secret"
			})
			tadmin := adminToken(t, h)
			coach, tc := onDutyCoach(t, h, tadmin)
			_, tu := demoRequester(t, h, tadmin)
			id := chosenReview(t, h, tu, coach.String(), "card", false)
			payChosenReview(t, h, id, "card")
			offers := coachRequests(t, h, tc).Offers
			if len(offers) != 1 {
				t.Fatalf("offers %+v", offers)
			}
			if action == "decline" {
				if rec := doJSON(t, h, http.MethodPost, "/v1/coach/offers/"+offers[0].ID+"/decline", tc, nil); rec.Code != http.StatusNoContent {
					t.Fatalf("decline %d %s", rec.Code, rec.Body.String())
				}
			} else {
				execSQLArgs(t, `UPDATE review_offers SET expires_at=now()-interval '1 minute' WHERE request_id=$1`, id)
			}
			execSQLArgs(t, `UPDATE review_payments SET authorized_at=now()-interval '2 minutes' WHERE request_id=$1`, id)
			if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK {
				t.Fatalf("tick %d %s", rec.Code, rec.Body.String())
			}
			if got := reviewStatus(t, h, tu, id); got != "expired" {
				t.Fatalf("status %s", got)
			}
			if body := notificationKinds(t, h, tu); countKind(body, "review_expired") != 1 {
				t.Fatalf("requester %s", body)
			}
			// コーチ側の期限切れは、既存の添削中・差し戻し終了と同じく message 種別（行き先つき）で届く。
			wantCoach := 0
			if action == "timeout" {
				wantCoach = 1
			}
			if body := notificationKinds(t, h, tc); strings.Count(body, `"title":"打診中の依頼が終わりました"`) != wantCoach {
				t.Fatalf("%s coach %s", action, body)
			}
		})
	}
}

// 打診中に返却の目安を過ぎたら、返事を待っていたコーチにも期限切れが届く。
func TestReviewOverdueOfferedNotifiesCoach(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setupConfig(t, func(c *config.Config) { c.CoachingTickSecret = "tick-secret" })
	tadmin := adminToken(t, h)
	_, tc := onDutyCoach(t, h, tadmin)
	_, tu := demoRequester(t, h, tadmin)
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, reviewBodyFor(t, h, tu))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct{ ID string }
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	execSQLArgs(t, `UPDATE review_requests SET due_at=now()-interval '1 minute' WHERE id=$1`, created.ID)
	if got := reviewStatus(t, h, tu, created.ID); got != "expired" {
		t.Fatalf("status %s", got)
	}
	if body := notificationKinds(t, h, tu); countKind(body, "review_expired") != 1 {
		t.Fatalf("requester %s", body)
	}
	// コーチ側の期限切れは、既存の添削中の期限切れと同じく message 種別（行き先つき）で届く。
	if body := notificationKinds(t, h, tc); strings.Count(body, `"title":"打診中の依頼が期限を過ぎました"`) != 1 {
		t.Fatalf("coach %s", body)
	}
}
