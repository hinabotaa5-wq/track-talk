package httpx_test

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// lastMailTo は、そのアドレスに最後に届いたメール。
func lastMailTo(t *testing.T, to string) sentMail {
	t.Helper()
	testOutboxMu.Lock()
	defer testOutboxMu.Unlock()
	for i := len(testOutbox) - 1; i >= 0; i-- {
		if m := testOutbox[i]; len(m.To) == 1 && m.To[0] == to {
			return m
		}
	}
	t.Fatalf("no mail to %s", to)
	return sentMail{}
}

func guardianEvents(t *testing.T, minor uuid.UUID) string {
	t.Helper()
	var events string
	queryRowArgs(t, `SELECT COALESCE(string_agg(event, ',' ORDER BY id), '') FROM guardian_consent_events WHERE minor_id = $1`, []any{minor}, &events)
	return events
}

// 未成年が自分の PayPay で動画添削・追加相談を買う流れ（保護者同意のフル版）。
// 同意がなければ買えず、保護者がリンクから同意すると上限の範囲で買え、購入のたびに保護者へ通知が届く。
// 上限を超える購入は止めて保護者にお願いのメールを送り、保護者が上限を上げると買える。撤回すると以後は買えない。
func TestGuardianFullConsentPurchaseFlow(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupCoachChoice(t)
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)
	minor, tm := demoUser(t, h, time.Date(time.Now().Year()-14, 1, 1, 0, 0, 0, 0, time.UTC))
	email := guardianEmail()

	newReview := func(method string) *httptest.ResponseRecorder {
		b := reviewBodyFor(t, h, tm)
		b["method"] = method
		b["requested_coach_id"] = coach.String()
		return doJSON(t, h, "POST", "/v1/coaching/requests", tm, b)
	}

	// 同意がないと、支払い画面を作らずに止める。
	before := sessionCount(f)
	if r := newReview("paypay"); r.Code != 422 || !strings.Contains(r.Body.String(), "guardian_required") {
		t.Fatalf("purchase without consent %d %s", r.Code, r.Body.String())
	}
	if sessionCount(f) != before {
		t.Fatal("checkout created without consent")
	}

	// 本人が保護者のアドレスを入れると、同意のリンクは保護者のメールにだけ届く（本人には返らない）。
	gt := issueGuardianLink(t, h, tm, email)
	if mailsTo(email) != 1 || !strings.Contains(lastMailTo(t, email).Text, "PayPay") {
		t.Fatalf("consent request mail %d", mailsTo(email))
	}
	// 同意依頼中もまだ買えない。
	if r := newReview("paypay"); r.Code != 422 || !strings.Contains(r.Body.String(), "guardian_required") {
		t.Fatalf("purchase while pending %d %s", r.Code, r.Body.String())
	}

	// 保護者がリンクから上限3,000円で同意する。日時・IP・規約の版・使ったリンクが残る。
	gp := "/v1/guardian/" + gt
	rec := doJSON(t, h, "POST", gp+"/consent", "", map[string]any{
		"guardian_name": "山田 花子", "relationship": "mother", "self_pay_limit_yen": 3000, "agree": true,
	})
	if rec.Code != 200 {
		t.Fatalf("consent %d %s", rec.Code, rec.Body.String())
	}
	var ip, terms string
	var tokenUsed bool
	queryRowArgs(t, `SELECT COALESCE(consent_ip,''), COALESCE(terms_version,''), consent_token_hash = token_hash FROM guardian_consents WHERE minor_id = $1`, []any{minor}, &ip, &terms, &tokenUsed)
	if ip == "" || terms == "" || !tokenUsed {
		t.Fatalf("consent record ip=%q terms=%q used=%v", ip, terms, tokenUsed)
	}
	if mailsTo(email) != 2 || !strings.Contains(lastMailTo(t, email).Subject, "同意を受け付けました") {
		t.Fatalf("consented mail %d", mailsTo(email))
	}

	// 同意の範囲で、本人が1,500円の添削を自分の PayPay で買える。支払いが届くと保護者に通知が届く。
	first := chosenReview(t, h, tm, coach.String(), "paypay", false)
	if mailsTo(email) != 2 {
		t.Fatal("purchase notice before payment")
	}
	payChosenReview(t, h, first, "paypay")
	notice := lastMailTo(t, email)
	if mailsTo(email) != 3 || !strings.Contains(notice.Subject, "動画添削") || !strings.Contains(notice.Text, "1,500円") ||
		strings.Contains(notice.Text, "（不明）") || !guardianLinkPattern.MatchString(notice.Text) {
		t.Fatalf("purchase notice %+v", notice)
	}
	// 同じ支払いの知らせが2回届いても、保護者への通知は1回。
	payChosenReview(t, h, first, "paypay")
	if mailsTo(email) != 3 {
		t.Fatalf("duplicate notice %d", mailsTo(email))
	}
	second := chosenReview(t, h, tm, coach.String(), "card", false)
	payChosenReview(t, h, second, "card")
	if mailsTo(email) != 4 {
		t.Fatalf("second notice %d", mailsTo(email))
	}

	// 1本目を返却してもらい、追加相談（600円）を頼む。今月は3,000円に達しているので止まり、保護者にお願いが届く。
	offers := coachRequests(t, h, tc).Offers
	for _, o := range offers {
		if r := doJSON(t, h, "POST", "/v1/coach/offers/"+o.ID+"/accept", tc, nil); r.Code != 200 {
			t.Fatal(r.Body.String())
		}
	}
	if r := doJSON(t, h, "POST", "/v1/coaching/requests/"+first+"/response", tc, responseBody()); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	path := "/v1/coaching/requests/" + first + "/consultations"
	c := consultationDraft(t, h, tm, path, uuid.NewString(), "次の大会ではどこを意識しますか")
	if c.Funding != "cash" || c.Price != 600 {
		t.Fatalf("draft %+v", c)
	}
	rec = doJSON(t, h, "POST", "/v1/coaching/consultations/"+c.ID+"/submit", tm, nil)
	if rec.Code != 422 || !strings.Contains(rec.Body.String(), "guardian_limit") || !strings.Contains(rec.Body.String(), "保護者の確認が必要") ||
		!strings.Contains(rec.Body.String(), "お願いするメールを送りました") {
		t.Fatalf("over limit %d %s", rec.Code, rec.Body.String())
	}
	request := lastMailTo(t, email)
	if mailsTo(email) != 5 || !strings.Contains(request.Subject, "上限") {
		t.Fatalf("limit request mail %+v", request)
	}
	// お願いのメールは12時間に1回まで。
	if rec := doJSON(t, h, "POST", "/v1/coaching/consultations/"+c.ID+"/submit", tm, nil); rec.Code != 422 || mailsTo(email) != 5 {
		t.Fatalf("second over limit %d mails %d", rec.Code, mailsTo(email))
	}

	// 保護者がお願いのメールのリンクから上限を5,000円に上げると、買えるようになる。
	manage := "/v1/guardian/" + guardianLinkPattern.FindStringSubmatch(request.Text)[1]
	if rec := doJSON(t, h, "PATCH", manage, "", map[string]any{"self_pay_limit_yen": 5000}); rec.Code != 200 {
		t.Fatalf("raise limit %d %s", rec.Code, rec.Body.String())
	}
	var prev, next int
	queryRowArgs(t, `SELECT previous_self_pay_limit_yen, self_pay_limit_yen FROM guardian_consent_events WHERE minor_id = $1 AND event = 'limit_changed'`, []any{minor}, &prev, &next)
	if prev != 3000 || next != 5000 {
		t.Fatalf("limit history %d -> %d", prev, next)
	}
	rec = doJSON(t, h, "POST", "/v1/coaching/consultations/"+c.ID+"/submit", tm, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "checkout_url") {
		t.Fatalf("submit after raise %d %s", rec.Code, rec.Body.String())
	}
	form := f.lastSession()
	if form.Get("payment_method_types[1]") != "paypay" || form.Get("success_url") != "https://web.test/coaching/requests/"+first+"?consultation="+c.ID {
		t.Fatalf("consultation session %v", form)
	}
	if code := payConsultation(t, h, c.ID, 600); code != 200 {
		t.Fatalf("pay consultation %d", code)
	}
	notice = lastMailTo(t, email)
	if mailsTo(email) != 6 || !strings.Contains(notice.Subject, "追加相談") || !strings.Contains(notice.Text, "600円") {
		t.Fatalf("consultation notice %+v", notice)
	}

	// 変更用のリンクでは同意そのものはできない。
	if rec := doJSON(t, h, "POST", manage+"/consent", "", map[string]any{"guardian_name": "誰か", "relationship": "other", "agree": true}); rec.Code != 403 {
		t.Fatalf("consent via manage link %d %s", rec.Code, rec.Body.String())
	}

	// 保護者が撤回すると、以後の購入は止まる。購入済みの添削・相談はそのまま（取り消し・返金しない）。
	if rec := doJSON(t, h, "POST", manage+"/revoke", "", nil); rec.Code != 200 {
		t.Fatalf("revoke %d %s", rec.Code, rec.Body.String())
	}
	if r := newReview("paypay"); r.Code != 422 || !strings.Contains(r.Body.String(), "guardian_revoked") {
		t.Fatalf("purchase after revoke %d %s", r.Code, r.Body.String())
	}
	if refunds := f.refundList(); len(refunds) != 0 {
		t.Fatalf("revoke refunded %v", refunds)
	}
	var reviewStatus, consultStatus string
	queryRowArgs(t, `SELECT status FROM review_requests WHERE id = $1`, []any{second}, &reviewStatus)
	queryRowArgs(t, `SELECT status FROM coaching_consultations WHERE id = $1`, []any{c.ID}, &consultStatus)
	if reviewStatus != "in_review" || consultStatus != "offered" {
		t.Fatalf("after revoke review=%s consultation=%s", reviewStatus, consultStatus)
	}

	if got := guardianEvents(t, minor); got != "link_sent,consented,limit_request_sent,limit_changed,revoked" {
		t.Fatalf("events %s", got)
	}
	var sent, purchases int
	queryRowArgs(t, `SELECT count(*) FILTER (WHERE status = 'sent'), count(*) FILTER (WHERE kind = 'purchase') FROM guardian_notices WHERE minor_id = $1`, []any{minor}, &sent, &purchases)
	if sent != 6 || purchases != 3 {
		t.Fatalf("notice log sent=%d purchases=%d", sent, purchases)
	}
}

// 同意用のリンクは推測できず、期限があり、同意には1回しか使えない。本人のアカウントからは同意できない。
func TestGuardianConsentLinkIsSingleUseAndExpires(t *testing.T) {
	h := setup(t)
	minor, tm := demoUser(t, h, time.Date(time.Now().Year()-15, 1, 1, 0, 0, 0, 0, time.UTC))
	email := guardianEmail()
	gt := issueGuardianLink(t, h, tm, email)
	gp := "/v1/guardian/" + gt
	consent := map[string]any{"guardian_name": "山田 太郎", "relationship": "father", "self_pay_limit_yen": 1500, "agree": true}

	// 本人のログインで同意用の API を呼んでも、トークンを知らなければ同意できない。
	if rec := doJSON(t, h, "POST", "/v1/guardian/"+strings.Repeat("A", 43)+"/consent", tm, consent); rec.Code != 404 {
		t.Fatalf("guess %d", rec.Code)
	}
	if rec := doJSON(t, h, "POST", "/v1/me/guardian-consent", tm, consent); rec.Code != 404 && rec.Code != 405 {
		t.Fatalf("self consent route %d", rec.Code)
	}

	if rec := doJSON(t, h, "POST", gp+"/consent", "", consent); rec.Code != 200 {
		t.Fatalf("consent %d %s", rec.Code, rec.Body.String())
	}
	// 同じリンクで二度目の同意はできない（撤回のあとでも）。
	if rec := doJSON(t, h, "POST", gp+"/consent", "", consent); rec.Code != 409 {
		t.Fatalf("reuse while consented %d", rec.Code)
	}
	if rec := doJSON(t, h, "POST", gp+"/revoke", "", nil); rec.Code != 200 {
		t.Fatalf("revoke %d", rec.Code)
	}
	rec := doJSON(t, h, "POST", gp+"/consent", "", consent)
	if rec.Code != 409 || !strings.Contains(rec.Body.String(), "consent_link_used") {
		t.Fatalf("reuse after revoke %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, "GET", gp, "", nil); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"can_consent":false`) {
		t.Fatalf("page after revoke %d %s", rec.Code, rec.Body.String())
	}

	// 新しいリンクなら同意し直せる。出してから30日を過ぎたリンクは使えない。
	fresh := issueGuardianLink(t, h, tm, email)
	execSQLArgs(t, `UPDATE guardian_consents SET link_issued_at = now() - interval '31 days' WHERE minor_id = $1`, minor)
	if rec := doJSON(t, h, "POST", "/v1/guardian/"+fresh+"/consent", "", consent); rec.Code != 404 {
		t.Fatalf("expired link %d", rec.Code)
	}
	execSQLArgs(t, `UPDATE guardian_consents SET link_issued_at = now() WHERE minor_id = $1`, minor)
	if rec := doJSON(t, h, "POST", "/v1/guardian/"+fresh+"/consent", "", consent); rec.Code != 200 {
		t.Fatalf("consent with new link %d %s", rec.Code, rec.Body.String())
	}

	// 変更用のリンクも期限を過ぎると使えない。
	manage := guardianLinkPattern.FindStringSubmatch(lastMailTo(t, email).Text)[1]
	if rec := doJSON(t, h, "GET", "/v1/guardian/"+manage, "", nil); rec.Code != 200 {
		t.Fatalf("manage link %d", rec.Code)
	}
	execSQLArgs(t, `UPDATE guardian_manage_links SET expires_at = now() - interval '1 minute' WHERE minor_id = $1`, minor)
	if rec := doJSON(t, h, "GET", "/v1/guardian/"+manage, "", nil); rec.Code != 404 {
		t.Fatalf("expired manage link %d", rec.Code)
	}
}

// 上限の判定はロックの中で行うので、添削と追加相談を同時に頼んでも、今月の合計は上限を超えない。
func TestGuardianLimitHoldsUnderConcurrentPurchases(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupCoachChoice(t)
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)
	minor, _ := guardianMinor(t, h)
	execSQLArgs(t, `UPDATE guardian_consents SET self_pay_limit_yen = 5000`)
	// 添削を2本（3,000円）買って返却してもらい、それぞれに追加相談の下書きを作る。残りは2,000円。
	roots := []string{chosenReview(t, h, minor, coach.String(), "paypay", false), chosenReview(t, h, minor, coach.String(), "paypay", false)}
	for _, root := range roots {
		payChosenReview(t, h, root, "paypay")
	}
	for _, o := range coachRequests(t, h, tc).Offers {
		if r := doJSON(t, h, "POST", "/v1/coach/offers/"+o.ID+"/accept", tc, nil); r.Code != 200 {
			t.Fatal(r.Body.String())
		}
	}
	drafts := []string{}
	for _, root := range roots {
		if r := doJSON(t, h, "POST", "/v1/coaching/requests/"+root+"/response", tc, responseBody()); r.Code != 200 {
			t.Fatal(r.Body.String())
		}
		drafts = append(drafts, consultationDraft(t, h, minor, "/v1/coaching/requests/"+root+"/consultations", uuid.NewString(), "質問").ID)
	}
	// 相談2件（600円ずつ）と添削2本（1,500円ずつ）を同時に頼む。
	bodies := []map[string]any{}
	for range 2 {
		b := reviewBodyFor(t, h, minor)
		b["method"] = "paypay"
		b["requested_coach_id"] = coach.String()
		bodies = append(bodies, b)
	}
	codes := make(chan int, 4)
	for _, id := range drafts {
		go func() { codes <- doJSON(t, h, "POST", "/v1/coaching/consultations/"+id+"/submit", minor, nil).Code }()
	}
	for _, b := range bodies {
		go func() { codes <- doJSON(t, h, "POST", "/v1/coaching/requests", minor, b).Code }()
	}
	ok := 0
	for range 4 {
		if code := <-codes; code == 200 || code == 201 {
			ok++
		}
	}
	var me coachingMe
	_ = json.Unmarshal(doJSON(t, h, "GET", "/v1/me/coaching", minor, nil).Body.Bytes(), &me)
	if ok == 0 || ok == 4 || me.Guardian == nil || me.Guardian.MonthSpentYen > 5000 || me.Guardian.MonthSpentYen < 3600 {
		t.Fatalf("accepted %d, spent %+v", ok, me.Guardian)
	}
}

// 大人の購入には同意も通知もいらない。
func TestAdultPurchaseNeedsNoGuardian(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupCoachChoice(t)
	admin := adminToken(t, h)
	coach, _ := onDutyCoach(t, h, admin)
	adult, tu := demoRequester(t, h, admin)
	id := chosenReview(t, h, tu, coach.String(), "paypay", false)
	payChosenReview(t, h, id, "paypay")
	var notices int
	queryRowArgs(t, `SELECT count(*)::int FROM guardian_notices WHERE minor_id = $1`, []any{adult}, &notices)
	if notices != 0 {
		t.Fatalf("adult notices %d", notices)
	}
}
