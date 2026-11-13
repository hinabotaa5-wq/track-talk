package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type workOrderTermsView struct {
	Items []struct {
		Label string `json:"label"`
		Value string `json:"value"`
	} `json:"items"`
	AmountYen  int    `json:"amount_yen"`
	DeliverBy  string `json:"deliver_by"`
	PaymentDue string `json:"payment_due"`
}

// フォーム添削を打診するときは、引き受ける前に取引条件(9項目)を見せ、引き受けたら書き留める。
func TestReviewOfferShowsAndRecordsWorkOrderTerms(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripeConfig(t, nil)
	admin := adminToken(t, h)
	_, coach := onDutyCoach(t, h, admin)
	_, owner := demoRequester(t, h, admin)
	buySixTickets(t, h, owner)
	request := ticketReview(t, h, owner)

	rec := doJSON(t, h, http.MethodGet, "/v1/coach/requests/"+request, coach, nil)
	var accepted struct {
		WorkOrder struct {
			TermsVersion int                `json:"terms_version"`
			Terms        workOrderTermsView `json:"terms"`
		} `json:"work_order"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &accepted); err != nil || rec.Code != 200 {
		t.Fatalf("accept %d %s", rec.Code, rec.Body.String())
	}
	if accepted.WorkOrder.TermsVersion != 4 || accepted.WorkOrder.Terms.AmountYen != 350 || len(accepted.WorkOrder.Terms.Items) != 9 {
		t.Fatalf("work order after accept: %s", rec.Body.String())
	}

	terms := accepted.WorkOrder.Terms
	if got := terms.Items[7]; got.Label != "支払期日" || !strings.Contains(got.Value, "累積") || strings.Contains(got.Value, "1回まで") {
		t.Fatalf("payment due terms: %+v", got)
	}
	// 後から依頼の画面で見返せる。
	rec = doJSON(t, h, http.MethodGet, "/v1/coach/requests/"+request, coach, nil)
	var detail struct {
		WorkOrder *struct {
			Terms workOrderTermsView `json:"terms"`
		} `json:"work_order"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil || detail.WorkOrder == nil || detail.WorkOrder.Terms.AmountYen != 350 {
		t.Fatalf("detail %d %s", rec.Code, rec.Body.String())
	}
	assertWorkOrderDelivered(t, h, coach, "review_request_id", request, "/coach/requests/"+request+"#work-order", "動画添削の依頼が届きました")
	if !strings.Contains(rec.Body.String(), `"delivered_at"`) {
		t.Fatalf("detail has no delivered_at: %s", rec.Body.String())
	}

	// 交付済みの旧版は、現在の文面から作り直さず、保存時の版と内容を返す。
	execSQLArgs(t, `UPDATE coach_work_orders SET terms_version=1, terms=jsonb_set(terms, '{items,7,value}', to_jsonb('旧版の支払期日'::text)) WHERE review_request_id=$1`, request)
	rec = doJSON(t, h, http.MethodGet, "/v1/coach/requests/"+request, coach, nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &accepted); err != nil || rec.Code != 200 || accepted.WorkOrder.TermsVersion != 1 || accepted.WorkOrder.Terms.Items[7].Value != "旧版の支払期日" {
		t.Fatalf("stored version 1 must remain unchanged: %d %s", rec.Code, rec.Body.String())
	}
}

// assertWorkOrderDelivered は、取引条件がコーチのお知らせに宛先付きで1回だけ届き、届けた記録が残ったことを確かめる。
// お知らせの文面は「届いた」の1行だけで本文は空。9項目の全文は開く先の依頼の画面に出す（本体 #609）。
func assertWorkOrderDelivered(t *testing.T, h http.Handler, coach, column, id, url, wantTitle string) {
	t.Helper()
	var count int
	var title, body, gotURL, channel string
	queryRowArgs(t, `SELECT count(*), max(d.title), max(d.body), max(d.url), max(d.channel)
		FROM coach_work_order_deliveries d JOIN coach_work_orders w ON w.id = d.work_order_id
		WHERE w.`+column+`=$1 AND d.recipient_id = w.coach_id AND d.terms_version = w.terms_version`, []any{id}, &count, &title, &body, &gotURL, &channel)
	if count != 1 || gotURL != url || channel != "in_app" || title != wantTitle || body != "" {
		t.Fatalf("delivery %d %q %q %q %q", count, title, body, gotURL, channel)
	}
	items, _ := listNotifications(t, h, coach)
	found := 0
	for _, n := range items {
		if n.URL == url && n.Title == title && n.Body == body {
			found++
		}
	}
	if found != 1 {
		t.Fatalf("notification for %s: %d in %+v", url, found, items)
	}
}

// フォーム添削の追加質問も、引き受ける前に取引条件を見せ、引き受けたら書き留める。
func TestReviewConsultationShowsAndRecordsWorkOrderTerms(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	owner, coach, root := answeredTicketRoom(t, h)
	path := "/v1/coaching/requests/" + root + "/consultations"
	c := consultationDraft(t, h, owner, path, uuid.NewString(), "次の大会ではどこを意識しますか")
	consultationAction(t, h, owner, c.ID, "submit", nil)
	if code := payConsultation(t, h, c.ID, 500); code != 200 {
		t.Fatalf("pay %d", code)
	}

	room := func() map[string]json.RawMessage {
		rec := doJSON(t, h, http.MethodGet, path, coach, nil)
		var out struct {
			Consultations []map[string]json.RawMessage `json:"consultations"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || len(out.Consultations) != 1 {
			t.Fatalf("room %d %s", rec.Code, rec.Body.String())
		}
		return out.Consultations[0]
	}
	var terms workOrderTermsView
	if err := json.Unmarshal(room()["terms"], &terms); err != nil || terms.AmountYen != 350 || len(terms.Items) != 9 {
		t.Fatalf("terms before accept: %+v %v", terms, err)
	}
	consultationAction(t, h, coach, c.ID, "accept", nil)
	var wo struct {
		Terms workOrderTermsView `json:"terms"`
	}
	if err := json.Unmarshal(room()["work_order"], &wo); err != nil || wo.Terms.AmountYen != 350 {
		t.Fatalf("work order after accept: %+v %v", wo, err)
	}
	var count int
	queryRowArgs(t, `SELECT count(*) FROM coach_work_orders WHERE consultation_id=$1`, []any{c.ID}, &count)
	if count != 1 {
		t.Fatalf("work orders %d", count)
	}
	assertWorkOrderDelivered(t, h, coach, "consultation_id", c.ID, "/coach/requests/"+root+"#work-order", "追加質問が届きました")
	if !strings.Contains(string(room()["work_order"]), `"delivered_at"`) {
		t.Fatalf("room work order has no delivered_at")
	}
}

// お知らせ(ベル)を切っているコーチにも、取引条件の通知は届ける(法律上の明示なので設定に従わない)。
func TestWorkOrderDeliveredEvenWhenNotificationsOff(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripeConfig(t, nil)
	admin := adminToken(t, h)
	_, coach := onDutyCoach(t, h, admin)
	_, owner := demoRequester(t, h, admin)
	for _, category := range []string{"review", "coaching"} {
		if rec := doJSON(t, h, http.MethodPut, "/v1/me/notification-settings/"+category, coach, map[string]any{"in_app": false, "push": false}); rec.Code != 200 && rec.Code != 204 {
			t.Fatalf("setting %s %d %s", category, rec.Code, rec.Body.String())
		}
	}
	buySixTickets(t, h, owner)
	request := ticketReview(t, h, owner)
	assertWorkOrderDelivered(t, h, coach, "review_request_id", request, "/coach/requests/"+request+"#work-order", "動画添削の依頼が届きました")
}
