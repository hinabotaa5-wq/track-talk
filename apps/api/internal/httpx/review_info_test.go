package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/config"
)

// 返却前の「内容確認のための差し戻し」（Issue #319）。

const infoQuestion = "動画のどの場面（スタートか中盤か後半か）を見てほしいのかが分かりません。気になっている区間を教えてください。"

type infoRequestView struct {
	Body             *string    `json:"body"`
	Reason           *string    `json:"reason"`
	FromStatus       *string    `json:"from_status"`
	ResubmitBy       *time.Time `json:"resubmit_by"`
	OriginalQuestion *string    `json:"original_question"`
	VideoReplaced    bool       `json:"video_replaced"`
	ResubmittedAt    *time.Time `json:"resubmitted_at"`
}

type infoReviewView struct {
	ID          string           `json:"id"`
	Status      string           `json:"status"`
	Question    string           `json:"question"`
	DueAt       time.Time        `json:"due_at"`
	VideoURL    string           `json:"video_url"`
	InfoRequest *infoRequestView `json:"info_request"`
}

func requesterReviewView(t *testing.T, h http.Handler, tu, id string) infoReviewView {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/coaching/requests/"+id, tu, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get %d %s", rec.Code, rec.Body.String())
	}
	var out infoReviewView
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}

func askInfo(t *testing.T, h http.Handler, tc, id string, body map[string]any, want int) infoReviewView {
	t.Helper()
	rec := doJSON(t, h, http.MethodPost, "/v1/coach/requests/"+id+"/info-request", tc, body)
	if rec.Code != want {
		t.Fatalf("info request want %d got %d %s", want, rec.Code, rec.Body.String())
	}
	var out infoReviewView
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}

func within(t time.Time, want time.Time, slack time.Duration) bool {
	return !t.Before(want.Add(-slack)) && !t.After(want.Add(slack))
}

// 引受後に差し戻すと、再提出まで支払いはそのまま。元の返却期限を過ぎていても、再提出から24時間で添削に戻れる。
func TestReviewInfoRequestAfterAccept(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupCoachChoice(t)
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)
	_, tother := onDutyCoach(t, h, admin)
	_, tu := demoRequester(t, h, admin)
	id := chosenReview(t, h, tu, coach.String(), "card", false)
	payChosenReview(t, h, id, "card")
	offers := coachRequests(t, h, tc).Offers
	if r := doJSON(t, h, "POST", "/v1/coach/offers/"+offers[0].ID+"/accept", tc, nil); r.Code != 200 {
		t.Fatal(r.Body.String())
	}

	// 担当でないコーチ・依頼者は差し戻せない。
	askInfo(t, h, tother, id, map[string]any{"body": infoQuestion}, http.StatusNotFound)
	askInfo(t, h, tu, id, map[string]any{"body": infoQuestion}, http.StatusForbidden)
	// 「意味が分からない」だけでは差し戻せない。種類は決まったものだけ。
	for _, body := range []map[string]any{{"body": ""}, {"body": "意味が分からない"}, {"body": "？？？？？？？？？？？？？？？？？？"}, {"body": infoQuestion, "reason": "bad"}} {
		askInfo(t, h, tc, id, body, http.StatusUnprocessableEntity)
	}
	before := time.Now()
	asked := askInfo(t, h, tc, id, map[string]any{"body": infoQuestion, "reason": "scene"}, http.StatusOK)
	if asked.Status != "needs_info" || asked.VideoURL == "" || asked.InfoRequest == nil || *asked.InfoRequest.FromStatus != "in_review" {
		t.Fatalf("asked %+v", asked)
	}
	// 依頼者には、コーチの質問と再提出期限（差し戻しから24時間）が見える。
	view := requesterReviewView(t, h, tu, id)
	if view.Status != "needs_info" || view.InfoRequest == nil || *view.InfoRequest.Body != infoQuestion || *view.InfoRequest.Reason != "scene" ||
		!within(*view.InfoRequest.ResubmitBy, before.Add(24*time.Hour), time.Minute) {
		t.Fatalf("requester view %+v", view)
	}
	var kind string
	queryRowArgs(t, `SELECT kind FROM notifications WHERE review_request_id=$1 ORDER BY created_at DESC LIMIT 1`, []any{id}, &kind)
	if kind != "review_info_request" {
		t.Fatalf("notification %s", kind)
	}
	// 差し戻し中は返金も仮押さえの取り消しも起きない。返却期限を過ぎても期限切れにしない。
	execSQLArgs(t, `UPDATE review_payments SET authorized_at = now() - interval '2 minutes' WHERE request_id = $1`, id)
	execSQLArgs(t, `UPDATE review_requests SET due_at = now() - interval '1 hour' WHERE id = $1`, id)
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if _, canceled := f.calls(); len(canceled) != 0 || len(f.refundList()) != 0 || reviewPaymentStatus(t, id) != "authorized" {
		t.Fatalf("payment touched while waiting: canceled=%v refunds=%v status=%s", canceled, f.refundList(), reviewPaymentStatus(t, id))
	}
	if reviewStatus(t, h, tu, id) != "needs_info" {
		t.Fatal("needs_info should survive the old due date")
	}
	// 差し戻し中は返却できず、2回目の差し戻しもできない。
	if r := doJSON(t, h, "POST", "/v1/coaching/requests/"+id+"/response", tc, responseBody()); r.Code != http.StatusConflict {
		t.Fatalf("response while needs_info %d %s", r.Code, r.Body.String())
	}
	askInfo(t, h, tc, id, map[string]any{"body": infoQuestion}, http.StatusConflict)

	// 再提出は依頼者本人だけ。相談内容を直さずには出せない。
	if r := doJSON(t, h, "POST", "/v1/coaching/requests/"+id+"/resubmit", tother, map[string]any{"question": "後半です"}); r.Code != http.StatusNotFound {
		t.Fatalf("other resubmit %d", r.Code)
	}
	if r := doJSON(t, h, "POST", "/v1/coaching/requests/"+id+"/resubmit", tu, map[string]any{"question": "  "}); r.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty resubmit %d", r.Code)
	}
	before = time.Now()
	rec := doJSON(t, h, "POST", "/v1/coaching/requests/"+id+"/resubmit", tu, map[string]any{"question": "60m以降の後半です。失速しないためのポイントを教えてください。"})
	if rec.Code != http.StatusOK {
		t.Fatalf("resubmit %d %s", rec.Code, rec.Body.String())
	}
	var resubmitted infoReviewView
	_ = json.Unmarshal(rec.Body.Bytes(), &resubmitted)
	// 元の期限を過ぎていても、返却期限は再提出から24時間。引き受けたまま添削に戻る。
	if resubmitted.Status != "in_review" || !within(resubmitted.DueAt, before.Add(24*time.Hour), time.Minute) ||
		resubmitted.InfoRequest == nil || resubmitted.InfoRequest.OriginalQuestion == nil || resubmitted.InfoRequest.ResubmittedAt == nil || resubmitted.InfoRequest.VideoReplaced {
		t.Fatalf("resubmitted %+v", resubmitted)
	}
	queryRowArgs(t, `SELECT kind FROM notifications WHERE review_request_id=$1 ORDER BY created_at DESC LIMIT 1`, []any{id}, &kind)
	if kind != "review_resubmitted_after_accept" {
		t.Fatalf("coach notification %s", kind)
	}
	if r := doJSON(t, h, "POST", "/v1/coaching/requests/"+id+"/resubmit", tu, map[string]any{"question": "もう一度"}); r.Code != http.StatusConflict {
		t.Fatalf("second resubmit %d", r.Code)
	}
	// 再提出のあとも差し戻しは2回目を使えない。
	askInfo(t, h, tc, id, map[string]any{"body": infoQuestion}, http.StatusConflict)
	if r := doJSON(t, h, "POST", "/v1/coaching/requests/"+id+"/response", tc, responseBody()); r.Code != 200 {
		t.Fatalf("response after resubmit %d %s", r.Code, r.Body.String())
	}
	if captured, _ := f.calls(); len(captured) != 1 {
		t.Fatalf("captured %v", captured)
	}
}

// 引受前に差し戻すと、打診は再提出まで止まり、再提出で引受期限90分を数え直す。動画も差し替えられる。
func TestReviewInfoRequestBeforeAccept(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupCoachChoice(t)
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)
	_, tu := demoRequester(t, h, admin)
	id := chosenReview(t, h, tu, coach.String(), "paypay", false)
	payChosenReview(t, h, id, "paypay")
	if len(coachRequests(t, h, tc).Offers) != 1 {
		t.Fatal("no offer")
	}
	asked := askInfo(t, h, tc, id, map[string]any{"body": infoQuestion}, http.StatusOK)
	if asked.Status != "needs_info" || asked.VideoURL != "" || *asked.InfoRequest.FromStatus != "offered" {
		t.Fatalf("asked %+v", asked)
	}
	// 差し戻し中の打診は一覧に出さず、引き受け・見送りもできない。添削中の欄に動画なしで出る。
	list := coachRequests(t, h, tc)
	if len(list.Offers) != 0 || len(list.Reviewing) != 1 || list.Reviewing[0].Status != "needs_info" || list.Reviewing[0].VideoURL != "" {
		t.Fatalf("coach list %+v", list)
	}
	var offerID string
	queryRowArgs(t, `SELECT id::text FROM review_offers WHERE request_id=$1`, []any{id}, &offerID)
	if r := doJSON(t, h, "POST", "/v1/coach/offers/"+offerID+"/accept", tc, nil); r.Code != http.StatusConflict {
		t.Fatalf("accept while needs_info %d %s", r.Code, r.Body.String())
	}
	if r := doJSON(t, h, "POST", "/v1/coach/offers/"+offerID+"/decline", tc, nil); r.Code != http.StatusConflict {
		t.Fatalf("decline while needs_info %d %s", r.Code, r.Body.String())
	}
	// 引受期限（90分）を過ぎても、差し戻し中は打診が切れない。
	execSQLArgs(t, `UPDATE review_offers SET offered_at = now() - interval '3 hours' WHERE request_id=$1`, id)
	execSQLArgs(t, `UPDATE review_payments SET authorized_at = now() - interval '2 minutes' WHERE request_id = $1`, id)
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if reviewStatus(t, h, tu, id) != "needs_info" || len(f.refundList()) != 0 || reviewPaymentStatus(t, id) != "captured" {
		t.Fatalf("ended while waiting: refunds=%v", f.refundList())
	}

	// 動画を差し替えて再提出すると、もう一度同じコーチに打診され、引受期限を新しく数える。
	video := signedCoachingVideo(t, h, tu)
	rec := doJSON(t, h, "POST", "/v1/coaching/requests/"+id+"/resubmit", tu, map[string]any{"question": "スタートから30mまでです", "video_public_id": video})
	if rec.Code != http.StatusOK {
		t.Fatalf("resubmit %d %s", rec.Code, rec.Body.String())
	}
	var resubmitted infoReviewView
	_ = json.Unmarshal(rec.Body.Bytes(), &resubmitted)
	if resubmitted.Status != "offered" || !resubmitted.InfoRequest.VideoReplaced || !strings.Contains(resubmitted.VideoURL, strings.TrimPrefix(video, "track-talk-coaching/")) {
		t.Fatalf("resubmitted %+v", resubmitted)
	}
	var fresh bool
	queryRowArgs(t, `SELECT o.expires_at > now() + interval '89 minutes' AND o.expires_at <= r.due_at FROM review_offers o JOIN review_requests r ON r.id=o.request_id WHERE o.request_id=$1 AND o.result IS NULL`, []any{id}, &fresh)
	if !fresh {
		t.Fatal("offer window was not restarted")
	}
	offers := coachRequests(t, h, tc).Offers
	if len(offers) != 1 || offers[0].Request.ID != id {
		t.Fatalf("offers %+v", offers)
	}
	if r := doJSON(t, h, "POST", "/v1/coach/offers/"+offers[0].ID+"/accept", tc, nil); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if v := requesterReviewView(t, h, tu, id); v.Status != "in_review" || v.Question != "スタートから30mまでです" {
		t.Fatalf("after accept %+v", v)
	}
}

// 再提出期限までに再提出がなければ、依頼は終わり、既存の期限切れと同じく取り消し・返金する。
func TestReviewInfoRequestExpiresWithoutResubmit(t *testing.T) {
	for _, tc := range []struct{ method, origin string }{{"card", "offered"}, {"paypay", "in_review"}} {
		t.Run(tc.method+"_"+tc.origin, func(t *testing.T) {
			fakeCoachingCloudinary(t)
			h, f := setupCoachChoice(t)
			admin := adminToken(t, h)
			coach, tcoach := onDutyCoach(t, h, admin)
			_, tu := demoRequester(t, h, admin)
			id := chosenReview(t, h, tu, coach.String(), tc.method, false)
			payChosenReview(t, h, id, tc.method)
			if tc.origin == "in_review" {
				offers := coachRequests(t, h, tcoach).Offers
				if r := doJSON(t, h, "POST", "/v1/coach/offers/"+offers[0].ID+"/accept", tcoach, nil); r.Code != 200 {
					t.Fatal(r.Body.String())
				}
			}
			askInfo(t, h, tcoach, id, map[string]any{"body": infoQuestion}, http.StatusOK)
			execSQLArgs(t, `UPDATE review_requests SET resubmit_by = now() - interval '1 minute' WHERE id = $1`, id)
			execSQLArgs(t, `UPDATE review_payments SET authorized_at = now() - interval '2 minutes' WHERE request_id = $1`, id)
			// 期限を過ぎてからの再提出は受け付けない。
			if r := doJSON(t, h, "POST", "/v1/coaching/requests/"+id+"/resubmit", tu, map[string]any{"question": "後半です"}); r.Code != http.StatusConflict {
				t.Fatalf("late resubmit %d", r.Code)
			}
			if r := tick(t, h, "tick-secret"); r.Code != 200 || !strings.Contains(r.Body.String(), `"unresubmitted":1`) || !strings.Contains(r.Body.String(), `"review_payments_released":1`) {
				t.Fatalf("tick %d %s", r.Code, r.Body.String())
			}
			if reviewStatus(t, h, tu, id) != "expired" {
				t.Fatal("not expired")
			}
			var open int
			queryRowArgs(t, `SELECT count(*) FROM review_offers WHERE request_id=$1 AND result IS NULL`, []any{id}, &open)
			if open != 0 {
				t.Fatal("offer left open")
			}
			_, canceled := f.calls()
			switch tc.method {
			case "card":
				if len(canceled) != 1 || reviewPaymentStatus(t, id) != "canceled" {
					t.Fatalf("card canceled=%v status=%s", canceled, reviewPaymentStatus(t, id))
				}
			case "paypay":
				if refunds := f.refundList(); len(refunds) != 1 || refunds[0].Get("amount") != "1500" || reviewPaymentStatus(t, id) != "refunded" {
					t.Fatalf("paypay refunds=%v status=%s", refunds, reviewPaymentStatus(t, id))
				}
			}
			// 別のコーチには回さない。
			var offersCount int
			queryRowArgs(t, `SELECT count(*) FROM review_offers WHERE request_id=$1`, []any{id}, &offersCount)
			if offersCount != 1 {
				t.Fatalf("reassigned: %d offers", offersCount)
			}
		})
	}
}

// 差し戻しはコーチから依頼者への質問なので、メッセージ機能を閉じている間は使えない。
func TestReviewInfoRequestNeedsMessaging(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setupConfig(t, func(c *config.Config) { c.CoachingMessaging = false })
	admin := adminToken(t, h)
	_, tc := onDutyCoach(t, h, admin)
	_, tu := demoRequester(t, h, admin)
	id := acceptedReview(t, h, admin, tc, tu)
	if r := doJSON(t, h, "POST", "/v1/coach/requests/"+id+"/info-request", tc, map[string]any{"body": infoQuestion}); r.Code != http.StatusConflict || !strings.Contains(r.Body.String(), "messaging_closed") {
		t.Fatalf("%d %s", r.Code, r.Body.String())
	}
}
