package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

// guardianRecordCounts は、本人の保護者同意の証跡（同意・証跡・メールの記録）の行数。
func guardianRecordCounts(t *testing.T, minor uuid.UUID) (consents, events, notices int) {
	t.Helper()
	queryRowArgs(t, `SELECT (SELECT count(*) FROM guardian_consents WHERE minor_id = $1),
		(SELECT count(*) FROM guardian_consent_events WHERE minor_id = $1),
		(SELECT count(*) FROM guardian_notices WHERE minor_id = $1)`, []any{minor}, &consents, &events, &notices)
	return
}

// 保護者同意の証跡と購入の記録は、未成年の本人が退会しても残り（プロフィールなど他の情報は消える）、
// 最後の取引から5年たったら日次の処理で消える。取引がないときは最後の記録から数える。退会していない人の分は消さない。
func TestGuardianRecordsKeptAfterWithdrawalForFiveYears(t *testing.T) {
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/resources/video/authenticated/track-talk-coaching/"):
			_, _ = w.Write([]byte(`{"resource_type":"video","type":"authenticated","format":"mp4","bytes":1024,"duration":12}`))
		case r.Method == http.MethodDelete:
			json.NewEncoder(w).Encode(map[string]any{"deleted": map[string]string{}})
		default:
			json.NewEncoder(w).Encode(map[string]any{"resources": []any{}})
		}
	}))
	defer cloud.Close()
	t.Setenv("CLOUDINARY_CLOUD_NAME", "demo")
	t.Setenv("CLOUDINARY_API_KEY", "key")
	t.Setenv("CLOUDINARY_API_SECRET", "secret")
	t.Setenv("CLOUDINARY_API_BASE", cloud.URL)
	now := time.Now()
	h, _ := setupStripeConfig(t, func(c *config.Config) {
		c.ReviewRandomAssign = false
		c.StorageCleanupSecret = "storage-secret"
		c.Clock = func() time.Time { return now }
	})
	cleanup := func(at time.Time) {
		t.Helper()
		now = at
		if rec := cleanupRequest(h, "storage-secret"); rec.Code != http.StatusOK {
			t.Fatalf("cleanup %d %s", rec.Code, rec.Body.String())
		}
	}
	withdraw := func(tu string) {
		t.Helper()
		if rec := doJSON(t, h, http.MethodDelete, "/v1/me", tu, map[string]string{"confirm": "退会する"}); rec.Code != http.StatusOK {
			t.Fatalf("withdraw %d %s", rec.Code, rec.Body.String())
		}
	}
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)

	// A: 保護者が同意し、本人が動画添削を自分の PayPay で買い、返却されたあとで退会する。
	minor, tm := demoUser(t, h, time.Date(time.Now().Year()-14, 1, 1, 0, 0, 0, 0, time.UTC))
	email := guardianEmail()
	gt := issueGuardianLink(t, h, tm, email)
	if rec := doJSON(t, h, "POST", "/v1/guardian/"+gt+"/consent", "", map[string]any{
		"guardian_name": "山田 花子", "relationship": "mother", "self_pay_limit_yen": 3000, "agree": true,
	}); rec.Code != 200 {
		t.Fatalf("consent %d %s", rec.Code, rec.Body.String())
	}
	review := chosenReview(t, h, tm, coach.String(), "paypay", false)
	payChosenReview(t, h, review, "paypay")
	manage := guardianLinkPattern.FindStringSubmatch(lastMailTo(t, email).Text)[1]
	for _, o := range coachRequests(t, h, tc).Offers {
		if r := doJSON(t, h, "POST", "/v1/coach/offers/"+o.ID+"/accept", tc, nil); r.Code != 200 {
			t.Fatal(r.Body.String())
		}
	}
	if r := doJSON(t, h, "POST", "/v1/coaching/requests/"+review+"/response", tc, responseBody()); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	var intent string
	queryRowArgs(t, `SELECT payment_intent_id FROM review_payments WHERE request_id = $1`, []any{review}, &intent)
	withdraw(tm)

	// プロフィールと依頼（動画・相談内容）は消える。
	var profiles, requests int
	queryRowArgs(t, `SELECT (SELECT count(*) FROM profiles WHERE id = $1), (SELECT count(*) FROM review_requests WHERE requester_id = $1)`, []any{minor}, &profiles, &requests)
	if profiles != 0 || requests != 0 {
		t.Fatalf("after withdrawal profiles=%d requests=%d", profiles, requests)
	}
	// 同意の記録（保護者の氏名・メール・続柄、同意日時、規約の版、IP、上限）と証跡、購入の通知は残る。
	var name, gEmail, relationship, terms, ip string
	var limit int
	var deletedAt, lastTx, consentedAt *time.Time
	queryRowArgs(t, `SELECT guardian_name, guardian_email, relationship, terms_version, consent_ip, self_pay_limit_yen, account_deleted_at, last_transaction_at, consented_at
		FROM guardian_consents WHERE minor_id = $1`, []any{minor}, &name, &gEmail, &relationship, &terms, &ip, &limit, &deletedAt, &lastTx, &consentedAt)
	if name != "山田 花子" || gEmail != email || relationship != "mother" || terms != "guardian-consent-2026-10-final" || ip == "" || limit != 3000 ||
		deletedAt == nil || lastTx == nil || consentedAt == nil {
		t.Fatalf("consent record name=%q email=%q rel=%q terms=%q ip=%q limit=%d deleted=%v lastTx=%v", name, gEmail, relationship, terms, ip, limit, deletedAt, lastTx)
	}
	if events := guardianEvents(t, minor); events != "link_sent,consented" {
		t.Fatalf("events %q", events)
	}
	var purchases int
	queryRowArgs(t, `SELECT count(*) FROM guardian_notices WHERE minor_id = $1 AND kind = 'purchase' AND ref_id = $2 AND amount_yen = 1500 AND payment_intent_id = $3`,
		[]any{minor, review, intent}, &purchases)
	if purchases != 1 {
		t.Fatal("purchase record not kept")
	}
	// 取引の記録（支払い）も残り、Stripe の支払い ID で購入の通知と突き合わせられる。
	var payments int
	queryRowArgs(t, `SELECT count(*) FROM review_payments WHERE payment_intent_id = $1 AND status = 'captured'`, []any{intent}, &payments)
	if payments != 1 {
		t.Fatal("payment record missing")
	}
	// 退会したあとは、保護者のリンクは同意用も変更・撤回用も使えない。
	for _, link := range []string{gt, manage} {
		if rec := doJSON(t, h, http.MethodGet, "/v1/guardian/"+link, "", nil); rec.Code != http.StatusNotFound {
			t.Fatalf("guardian link after withdrawal %d %s", rec.Code, rec.Body.String())
		}
	}
	var links int
	queryRowArgs(t, `SELECT count(*) FROM guardian_manage_links WHERE minor_id = $1`, []any{minor}, &links)
	if links != 0 {
		t.Fatalf("manage links %d", links)
	}

	// B: 保護者にリンクを送っただけ（同意も取引もない）で退会する。
	pendingMinor, tp := demoUser(t, h, time.Date(time.Now().Year()-15, 1, 1, 0, 0, 0, 0, time.UTC))
	issueGuardianLink(t, h, tp, guardianEmail())
	withdraw(tp)
	// C: 同意済みで、退会していない。
	active, tActive := demoUser(t, h, time.Date(time.Now().Year()-16, 1, 1, 0, 0, 0, 0, time.UTC))
	ga := issueGuardianLink(t, h, tActive, guardianEmail())
	if rec := doJSON(t, h, "POST", "/v1/guardian/"+ga+"/consent", "", map[string]any{
		"guardian_name": "保護者", "relationship": "father", "agree": true,
	}); rec.Code != 200 {
		t.Fatalf("consent %d %s", rec.Code, rec.Body.String())
	}

	// 最後の取引（A）・最後の記録（B）から5年たつまでは残す。
	start := time.Now()
	cleanup(start.AddDate(4, 11, 0))
	for _, m := range []uuid.UUID{minor, pendingMinor} {
		if c, e, n := guardianRecordCounts(t, m); c != 1 || e == 0 || n == 0 {
			t.Fatalf("deleted before 5 years %v: %d %d %d", m, c, e, n)
		}
	}
	// 5年を過ぎたら、同意・証跡・メールの記録をまとめて消す。退会していない人の分は残る。
	cleanup(start.AddDate(5, 0, 7))
	for _, m := range []uuid.UUID{minor, pendingMinor} {
		if c, e, n := guardianRecordCounts(t, m); c != 0 || e != 0 || n != 0 {
			t.Fatalf("kept after 5 years %v: %d %d %d", m, c, e, n)
		}
	}
	if c, _, _ := guardianRecordCounts(t, active); c != 1 {
		t.Fatal("active minor's consent deleted")
	}
}
