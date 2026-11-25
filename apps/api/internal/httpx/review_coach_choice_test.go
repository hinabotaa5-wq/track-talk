package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/hinata/track-talk/apps/api/internal/config"
)

// 本番と同じく、動画添削のおまかせ(割り当て)を止めたサーバー。選手がコーチを選ぶ方式だけ。
func setupCoachChoice(t *testing.T) (http.Handler, *fakeStripe) {
	t.Helper()
	return setupStripeConfig(t, func(c *config.Config) { c.ReviewRandomAssign = false })
}

// PayPay を選んだときは、この申込みの間だけ PayPay を受け付ける（createReviewWith）。
func chosenReview(t *testing.T, h http.Handler, tu, coach, method string, tickets bool) string {
	t.Helper()
	b := reviewBodyFor(t, h, tu)
	b["method"] = method
	b["use_tickets"] = tickets
	b["requested_coach_id"] = coach
	// 18歳未満の購入のたびの宣言。18歳以上には使われない。
	b["guardian_permission"] = true
	r := createReviewWith(t, h, tu, method, b)
	if r.Code != 201 {
		t.Fatalf("create %d %s", r.Code, r.Body.String())
	}
	var v struct{ ID string }
	_ = json.Unmarshal(r.Body.Bytes(), &v)
	return v.ID
}

func payChosenReview(t *testing.T, h http.Handler, id, method string) {
	t.Helper()
	var session string
	queryRowArgs(t, `SELECT checkout_session_id FROM review_payments WHERE request_id=$1`, []any{id}, &session)
	status := "paid"
	if method == "card" {
		status = "unpaid"
	}
	r := stripeWebhook(t, h, testWebhookSecret, "checkout.session.completed", map[string]any{"id": session, "payment_status": status, "payment_intent": "pi_" + method + "_" + id, "metadata": map[string]any{"kind": "review"}})
	if r.Code != 200 {
		t.Fatal(r.Body.String())
	}
}

// コーチを選んで申し込み、1,000円を払うと、選んだコーチにだけ依頼が届く。返却すると報酬は700円（本体 #569、価格は #983）。
// webhook が2回届いても、依頼・打診・支払いは1つずつ。
func TestReviewCoachChoicePaymentAndReward(t *testing.T) {
	for _, method := range []string{"card", "paypay"} {
		t.Run(method, func(t *testing.T) {
			fakeCoachingCloudinary(t)
			h, f := setupCoachChoice(t)
			admin := adminToken(t, h)
			_, other := onDutyCoach(t, h, admin)
			coach, tc := onDutyCoach(t, h, admin)
			_, tu := demoRequester(t, h, admin)
			id := chosenReview(t, h, tu, coach.String(), method, false)
			form := f.lastSession()
			if form.Get("line_items[0][price_data][unit_amount]") != "1000" || form.Get("line_items[0][price_data][product_data][name]") != "スタンダード" ||
				form.Get("payment_method_types[0]") != method {
				t.Fatalf("session %v", form)
			}
			if len(coachRequests(t, h, tc).Reviewing) != 0 {
				t.Fatal("offered before payment")
			}
			payChosenReview(t, h, id, method)
			payChosenReview(t, h, id, method)
			offers := coachRequests(t, h, tc).Reviewing
			if len(offers) != 1 || offers[0].ID != id || len(coachRequests(t, h, other).Reviewing) != 0 {
				t.Fatalf("wrong assignment %+v", offers)
			}
			var requests, offerRows, payments int
			queryRowArgs(t, `SELECT (SELECT count(*) FROM review_requests), (SELECT count(*) FROM review_offers), (SELECT count(*) FROM review_payments)`, nil, &requests, &offerRows, &payments)
			if requests != 1 || offerRows != 0 || payments != 1 {
				t.Fatalf("duplicates requests=%d offers=%d payments=%d", requests, offerRows, payments)
			}
			list := doJSON(t, h, "GET", "/v1/coach/requests/"+id, tc, nil).Body.String()
			if !strings.Contains(list, `"coach_reward_yen":700`) || !strings.Contains(list, `"label":"報酬","value":"¥700"`) {
				t.Fatalf("offer terms %s", list)
			}
			if r := doJSON(t, h, "POST", "/v1/coach/requests/"+offers[0].ID+"/start", tc, nil); r.Code != 200 {
				t.Fatal(r.Body.String())
			}
			if r := doJSON(t, h, "POST", "/v1/coaching/requests/"+id+"/response", tc, responseBody()); r.Code != 200 {
				t.Fatal(r.Body.String())
			}
			for range 2 {
				if r := tick(t, h, "tick-secret"); r.Code != 200 {
					t.Fatal(r.Body.String())
				}
			}
			e := coachEarnings(t, h, tc)
			if e.UnpaidYen != 700 || len(e.Earnings) != 1 || e.Earnings[0].GrossYen != 1000 || e.Earnings[0].SharePercent != 70 {
				t.Fatalf("reward %+v", e)
			}
			if method == "card" {
				if captured, _ := f.calls(); len(captured) != 1 {
					t.Fatalf("captured %v", captured)
				}
			}
			audit := doJSON(t, h, "GET", "/v1/admin/coaching/reconciliation", admin, nil)
			if audit.Code != 200 || !strings.Contains(audit.Body.String(), `"consumed_ticket_earnings_mismatches":0`) {
				t.Fatalf("reconciliation %s", audit.Body.String())
			}
		})
	}
}

// おまかせを止めている間は、コーチを選ばずに申し込めず、選んでいない依頼が残っていても誰にも割り当てない。
func TestReviewRandomAssignOff(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupCoachChoice(t)
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)
	_, tu := demoRequester(t, h, admin)
	b := reviewBodyFor(t, h, tu)
	b["method"] = "card"
	if r := doJSON(t, h, "POST", "/v1/coaching/requests", tu, b); r.Code != 422 || !strings.Contains(r.Body.String(), "requested_coach_id") {
		t.Fatalf("no coach %d %s", r.Code, r.Body.String())
	}
	// 切り替える前に送られた、コーチを選んでいない依頼。
	id := chosenReview(t, h, tu, coach.String(), "card", false)
	payChosenReview(t, h, id, "card")

	execSQLArgs(t, `UPDATE review_requests SET status='queued', coach_id=NULL, requested_coach_id=NULL, requested_coach_name='' WHERE id=$1`, id)
	if r := tick(t, h, "tick-secret"); r.Code != 200 || !strings.Contains(r.Body.String(), `"offered":0`) {
		t.Fatalf("tick %d %s", r.Code, r.Body.String())
	}
	if len(coachRequests(t, h, tc).Reviewing) != 0 || reviewStatus(t, h, tu, id) != "queued" {
		t.Fatal("random assignment ran")
	}
}

// 選んだコーチが辞退した・引受期限までに返事がないときは、別のコーチに回さずに終わり、代金を戻す。
func TestReviewChosenCoachDeclineAndTimeoutNeverReassign(t *testing.T) {
	for _, action := range []string{"return", "timeout", "cancel"} {
		t.Run(action, func(t *testing.T) {
			fakeCoachingCloudinary(t)
			h, f := setupCoachChoice(t)
			admin := adminToken(t, h)
			coach, tc := onDutyCoach(t, h, admin)
			_, other := onDutyCoach(t, h, admin)
			_, tu := demoRequester(t, h, admin)
			id := chosenReview(t, h, tu, coach.String(), "paypay", false)
			payChosenReview(t, h, id, "paypay")
			offers := coachRequests(t, h, tc).Reviewing
			if len(offers) != 1 {
				t.Fatalf("offers %+v", offers)
			}
			switch action {

			case "return":
				withdrawReview(t, h, tc, id, map[string]any{"reason_code": "video_unviewable"}, http.StatusOK)
			case "timeout":
				execSQLArgs(t, `UPDATE review_requests SET due_at=now()-interval '1 minute' WHERE id=$1`, id)
			case "cancel":
				if r := doJSON(t, h, "POST", "/v1/coaching/requests/"+id+"/cancel", tu, nil); r.Code != 200 {
					t.Fatalf("cancel %d %s", r.Code, r.Body.String())
				}
			}
			execSQLArgs(t, `UPDATE review_payments SET authorized_at=now()-interval '2 minutes' WHERE request_id=$1`, id)
			if r := tick(t, h, "tick-secret"); r.Code != 200 {
				t.Fatal(r.Body.String())
			}
			if status := reviewStatus(t, h, tu, id); status != "expired" {
				t.Fatalf("status %s", status)
			}
			if len(coachRequests(t, h, other).Reviewing) != 0 {
				t.Fatal("reassigned")
			}
			refunds := f.refundList()
			if len(refunds) != 1 || refunds[0].Get("amount") != "1000" {
				t.Fatalf("refunds %v", refunds)
			}
		})
	}
}

// 選べるコーチは受付中で上限に届いていない人だけ。人気の指標は出さず、並びは人気順ではない。
func TestReviewCoachChoiceList(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupCoachChoice(t)
	admin := adminToken(t, h)
	coach, _ := onDutyCoach(t, h, admin)
	busy, _ := onDutyCoach(t, h, admin)
	_, tu := demoRequester(t, h, admin)
	r := doJSON(t, h, "GET", "/v1/coaching/nominees?event=100m", tu, nil)
	if r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	var out struct {
		PriceYen int              `json:"price_yen"`
		Coaches  []map[string]any `json:"coaches"`
	}
	_ = json.Unmarshal(r.Body.Bytes(), &out)
	if out.PriceYen != 1000 || len(out.Coaches) != 2 {
		t.Fatalf("list %s", r.Body.String())
	}
	for _, c := range out.Coaches {
		for _, k := range []string{"rating_count", "rating_average", "review_count", "nomination_fee_yen", "total_price_yen"} {
			if _, ok := c[k]; ok {
				t.Fatalf("%s shown: %v", k, c)
			}
		}
		if _, ok := c["events"]; !ok {
			t.Fatalf("events missing %v", c)
		}
	}
	// 実績(返却した添削)が多いコーチを上にしない。並びは日替わりで、同じ日なら同じ。
	execSQLArgs(t, `UPDATE coach_profiles SET last_offered_at = CASE WHEN profile_id=$1 THEN now() ELSE NULL END WHERE profile_id IN ($1,$2)`, coach, busy)
	var want string
	queryRowArgs(t, `SELECT string_agg(profile_id::text, ',' ORDER BY md5(profile_id::text || extract(epoch FROM date_trunc('day', now() AT TIME ZONE 'Asia/Tokyo') AT TIME ZONE 'Asia/Tokyo')::bigint::text))
		FROM coach_profiles WHERE profile_id IN ($1,$2)`, []any{coach, busy}, &want)
	r = doJSON(t, h, "GET", "/v1/coaching/nominees?event=100m", tu, nil)
	_ = json.Unmarshal(r.Body.Bytes(), &out)
	if got := out.Coaches[0]["id"].(string) + "," + out.Coaches[1]["id"].(string); got != want {
		t.Fatalf("order %s want %s", got, want)
	}
	if again := doJSON(t, h, "GET", "/v1/coaching/nominees?event=100m", tu, nil).Body.String(); again != r.Body.String() {
		t.Fatal("order changed within the day")
	}
	execSQLArgs(t, `UPDATE coach_profiles SET daily_cap=1 WHERE profile_id=$1`, busy)
	id := chosenReview(t, h, tu, busy.String(), "card", false)
	payChosenReview(t, h, id, "card")
	if r := doJSON(t, h, "GET", "/v1/coaching/nominees?event=100m", tu, nil); !strings.Contains(r.Body.String(), busy.String()) {
		t.Fatal("busy coach should remain listed")
	}
	b := reviewBodyFor(t, h, tu)
	b["method"] = "card"
	b["requested_coach_id"] = busy
	if r := doJSON(t, h, "POST", "/v1/coaching/requests", tu, b); r.Code != 201 {
		t.Fatalf("busy coach %d %s", r.Code, r.Body.String())
	}
	execSQLArgs(t, `UPDATE coach_profiles SET on_duty=false WHERE profile_id=$1`, coach)
	if r := doJSON(t, h, "GET", "/v1/coaching/nominees?event=100m", tu, nil); strings.Contains(r.Body.String(), coach.String()) {
		t.Fatal("off-duty coach listed")
	}
}

// 購入済みのチケットで払う場合も、選んだコーチにだけ届く。払わずに放置した依頼はチケットを戻す。
func TestReviewCoachChoiceWithTickets(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupCoachChoice(t)
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)
	_, other := onDutyCoach(t, h, admin)
	_, tu := demoRequester(t, h, admin)
	buySixTickets(t, h, tu)
	id := chosenReview(t, h, tu, coach.String(), "tickets", true)
	offers := coachRequests(t, h, tc).Reviewing
	if len(offers) != 1 || offers[0].ID != id || len(coachRequests(t, h, other).Reviewing) != 0 {
		t.Fatalf("offers %+v", offers)
	}
	if b := ticketSummary(t, h, tu).Balance; b.Available != 5 || b.Reserved != 1 {
		t.Fatalf("balance %+v", b)
	}
}

// 動画添削もサポートプランも、支払い画面は運営者のアカウントで作り（コーチへの分割や手数料の指定はない）、
// コーチへの報酬は月1回の Transfer でまとめて送る。
func TestReviewAndSupportPaidToOperatorAndTransferredMonthly(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupCoachChoice(t)
	admin := adminToken(t, h)
	noSplit := func(form url.Values) {
		t.Helper()
		for k := range form {
			if strings.HasPrefix(k, "payment_intent_data[transfer_data]") || strings.HasPrefix(k, "payment_intent_data[on_behalf_of]") ||
				strings.HasPrefix(k, "payment_intent_data[application_fee_amount]") || strings.HasPrefix(k, "transfer_data") || k == "subscription_data" {
				t.Fatalf("split field %s in %v", k, form)
			}
		}
		if form.Get("mode") != "payment" {
			t.Fatalf("mode %v", form)
		}
	}
	// サポートプラン: 8,000円の期間が終わった。手数料は25%。
	tu, tc, contractID := adultAccepted(t, h)
	var coach string
	queryRowArgs(t, `SELECT coach_id FROM coaching_contracts WHERE id = $1`, []any{contractID}, &coach)
	checkout(t, h, tu, contractID)
	noSplit(f.lastSession())
	paidSession(t, h, sessionIDFor(t, contractID), "pi_support_op", 8000)
	execSQLArgs(t, `UPDATE coaching_contracts SET ends_at = now() - interval '1 minute' WHERE id = $1`, contractID)

	// 同じコーチが動画添削も受ける。
	execSQLArgs(t, `UPDATE coach_profiles SET tier='coach', events=ARRAY['100m'], on_duty=true WHERE profile_id=$1`, coach)
	_, tr := demoRequester(t, h, admin)
	id := chosenReview(t, h, tr, coach, "card", false)
	noSplit(f.lastSession())
	payChosenReview(t, h, id, "card")
	offers := coachRequests(t, h, tc).Reviewing
	if len(offers) != 1 {
		t.Fatalf("offers %+v", offers)
	}
	if r := doJSON(t, h, "POST", "/v1/coach/requests/"+offers[0].ID+"/start", tc, nil); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if r := doJSON(t, h, "POST", "/v1/coaching/requests/"+id+"/response", tc, responseBody()); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if r := tick(t, h, "tick-secret"); r.Code != 200 || !strings.Contains(r.Body.String(), `"earnings_recorded":2`) {
		t.Fatalf("tick %s", r.Body.String())
	}
	if len(f.transferList()) != 0 {
		t.Fatal("transferred per sale")
	}
	execSQLArgs(t, `UPDATE coach_earnings SET confirmed_at = `+lastMonthJST)
	if r := doJSON(t, h, "POST", "/v1/admin/coaching/payouts/run", admin, nil); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	transfers := f.transferList()
	if len(transfers) != 1 || transfers[0].Get("destination") != "acct_"+coach || transfers[0].Get("amount") != "6700" {
		t.Fatalf("transfers %v", transfers)
	}
}

// 本体 #1035：申込で受け付ける種目は 100m／200m／400m。担当種目に200m・400mを含むコーチは
// その種目の候補に出て、依頼が届く。価格は種目によらず同じ。800m などの対象外は従来どおり弾く。
func TestReviewSprintEvents200And400(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupCoachChoice(t)
	admin := adminToken(t, h)
	sprinter, tc := onDutyCoach(t, h, admin)
	short, ts := onDutyCoach(t, h, admin)
	execSQLArgs(t, `UPDATE coach_profiles SET events=ARRAY['100m','200m','400m'] WHERE profile_id=$1`, sprinter)
	execSQLArgs(t, `UPDATE coach_profiles SET events=ARRAY['100m'] WHERE profile_id=$1`, short)
	_, tu := demoRequester(t, h, admin)

	fieldErr := func(label string, r *httptest.ResponseRecorder) string {
		t.Helper()
		var v struct {
			Error struct {
				Code   string            `json:"code"`
				Fields map[string]string `json:"fields"`
			} `json:"error"`
		}
		_ = json.Unmarshal(r.Body.Bytes(), &v)
		if r.Code != http.StatusUnprocessableEntity || v.Error.Code != "validation_error" || v.Error.Fields["event"] == "" {
			t.Fatalf("%s: want event field error, got %d %s", label, r.Code, r.Body.String())
		}
		return v.Error.Fields["event"]
	}

	// A1: nominees。200m・400m では担当するコーチだけが出る。対象外の種目は項目エラー。
	nominees := func(event string) []string {
		t.Helper()
		r := doJSON(t, h, "GET", "/v1/coaching/nominees?event="+event, tu, nil)
		if r.Code != 200 {
			t.Fatalf("nominees %s %d %s", event, r.Code, r.Body.String())
		}
		var out struct {
			Coaches []struct {
				ID string `json:"id"`
			} `json:"coaches"`
		}
		_ = json.Unmarshal(r.Body.Bytes(), &out)
		ids := []string{}
		for _, c := range out.Coaches {
			ids = append(ids, c.ID)
		}
		return ids
	}
	if got := nominees("100m"); len(got) != 2 {
		t.Fatalf("A1 100m %v", got)
	}
	for _, event := range []string{"200m", "400m"} {
		if got := nominees(event); len(got) != 1 || got[0] != sprinter.String() {
			t.Fatalf("A1 %s %v", event, got)
		}
	}
	r := doJSON(t, h, "GET", "/v1/coaching/nominees?event=800m", tu, nil)
	if msg := fieldErr("A1 800m", r); msg != "対応している種目を選んでください" {
		t.Fatalf("A1 800m message %q", msg)
	}

	// A2: estimate。200m・400m も成功し、価格は100mと同じ。800m は項目エラー。
	price := func(event string) int {
		t.Helper()
		r := doJSON(t, h, "GET", "/v1/coaching/estimate?tier=coach&event="+event, tu, nil)
		if r.Code != 200 {
			t.Fatalf("estimate %s %d %s", event, r.Code, r.Body.String())
		}
		var out struct {
			PriceYen int `json:"price_yen"`
		}
		_ = json.Unmarshal(r.Body.Bytes(), &out)
		return out.PriceYen
	}
	base := price("100m")
	if base != 1000 {
		t.Fatalf("A2 100m price %d", base)
	}
	for _, event := range []string{"200m", "400m"} {
		if got := price(event); got != base {
			t.Fatalf("A2 %s price %d want %d", event, got, base)
		}
	}
	r = doJSON(t, h, "GET", "/v1/coaching/estimate?tier=coach&event=800m", tu, nil)
	if msg := fieldErr("A2 800m", r); msg != "枠と種目を選んでください" {
		t.Fatalf("A2 800m message %q", msg)
	}

	// A3: 依頼作成。200m・400m の依頼を作れ、払うと指名したコーチに届き、保存された種目は送った値のまま。
	for _, event := range []string{"200m", "400m"} {
		_, tr := demoRequester(t, h, admin)
		b := reviewBodyFor(t, h, tr)
		b["event"] = event
		b["method"] = "card"
		b["requested_coach_id"] = sprinter.String()
		created := createReviewWith(t, h, tr, "card", b)
		if created.Code != 201 {
			t.Fatalf("A3 %s create %d %s", event, created.Code, created.Body.String())
		}
		var v struct {
			ID    string `json:"id"`
			Event string `json:"event"`
		}
		_ = json.Unmarshal(created.Body.Bytes(), &v)
		if v.Event != event {
			t.Fatalf("A3 %s response event %q", event, v.Event)
		}
		payChosenReview(t, h, v.ID, "card")
		var stored string
		queryRowArgs(t, `SELECT event FROM review_requests WHERE id=$1`, []any{v.ID}, &stored)
		if stored != event {
			t.Fatalf("A3 %s stored %q", event, stored)
		}
		found := false
		for _, o := range coachRequests(t, h, tc).Reviewing {
			found = found || o.ID == v.ID
		}
		if !found {
			t.Fatalf("A3 %s not delivered to the chosen coach", event)
		}
	}
	if len(coachRequests(t, h, ts).Reviewing) != 0 {
		t.Fatal("A3 delivered to a coach who was not chosen")
	}

	// A3: 対象外の 800m は新しい文言の項目エラーで、依頼は作られない。
	var before int
	queryRowArgs(t, `SELECT count(*) FROM review_requests`, nil, &before)
	b := reviewBodyFor(t, h, tu)
	b["event"] = "800m"
	b["method"] = "card"
	b["requested_coach_id"] = sprinter.String()
	r = doJSON(t, h, "POST", "/v1/coaching/requests", tu, b)
	if msg := fieldErr("A3 800m", r); msg != "いまは100m・200m・400mだけ受け付けています" {
		t.Fatalf("A3 800m message %q", msg)
	}
	var after int
	queryRowArgs(t, `SELECT count(*) FROM review_requests`, nil, &after)
	if after != before {
		t.Fatalf("A3 800m created a request: %d -> %d", before, after)
	}
}
