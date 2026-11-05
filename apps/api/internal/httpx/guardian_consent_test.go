package httpx_test

import (
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
