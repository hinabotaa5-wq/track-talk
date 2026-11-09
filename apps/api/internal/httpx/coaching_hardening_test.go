package httpx_test

// 動画添削の申込みまわりで、今は正しく動いている挙動（Checkout の作り方・認可・未成年の宣言・通知）を
// 将来の変更で壊さないための回帰テスト（本体 #474）。Stripe・Push はテスト用のサーバーで代わりに受ける。

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/hinata/track-talk/apps/api/internal/stripe"
)

// sessionsSince は、n 番目より後に作られた支払い画面の中身と、そのときの Stripe-Version。
func sessionsSince(f *fakeStripe, n int) ([]map[string]string, []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var forms []map[string]string
	for _, s := range f.sessions[n:] {
		forms = append(forms, map[string]string{
			"managed":    s.Get("managed_payments[enabled]"),
			"method":     s.Get("payment_method_types[0]"),
			"methods":    strings.Join(s["payment_method_types[1]"], ","),
			"kind":       s.Get("metadata[kind]"),
			"capture":    s.Get("payment_intent_data[capture_method]"),
			"amount":     s.Get("line_items[0][price_data][unit_amount]"),
			"cancel_url": s.Get("cancel_url"),
		})
	}
	return forms, append([]string(nil), f.sessionVersions[n:]...)
}

// answerOffer は、コーチが届いた打診を引き受けて返却するところまで進める。
func answerOffer(t *testing.T, h http.Handler, tc, root string) {
	t.Helper()
	for _, o := range coachRequests(t, h, tc).Offers {
		if r := doJSON(t, h, "POST", "/v1/coach/offers/"+o.ID+"/accept", tc, nil); r.Code != 200 {
			t.Fatalf("accept %d %s", r.Code, r.Body.String())
		}
	}
	if r := doJSON(t, h, "POST", "/v1/coaching/requests/"+root+"/response", tc, responseBody()); r.Code != 200 {
		t.Fatalf("response %d %s", r.Code, r.Body.String())
	}
}

// 項目1：CreateCheckoutSession を通る動画添削の4経路（依頼の作成・支払い画面の開き直し・
// 追加相談の送信・追加相談の支払い画面の開き直し）のすべてで、managed_payments[enabled]=false と
// カードだけの payment_method_types を毎回送る。STRIPE_API_VERSION を Dahlia にピン留めしたときも、
// 未設定（アカウントの既定）のときも、Dahlia 世代の Stripe-Version が付き、payment_method_types で 400 にならない。
func TestEveryReviewCheckoutPathDisablesManagedPayments(t *testing.T) {
	for _, tc := range []struct{ name, version string }{
		{"pinned dahlia", "2026-08-26.dahlia"},
		{"unset", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fakeCoachingCloudinary(t)
			h, f := setupStripeConfig(t, func(c *config.Config) {
				c.ReviewRandomAssign = false
				c.StripeAPIVersion = tc.version
			})
			admin := adminToken(t, h)
			coach, tcoach := onDutyCoach(t, h, admin)
			_, tu := demoRequester(t, h, admin)
			base := sessionCount(f)

			// 1. 依頼の作成。
			root := chosenReview(t, h, tu, coach.String(), "card", false)
			// 2. 支払い画面の開き直し（前の画面の期限が近いと作り直す）。
			execSQLArgs(t, `UPDATE review_payments SET session_expires_at = now() WHERE request_id = $1`, root)
			if r := doJSON(t, h, "POST", "/v1/coaching/requests/"+root+"/checkout", tu, map[string]any{"method": "card"}); r.Code != 200 {
				t.Fatalf("reopen %d %s", r.Code, r.Body.String())
			}
			payChosenReview(t, h, root, "card")
			answerOffer(t, h, tcoach, root)

			// 3. 追加相談の送信。
			c := consultationDraft(t, h, tu, "/v1/coaching/requests/"+root+"/consultations", uuid.NewString(), "次の大会ではどこを意識しますか")
			if r := doJSON(t, h, "POST", "/v1/coaching/consultations/"+c.ID+"/submit", tu, nil); r.Code != 200 || !strings.Contains(r.Body.String(), "checkout_url") {
				t.Fatalf("submit %d %s", r.Code, r.Body.String())
			}
			// 4. 追加相談の支払い画面の開き直し（前の画面が残っていないとき＝送信時に作れなかったときに作る）。
			execSQLArgs(t, `UPDATE coaching_consultations SET checkout_url = NULL, checkout_session_id = NULL WHERE id = $1`, c.ID)
			if r := doJSON(t, h, "POST", "/v1/coaching/consultations/"+c.ID+"/checkout", tu, nil); r.Code != 200 || !strings.Contains(r.Body.String(), "checkout_url") {
				t.Fatalf("consultation reopen %d %s", r.Code, r.Body.String())
			}

			forms, versions := sessionsSince(f, base)
			if len(forms) != 4 {
				t.Fatalf("sessions %d, want 4: %+v", len(forms), forms)
			}
			wantKinds := []string{"review", "review", "consultation", "consultation"}
			for i, form := range forms {
				if form["managed"] != "false" || form["method"] != "card" || form["methods"] != "" || form["kind"] != wantKinds[i] {
					t.Fatalf("session %d %+v", i, form)
				}
				if versions[i] != stripe.CheckoutFallbackAPIVersion {
					t.Fatalf("session %d Stripe-Version %q", i, versions[i])
				}
			}
			// 動画添削は仮押さえ（返却時に確定）、追加相談は ¥600。
			if forms[0]["capture"] != "manual" || forms[1]["capture"] != "manual" || forms[0]["amount"] != "1500" || forms[2]["amount"] != "600" {
				t.Fatalf("sessions %+v", forms)
			}
		})
	}
}

// 項目2：未認証では、申込みまわりの API はどれも 401 で、申込みは作られない。
func TestCoachingEndpointsRejectAnonymous(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupStripeConfig(t, func(c *config.Config) { c.ReviewRandomAssign = false })
	admin := adminToken(t, h)
	coach, _ := onDutyCoach(t, h, admin)
	_, tu := demoRequester(t, h, admin)
	root := chosenReview(t, h, tu, coach.String(), "card", false)

	var before int
	queryRowArgs(t, `SELECT count(*) FROM review_requests`, nil, &before)
	sessions := sessionCount(f)
	for _, c := range []struct {
		method, path string
		body         any
	}{
		{"GET", "/v1/me", nil},
		{"GET", "/v1/coach/me", nil},
		{"GET", "/v1/me/coaching", nil},
		{"GET", "/v1/coaching/nominees?event=100m", nil},
		{"POST", "/v1/coaching/requests", map[string]any{}},
		{"POST", "/v1/coaching/requests", map[string]any{"method": "card", "requested_coach_id": coach.String(), "event": "100m"}},
		{"GET", "/v1/coaching/requests/" + root, nil},
		{"POST", "/v1/coaching/requests/" + root + "/checkout", map[string]any{"method": "card"}},
		{"GET", "/v1/coaching/requests/" + root + "/consultations", nil},
		{"POST", "/v1/coaching/requests/" + root + "/consultations", map[string]any{"request_key": uuid.NewString(), "body": "x"}},
		{"POST", "/v1/coaching/consultations/" + uuid.NewString() + "/submit", nil},
		{"POST", "/v1/coaching/consultations/" + uuid.NewString() + "/checkout", nil},
		{"GET", "/v1/admin/coaching/payments", nil},
		{"POST", "/v1/admin/coaching/payments/" + uuid.NewString() + "/refund", map[string]any{"source": "review", "reason": "x"}},
	} {
		for _, tok := range []string{"", "not-a-jwt"} {
			if r := doJSON(t, h, c.method, c.path, tok, c.body); r.Code != http.StatusUnauthorized {
				t.Fatalf("%s %s token=%q: %d %s", c.method, c.path, tok, r.Code, r.Body.String())
			}
		}
	}
	var after int
	queryRowArgs(t, `SELECT count(*) FROM review_requests`, nil, &after)
	if after != before || sessionCount(f) != sessions {
		t.Fatalf("anonymous calls created requests=%d sessions=%d", after-before, sessionCount(f)-sessions)
	}
}

// 項目2：コーチ一覧のフラグを閉じている間は、未認証でも認証済みでも /v1/coaches は 404。
func TestCoachListClosedIsNotFoundForEveryone(t *testing.T) {
	h, _ := setupStripeConfig(t, func(c *config.Config) {
		c.ReviewRandomAssign = false
		c.CoachingCoachList = false
	})
	_, tu := demoRequester(t, h, adminToken(t, h))
	for _, tok := range []string{"", tu} {
		if r := doJSON(t, h, http.MethodGet, "/v1/coaches", tok, nil); r.Code != http.StatusNotFound {
			t.Fatalf("coaches token=%v %d %s", tok != "", r.Code, r.Body.String())
		}
	}
}

// 項目2：ログインしていても、他人の依頼・追加相談・決済は ID を指定しても見られず、操作もできない（404 か 403）。
// 拒まれた操作では、支払い画面も宣言も作られず、状態も変わらない。
func TestOthersReviewConsultationAndPaymentAreOffLimits(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupStripeConfig(t, func(c *config.Config) { c.ReviewRandomAssign = false })
	admin := adminToken(t, h)
	coach, tcoach := onDutyCoach(t, h, admin)
	_, owner := demoRequester(t, h, admin)
	_, other := demoRequester(t, h, admin)
	_, otherCoach := onDutyCoach(t, h, admin)

	// 支払い前の依頼。
	pending := chosenReview(t, h, owner, coach.String(), "card", false)
	// 返却まで済んだ依頼と、その追加相談の下書き。
	root := chosenReview(t, h, owner, coach.String(), "card", false)
	payChosenReview(t, h, root, "card")
	answerOffer(t, h, tcoach, root)
	c := consultationDraft(t, h, owner, "/v1/coaching/requests/"+root+"/consultations", uuid.NewString(), "非公開の質問")

	denied := func(code int) bool { return code == http.StatusNotFound || code == http.StatusForbidden }
	sessions := sessionCount(f)
	for _, tok := range []struct{ who, tok string }{{"other requester", other}, {"other coach", otherCoach}} {
		for _, call := range []struct {
			method, path string
			body         any
		}{
			{"GET", "/v1/coaching/requests/" + pending, nil},
			{"POST", "/v1/coaching/requests/" + pending + "/checkout", map[string]any{"method": "card"}},
			{"GET", "/v1/coaching/requests/" + root, nil},
			{"GET", "/v1/coaching/requests/" + root + "/consultations", nil},
			{"POST", "/v1/coaching/requests/" + root + "/consultations", map[string]any{"request_key": uuid.NewString(), "body": "割り込み"}},
			{"POST", "/v1/coaching/consultations/" + c.ID + "/submit", nil},
			{"POST", "/v1/coaching/consultations/" + c.ID + "/checkout", nil},
			{"POST", "/v1/coaching/consultations/" + c.ID + "/cancel", nil},
			{"POST", "/v1/coaching/requests/" + root + "/rating", map[string]any{"stars": 1}},
			{"GET", "/v1/admin/coaching/payments", nil},
			{"POST", "/v1/admin/coaching/payments/" + uuid.NewString() + "/refund", map[string]any{"source": "review", "reason": "x"}},
		} {
			r := doJSON(t, h, call.method, call.path, tok.tok, call.body)
			if !denied(r.Code) {
				t.Fatalf("%s %s %s: %d %s", tok.who, call.method, call.path, r.Code, r.Body.String())
			}
			if strings.Contains(r.Body.String(), "非公開の質問") {
				t.Fatalf("%s %s leaked the consultation", tok.who, call.path)
			}
		}
	}
	if sessionCount(f) != sessions {
		t.Fatalf("checkout created by others: %d", sessionCount(f)-sessions)
	}
	var status string
	var consultations int
	queryRowArgs(t, `SELECT status FROM coaching_consultations WHERE id = $1`, []any{c.ID}, &status)
	queryRowArgs(t, `SELECT count(*) FROM coaching_consultations WHERE review_request_id = $1`, []any{root}, &consultations)
	if status != "draft" || consultations != 1 {
		t.Fatalf("consultation status=%s count=%d", status, consultations)
	}
	// 本人は同じ ID で読める（拒否が ID の書き間違いによるものではないことの確認）。
	if r := doJSON(t, h, "GET", "/v1/coaching/requests/"+pending, owner, nil); r.Code != 200 {
		t.Fatalf("owner read %d", r.Code)
	}
}

// 項目2：許可していない Origin には、プリフライトでも通常のリクエストでも Access-Control-Allow-Origin を返さない。
func TestCORSRejectsUnknownOrigins(t *testing.T) {
	h := setup(t)
	try := func(method, origin string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/v1/coaching/requests", nil)
		req.Header.Set("Origin", origin)
		if method == http.MethodOptions {
			req.Header.Set("Access-Control-Request-Method", "POST")
			req.Header.Set("Access-Control-Request-Headers", "authorization,content-type")
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	for _, origin := range []string{"https://evil.example", "http://localhost:3000.evil.example", "null", "http://localhost:3001"} {
		for _, method := range []string{http.MethodOptions, http.MethodPost, http.MethodGet} {
			rec := try(method, origin)
			if v := rec.Header().Get("Access-Control-Allow-Origin"); v != "" {
				t.Fatalf("%s from %s: Access-Control-Allow-Origin=%q", method, origin, v)
			}
			if v := rec.Header().Get("Access-Control-Allow-Credentials"); v != "" {
				t.Fatalf("%s from %s: Access-Control-Allow-Credentials=%q", method, origin, v)
			}
		}
	}
	// 許可した Origin には返る（テストが何も確かめていない状態になっていないことの確認）。
	if v := try(http.MethodOptions, "http://localhost:3000").Header().Get("Access-Control-Allow-Origin"); v != "http://localhost:3000" {
		t.Fatalf("allowed origin got %q", v)
	}
}

// 項目3：本番と同じカードだけの設定で、18歳未満は宣言のチェックがないと4経路のどれも進めず、
// 支払い画面も宣言の記録も作られない。
func TestMinorWithoutDeclarationCreatesNothingOnAnyPath(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupCoachChoice(t)
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)
	minor, tm := demoUser(t, h, time.Date(time.Now().Year()-15, 1, 1, 0, 0, 0, 0, time.UTC))

	refused := func(what string, r *httptest.ResponseRecorder) {
		t.Helper()
		if r.Code != http.StatusUnprocessableEntity || !strings.Contains(r.Body.String(), "guardian_permission_required") {
			t.Fatalf("%s %d %s", what, r.Code, r.Body.String())
		}
	}
	countDeclarations := func() int {
		var n int
		queryRowArgs(t, `SELECT count(*) FROM minor_purchase_declarations WHERE minor_id = $1`, []any{minor}, &n)
		return n
	}

	// 依頼の作成。
	body := reviewBodyFor(t, h, tm)
	body["method"] = "card"
	body["requested_coach_id"] = coach.String()
	sessions := sessionCount(f)
	refused("create", doJSON(t, h, "POST", "/v1/coaching/requests", tm, body))
	body["guardian_permission"] = false
	refused("create unchecked", doJSON(t, h, "POST", "/v1/coaching/requests", tm, body))
	if sessionCount(f) != sessions || countDeclarations() != 0 {
		t.Fatalf("create made sessions=%d declarations=%d", sessionCount(f)-sessions, countDeclarations())
	}

	// 支払い画面の開き直し（前の画面の期限が切れていて、作り直しになる場面）。
	root := chosenReview(t, h, tm, coach.String(), "card", false)
	declared := countDeclarations()
	execSQLArgs(t, `UPDATE review_payments SET session_expires_at = now() WHERE request_id = $1`, root)
	sessions = sessionCount(f)
	refused("reopen", doJSON(t, h, "POST", "/v1/coaching/requests/"+root+"/checkout", tm, map[string]any{"method": "card"}))
	refused("reopen unchecked", doJSON(t, h, "POST", "/v1/coaching/requests/"+root+"/checkout", tm, map[string]any{"method": "card", "guardian_permission": false}))
	if sessionCount(f) != sessions || countDeclarations() != declared {
		t.Fatalf("reopen made sessions=%d declarations=%d", sessionCount(f)-sessions, countDeclarations()-declared)
	}
	if r := doJSON(t, h, "POST", "/v1/coaching/requests/"+root+"/checkout", tm, map[string]any{"method": "card", "guardian_permission": true}); r.Code != 200 {
		t.Fatalf("reopen with declaration %d %s", r.Code, r.Body.String())
	}
	payChosenReview(t, h, root, "card")
	answerOffer(t, h, tc, root)

	// 追加相談の送信と、その支払い画面の開き直し。
	c := consultationDraft(t, h, tm, "/v1/coaching/requests/"+root+"/consultations", uuid.NewString(), "次の大会ではどこを意識しますか")
	declared = countDeclarations()
	sessions = sessionCount(f)
	refused("submit", doJSON(t, h, "POST", "/v1/coaching/consultations/"+c.ID+"/submit", tm, nil))
	refused("submit unchecked", doJSON(t, h, "POST", "/v1/coaching/consultations/"+c.ID+"/submit", tm, map[string]any{"guardian_permission": false}))
	if sessionCount(f) != sessions || countDeclarations() != declared {
		t.Fatalf("submit made sessions=%d declarations=%d", sessionCount(f)-sessions, countDeclarations()-declared)
	}
	if r := doJSON(t, h, "POST", "/v1/coaching/consultations/"+c.ID+"/submit", tm, map[string]any{"guardian_permission": true}); r.Code != 200 {
		t.Fatalf("submit with declaration %d %s", r.Code, r.Body.String())
	}
	execSQLArgs(t, `UPDATE coaching_consultations SET checkout_url = NULL, checkout_session_id = NULL WHERE id = $1`, c.ID)
	declared = countDeclarations()
	sessions = sessionCount(f)
	refused("consultation reopen", doJSON(t, h, "POST", "/v1/coaching/consultations/"+c.ID+"/checkout", tm, nil))
	if sessionCount(f) != sessions || countDeclarations() != declared {
		t.Fatalf("consultation reopen made sessions=%d declarations=%d", sessionCount(f)-sessions, countDeclarations()-declared)
	}
}

// 項目3：生まれた年月が未登録だと、依頼の作成は 422 birth_month_required で止まり、依頼も支払い画面も作られない。
func TestReviewWithoutBirthMonthCreatesNothing(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupCoachChoice(t)
	admin := adminToken(t, h)
	coach, _ := onDutyCoach(t, h, admin)
	id, tu := demoUser(t, h, time.Date(1995, 4, 1, 0, 0, 0, 0, time.UTC))
	execSQLArgs(t, `UPDATE profiles SET birth_month = NULL WHERE id = $1`, id)
	body := reviewBodyFor(t, h, tu)
	body["method"] = "card"
	body["requested_coach_id"] = coach.String()
	body["guardian_permission"] = true
	sessions := sessionCount(f)
	if r := doJSON(t, h, "POST", "/v1/coaching/requests", tu, body); r.Code != http.StatusUnprocessableEntity || !strings.Contains(r.Body.String(), "birth_month_required") {
		t.Fatalf("no birth month %d %s", r.Code, r.Body.String())
	}
	var requests, declarations int
	queryRowArgs(t, `SELECT count(*) FROM review_requests WHERE requester_id = $1`, []any{id}, &requests)
	queryRowArgs(t, `SELECT count(*) FROM minor_purchase_declarations WHERE minor_id = $1`, []any{id}, &declarations)
	if requests != 0 || declarations != 0 || sessionCount(f) != sessions {
		t.Fatalf("requests=%d declarations=%d sessions=%d", requests, declarations, sessionCount(f)-sessions)
	}
}

// 項目3：18歳以上は、4経路のどれでも宣言なしで進め、宣言の記録も未成年の印も残らない（カードだけの設定）。
func TestAdultNeedsNoDeclarationOnAnyPath(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupCoachChoice(t)
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)
	adult, tu := demoRequester(t, h, admin)

	body := reviewBodyFor(t, h, tu)
	body["method"] = "card"
	body["requested_coach_id"] = coach.String()
	r := doJSON(t, h, "POST", "/v1/coaching/requests", tu, body)
	if r.Code != http.StatusCreated {
		t.Fatalf("create %d %s", r.Code, r.Body.String())
	}
	var root string
	queryRowArgs(t, `SELECT id::text FROM review_requests WHERE requester_id = $1`, []any{adult}, &root)
	execSQLArgs(t, `UPDATE review_payments SET session_expires_at = now() WHERE request_id = $1`, root)
	if r := doJSON(t, h, "POST", "/v1/coaching/requests/"+root+"/checkout", tu, map[string]any{"method": "card"}); r.Code != 200 {
		t.Fatalf("reopen %d %s", r.Code, r.Body.String())
	}
	payChosenReview(t, h, root, "card")
	answerOffer(t, h, tc, root)
	c := consultationDraft(t, h, tu, "/v1/coaching/requests/"+root+"/consultations", uuid.NewString(), "次の大会ではどこを意識しますか")
	if r := doJSON(t, h, "POST", "/v1/coaching/consultations/"+c.ID+"/submit", tu, nil); r.Code != 200 {
		t.Fatalf("submit %d %s", r.Code, r.Body.String())
	}
	execSQLArgs(t, `UPDATE coaching_consultations SET checkout_url = NULL, checkout_session_id = NULL WHERE id = $1`, c.ID)
	if r := doJSON(t, h, "POST", "/v1/coaching/consultations/"+c.ID+"/checkout", tu, nil); r.Code != 200 {
		t.Fatalf("consultation reopen %d %s", r.Code, r.Body.String())
	}
	var declarations, minorPayments, minorConsultations int
	queryRowArgs(t, `SELECT count(*) FROM minor_purchase_declarations WHERE minor_id = $1`, []any{adult}, &declarations)
	queryRowArgs(t, `SELECT count(*) FROM review_payments WHERE request_id = $1 AND minor`, []any{root}, &minorPayments)
	queryRowArgs(t, `SELECT count(*) FROM coaching_consultations WHERE id = $1 AND minor`, []any{c.ID}, &minorConsultations)
	if declarations != 0 || minorPayments != 0 || minorConsultations != 0 {
		t.Fatalf("adult declarations=%d minor payments=%d minor consultations=%d", declarations, minorPayments, minorConsultations)
	}
}

// 項目5（層B のうち環境内で完結する分）：本番と同じ「指名・カード払い」の申込みで、
// 支払いが届くまでは打診も通知も作らず、支払いが届くとコーチへの打診のベル通知と Push の送信待ちができる。
// その場の Push が一時障害で送れなくても、tick が送り直して送信済みとして記録し、二重には送らない。
// Push はテスト用の購読（合成の宛先）に送り、実際の端末には届けない。
func TestPaidReviewOfferNotifiesCoachThroughTick(t *testing.T) {
	fakeCoachingCloudinary(t)
	gate := newPushGate(t, 0)
	priv, pub, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	h, _ := setupStripeConfig(t, func(c *config.Config) {
		c.ReviewRandomAssign = false
		c.VAPIDPublicKey = pub
		c.VAPIDPrivateKey = priv
		c.VAPIDSubject = "mailto:test@example.com"
		c.PushHTTP = gate.server.Client()
	})
	admin := adminToken(t, h)
	coach, _ := onDutyCoachTo(t, h, admin, gate.server.URL+"/coach/"+uuid.NewString())
	_, tu := demoRequester(t, h, admin)

	offerNotes := func() int {
		var n int
		queryRowArgs(t, `SELECT count(*) FROM notifications WHERE recipient_id = $1 AND kind = 'review_offer'`, []any{coach}, &n)
		return n
	}
	offerPushes := func() int {
		var n int
		queryRowArgs(t, `SELECT count(*) FROM push_outbox WHERE profile_id = $1 AND dedup_key LIKE 'review_offer:%'`, []any{coach}, &n)
		return n
	}

	// 申込み（支払い前）。打診・通知はまだ作らない。
	root := chosenReview(t, h, tu, coach.String(), "card", false)
	if n := offerNotes(); n != 0 {
		t.Fatalf("bell before payment %d", n)
	}
	if n := offerPushes(); n != 0 {
		t.Fatalf("push before payment %d", n)
	}

	// 支払い（仮押さえ）が届く。その場の Push は一時障害で失敗させる。
	gate.failNext(1)
	base := gate.sent.Load()
	payChosenReview(t, h, root, "card")
	var offers int
	queryRowArgs(t, `SELECT count(*) FROM review_offers WHERE request_id = $1 AND coach_id = $2`, []any{root, coach}, &offers)
	if offers != 1 || offerNotes() != 1 || offerPushes() != 1 {
		t.Fatalf("offers=%d bell=%d push=%d", offers, offerNotes(), offerPushes())
	}
	var dedup string
	queryRowArgs(t, `SELECT dedup_key FROM push_outbox WHERE profile_id = $1 AND dedup_key LIKE 'review_offer:%'`, []any{coach}, &dedup)
	if status, _ := outboxStatus(t, dedup); status != "pending" {
		t.Fatalf("after failed send %s", status)
	}
	if gate.sent.Load() != base {
		t.Fatalf("sent despite failure %d", gate.sent.Load()-base)
	}

	// tick が送り直し、送信済みとして記録する。
	execSQLArgs(t, `UPDATE push_outbox SET next_attempt_at = now() WHERE dedup_key = $1`, dedup)
	if rec := tickWithSecret(t, h, "tick-secret"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"push_resent":1`) {
		t.Fatalf("tick %d %s", rec.Code, rec.Body.String())
	}
	if status, attempts := outboxStatus(t, dedup); status != "sent" || attempts < 2 {
		t.Fatalf("after tick %s attempts=%d", status, attempts)
	}
	if gate.sent.Load() != base+1 {
		t.Fatalf("sent %d, want 1", gate.sent.Load()-base)
	}
	// もう一度 tick しても二重に送らず、ベル通知も増えない。
	if rec := tickWithSecret(t, h, "tick-secret"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"push_resent":0`) {
		t.Fatalf("second tick %d %s", rec.Code, rec.Body.String())
	}
	if gate.sent.Load() != base+1 || offerNotes() != 1 {
		t.Fatalf("duplicate sent=%d bell=%d", gate.sent.Load()-base, offerNotes())
	}
}
