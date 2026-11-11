package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/hinata/track-talk/apps/api/internal/config"
)

// 引受後・返却前のコーチ都合の辞退（Issue #323）。

type withdrawalView struct {
	FromStatus  *string `json:"from_status"`
	Reason      *string `json:"reason"`
	WithdrawnAt *string `json:"withdrawn_at"`
}

type withdrawnReviewView struct {
	ID         string          `json:"id"`
	Status     string          `json:"status"`
	Withdrawal *withdrawalView `json:"withdrawal"`
}

func withdrawReview(t *testing.T, h http.Handler, tc, id string, body map[string]any, want int) withdrawnReviewView {
	t.Helper()
	rec := doJSON(t, h, http.MethodPost, "/v1/coach/requests/"+id+"/withdraw", tc, body)
	if rec.Code != want {
		t.Fatalf("withdraw want %d got %d %s", want, rec.Code, rec.Body.String())
	}
	var out withdrawnReviewView
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}

func requesterWithdrawal(t *testing.T, h http.Handler, tu, id string) withdrawnReviewView {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/coaching/requests/"+id, tu, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get %d %s", rec.Code, rec.Body.String())
	}
	var out withdrawnReviewView
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}

func acceptChosen(t *testing.T, h http.Handler, tc, id string) {
	t.Helper()
	offers := coachRequests(t, h, tc).Offers
	if len(offers) != 1 || offers[0].Request.ID != id {
		t.Fatalf("offers %+v", offers)
	}
	if r := doJSON(t, h, http.MethodPost, "/v1/coach/offers/"+offers[0].ID+"/accept", tc, nil); r.Code != http.StatusOK {
		t.Fatalf("accept %d %s", r.Code, r.Body.String())
	}
}

// assertReleased は、辞退した依頼の支払いが既存の期限切れと同じ経路で取り消し・返金され、別のコーチに回っていないことを確かめる。
func assertReleased(t *testing.T, h http.Handler, f *fakeStripe, id, method string) {
	t.Helper()
	execSQLArgs(t, `UPDATE review_payments SET authorized_at = now() - interval '2 minutes' WHERE request_id = $1`, id)
	if r := tick(t, h, "tick-secret"); r.Code != 200 || !strings.Contains(r.Body.String(), `"review_payments_released":1`) {
		t.Fatalf("tick %d %s", r.Code, r.Body.String())
	}
	captured, canceled := f.calls()
	if len(captured) != 0 {
		t.Fatalf("captured %v", captured)
	}
	switch method {
	case "card":
		if len(canceled) != 1 || reviewPaymentStatus(t, id) != "canceled" {
			t.Fatalf("card canceled=%v status=%s", canceled, reviewPaymentStatus(t, id))
		}
	case "paypay":
		if refunds := f.refundList(); len(refunds) != 1 || refunds[0].Get("amount") != "500" || reviewPaymentStatus(t, id) != "refunded" {
			t.Fatalf("paypay refunds=%v status=%s", refunds, reviewPaymentStatus(t, id))
		}
	}
	var offers int
	queryRowArgs(t, `SELECT count(*) FROM review_offers WHERE request_id=$1`, []any{id}, &offers)
	if offers != 1 {
		t.Fatalf("reassigned: %d offers", offers)
	}
}

// 添削中に辞退すると、その場で終わり、依頼者に知らせ、コーチの枠が空く。支払いは既存の期限切れと同じく取り消し・返金する。
func TestReviewWithdrawInReview(t *testing.T) {
	for _, method := range []string{"card", "paypay"} {
		t.Run(method, func(t *testing.T) {
			fakeCoachingCloudinary(t)
			h, f := setupCoachChoice(t)
			admin := adminToken(t, h)
			coach, tc := onDutyCoach(t, h, admin)
			_, tother := onDutyCoach(t, h, admin)
			_, tu := demoRequester(t, h, admin)
			id := chosenReview(t, h, tu, coach.String(), method, false)
			payChosenReview(t, h, id, method)
			acceptChosen(t, h, tc, id)

			// 担当でないコーチ・依頼者は辞退できない。
			withdrawReview(t, h, tother, id, nil, http.StatusNotFound)
			withdrawReview(t, h, tu, id, nil, http.StatusForbidden)
			// 理由は任意。長すぎるもの・連絡先らしきものは書けない。
			withdrawReview(t, h, tc, id, map[string]any{"reason": strings.Repeat("あ", 201)}, http.StatusUnprocessableEntity)
			withdrawReview(t, h, tc, id, map[string]any{"reason": "詳しくは coach@example.com まで"}, http.StatusUnprocessableEntity)
			if reviewStatus(t, h, tu, id) != "in_review" {
				t.Fatal("rejected withdraw changed the request")
			}

			out := withdrawReview(t, h, tc, id, map[string]any{"reason": "  体調を崩してしまい、期限内に見られなくなりました  "}, http.StatusOK)
			if out.Status != "expired" || out.Withdrawal == nil || *out.Withdrawal.FromStatus != "in_review" {
				t.Fatalf("withdrawn %+v", out)
			}
			// 依頼者には、旧期限を待たずにすぐ終わったことと、コーチの理由が見える。
			view := requesterWithdrawal(t, h, tu, id)
			if view.Status != "expired" || view.Withdrawal == nil || view.Withdrawal.Reason == nil ||
				*view.Withdrawal.Reason != "体調を崩してしまい、期限内に見られなくなりました" {
				t.Fatalf("requester view %+v", view)
			}
			var kind string
			queryRowArgs(t, `SELECT kind FROM notifications WHERE review_request_id=$1 ORDER BY created_at DESC LIMIT 1`, []any{id}, &kind)
			if kind != "review_withdrawn" {
				t.Fatalf("notification %s", kind)
			}
			// 運営者が後から見られるよう、誰が・いつ・どの状態から辞退したかを残す。
			var by, from string
			var at bool
			queryRowArgs(t, `SELECT withdrawn_by::text, withdrawn_from_status, withdrawn_at IS NOT NULL FROM review_requests WHERE id=$1`, []any{id}, &by, &from, &at)
			if by != coach.String() || from != "in_review" || !at {
				t.Fatalf("record by=%s from=%s at=%v", by, from, at)
			}
			// 同時に持てる本数から外れ、添削中の一覧からも消える。
			var active int
			queryRowArgs(t, `SELECT count(*) FROM review_requests WHERE coach_id=$1 AND status IN ('offered','in_review','needs_info')`, []any{coach}, &active)
			if active != 0 || len(coachRequests(t, h, tc).Reviewing) != 0 {
				t.Fatalf("still holding: active=%d", active)
			}
			// 二重の辞退・辞退後の返却はきれいに断る。
			withdrawReview(t, h, tc, id, nil, http.StatusConflict)
			if r := doJSON(t, h, "POST", "/v1/coaching/requests/"+id+"/response", tc, responseBody()); r.Code == http.StatusOK {
				t.Fatalf("response after withdraw %d", r.Code)
			}
			assertReleased(t, h, f, id, method)
		})
	}
}

// 引受後の差し戻し中に辞退しても同じく終わり、再提出はできなくなる。
func TestReviewWithdrawNeedsInfo(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupCoachChoice(t)
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)
	_, tu := demoRequester(t, h, admin)
	id := chosenReview(t, h, tu, coach.String(), "paypay", false)
	payChosenReview(t, h, id, "paypay")
	acceptChosen(t, h, tc, id)
	askInfo(t, h, tc, id, map[string]any{"body": infoQuestion}, http.StatusOK)

	out := withdrawReview(t, h, tc, id, map[string]any{}, http.StatusOK)
	if out.Status != "expired" || out.Withdrawal == nil || *out.Withdrawal.FromStatus != "needs_info" || out.Withdrawal.Reason != nil {
		t.Fatalf("withdrawn %+v", out)
	}
	if r := doJSON(t, h, "POST", "/v1/coaching/requests/"+id+"/resubmit", tu, map[string]any{"question": "60m以降の後半を見てほしいです"}); r.Code != http.StatusConflict {
		t.Fatalf("resubmit after withdraw %d %s", r.Code, r.Body.String())
	}
	if len(coachRequests(t, h, tc).Reviewing) != 0 {
		t.Fatal("still listed")
	}
	assertReleased(t, h, f, id, "paypay")
	// 再提出期限の処理が、辞退した依頼に重ねて通知しない。
	if r := tick(t, h, "tick-secret"); r.Code != 200 || !strings.Contains(r.Body.String(), `"unresubmitted":0`) {
		t.Fatalf("tick %d %s", r.Code, r.Body.String())
	}
}

// 辞退できるのは引き受けたあと・返却前だけ。引受前（打診中・引受前の差し戻し中）、返却済み、終了済みは断る。
func TestReviewWithdrawRejectedStates(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupCoachChoice(t)
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)
	_, tu := demoRequester(t, h, admin)

	// 打診中（引受前）は見送りで断る。
	offered := chosenReview(t, h, tu, coach.String(), "card", false)
	payChosenReview(t, h, offered, "card")
	withdrawReview(t, h, tc, offered, nil, http.StatusConflict)
	// 引受前の差し戻し中も引き受けていないので辞退ではない。
	askInfo(t, h, tc, offered, map[string]any{"body": infoQuestion}, http.StatusOK)
	withdrawReview(t, h, tc, offered, nil, http.StatusConflict)
	if reviewStatus(t, h, tu, offered) != "needs_info" {
		t.Fatal("pre-accept needs_info changed")
	}

	// 返却済みは辞退できない。
	_, tu2 := demoRequester(t, h, admin)
	returned := chosenReview(t, h, tu2, coach.String(), "card", false)
	payChosenReview(t, h, returned, "card")
	acceptChosen(t, h, tc, returned)
	if r := doJSON(t, h, "POST", "/v1/coaching/requests/"+returned+"/response", tc, responseBody()); r.Code != http.StatusOK {
		t.Fatalf("response %d %s", r.Code, r.Body.String())
	}
	withdrawReview(t, h, tc, returned, nil, http.StatusConflict)
	if reviewStatus(t, h, tu2, returned) != "returned" {
		t.Fatal("returned changed")
	}

	// 返却期限を過ぎて終わった依頼も辞退できない（辞退の記録も付かない）。
	_, tu3 := demoRequester(t, h, admin)
	overdue := chosenReview(t, h, tu3, coach.String(), "card", false)
	payChosenReview(t, h, overdue, "card")
	acceptChosen(t, h, tc, overdue)
	execSQLArgs(t, `UPDATE review_requests SET due_at = now() - interval '1 minute' WHERE id = $1`, overdue)
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	withdrawReview(t, h, tc, overdue, nil, http.StatusConflict)
	var withdrawn bool
	queryRowArgs(t, `SELECT withdrawn_at IS NOT NULL FROM review_requests WHERE id=$1`, []any{overdue}, &withdrawn)
	if withdrawn {
		t.Fatal("overdue request marked withdrawn")
	}
}

// 辞退は依頼の状態を変えるだけでメッセージではないので、メッセージ機能を閉じていても使える。
func TestReviewWithdrawWithoutMessaging(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setupConfig(t, func(c *config.Config) { c.CoachingMessaging = false })
	admin := adminToken(t, h)
	_, tc := onDutyCoach(t, h, admin)
	_, tu := demoRequester(t, h, admin)
	id := acceptedReview(t, h, admin, tc, tu)
	if out := withdrawReview(t, h, tc, id, nil, http.StatusOK); out.Status != "expired" {
		t.Fatalf("withdrawn %+v", out)
	}
	if reviewStatus(t, h, tu, id) != "expired" {
		t.Fatal("not ended")
	}
}
