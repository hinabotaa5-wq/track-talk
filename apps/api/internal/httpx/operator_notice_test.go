package httpx_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

// 運営者通知の宛先。公開ページ掲載のサポート用アドレスで、実装側の定数と同じ。
const operatorNoticeTo = "tracktalk.office@gmail.com"

// operatorMailCount は、そのテストまでに運営者宛てに届いたメールの数。
// testOutbox はテスト全体で共有されるため、差分で数える。
func operatorMailCount() int {
	testOutboxMu.Lock()
	defer testOutboxMu.Unlock()
	n := 0
	for _, m := range testOutbox {
		if len(m.To) == 1 && m.To[0] == operatorNoticeTo {
			n++
		}
	}
	return n
}

func lastOperatorMail(t *testing.T) sentMail {
	t.Helper()
	testOutboxMu.Lock()
	defer testOutboxMu.Unlock()
	for i := len(testOutbox) - 1; i >= 0; i-- {
		if m := testOutbox[i]; len(m.To) == 1 && m.To[0] == operatorNoticeTo {
			return m
		}
	}
	t.Fatal("no mail to operator")
	return sentMail{}
}

func captureOperatorLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

// コーチ応募の新規・書き直し・出し直しで各1通、宛先・件名・本文項目が仕様どおりで、実績文を含まない。
func TestCoachApplicationNotifiesOperator(t *testing.T) {
	h := setup(t)
	applicant := uuid.New()
	ta := token(t, applicant)
	before := operatorMailCount()

	rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", ta, coachApplicationBody())
	if rec.Code != http.StatusOK {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
	if got := operatorMailCount() - before; got != 1 {
		t.Fatalf("mails after apply = %d, want 1", got)
	}
	m := lastOperatorMail(t)
	if m.Subject != "コーチ応募がありました" {
		t.Fatalf("subject %q", m.Subject)
	}
	for _, want := range []string{"テストコーチ", "coach", "110mh", "JST", "/admin"} {
		if !strings.Contains(m.Text, want) {
			t.Fatalf("mail missing %q: %q", want, m.Text)
		}
	}
	for _, leak := range []string{"インターハイ 100m 準決勝", "100m 10秒95（2025年）"} {
		if strings.Contains(m.Text, leak) {
			t.Fatalf("mail contains application prose %q: %q", leak, m.Text)
		}
	}

	// 審査待ち中の書き直しでも保存のたびに1通。
	edit := coachApplicationBody()
	edit["years"] = 9
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", ta, edit); rec.Code != http.StatusOK {
		t.Fatalf("edit %d %s", rec.Code, rec.Body.String())
	}
	if got := operatorMailCount() - before; got != 2 {
		t.Fatalf("mails after edit = %d, want 2", got)
	}

	// 見送り後の出し直しでも保存のたびに1通。
	admin := uuid.New()
	tadmin := token(t, admin)
	if rec := doJSON(t, h, http.MethodGet, "/v1/me", tadmin, nil); rec.Code != http.StatusOK {
		t.Fatalf("admin me %d %s", rec.Code, rec.Body.String())
	}
	setAdmin(t, admin)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+applicant.String()+"/reject", tadmin, nil); rec.Code != http.StatusOK {
		t.Fatalf("reject %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", ta, coachApplicationBody()); rec.Code != http.StatusOK {
		t.Fatalf("reapply %d %s", rec.Code, rec.Body.String())
	}
	if got := operatorMailCount() - before; got != 3 {
		t.Fatalf("mails after reapply = %d, want 3", got)
	}
	m = lastOperatorMail(t)
	if m.Subject != "コーチ応募がありました" {
		t.Fatalf("reapply subject %q", m.Subject)
	}
}

// 問い合わせは送信元3種で各1通。本文に問い合わせ本文を含まず、連絡先は入力がある場合だけ載る。
func TestInquiryNotifiesOperator(t *testing.T) {
	h := setup(t)
	user := uuid.New()
	tu := tokenWithProfile(t, user, "合成利用者", true)
	before := operatorMailCount()

	cases := []struct {
		source    string
		category  string
		subject   string
		body      string
		email     string
		wantEmail bool
	}{
		{source: "contact", category: "bug", subject: "不具合の件名", body: "問い合わせ本文その1-含めてはいけない", email: "contact-user@example.com", wantEmail: true},
		{source: "feedback", category: "request", subject: "要望の件名", body: "問い合わせ本文その2-含めてはいけない"},
		{source: "product_request", category: "other", subject: "相談の件名", body: "問い合わせ本文その3-含めてはいけない"},
	}
	for _, tc := range cases {
		req := map[string]string{
			"category": tc.category, "subject": tc.subject, "body": tc.body, "source": tc.source,
		}
		if tc.email != "" {
			req["email"] = tc.email
		}
		rec := doJSON(t, h, http.MethodPost, "/v1/me/inquiries", tu, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create %s %d %s", tc.source, rec.Code, rec.Body.String())
		}
	}
	if got := operatorMailCount() - before; got != len(cases) {
		t.Fatalf("mails = %d, want %d", got, len(cases))
	}
	// 直近3通を取り出して1通ずつ確かめる。
	testOutboxMu.Lock()
	var recent []sentMail
	for i := len(testOutbox) - 1; i >= 0 && len(recent) < len(cases); i-- {
		if m := testOutbox[i]; len(m.To) == 1 && m.To[0] == operatorNoticeTo {
			recent = append([]sentMail{m}, recent...)
		}
	}
	testOutboxMu.Unlock()
	if len(recent) != len(cases) {
		t.Fatalf("recent mails = %d, want %d", len(recent), len(cases))
	}
	for i, tc := range cases {
		m := recent[i]
		if m.Subject != "お問い合わせがありました" {
			t.Fatalf("source %s subject %q", tc.source, m.Subject)
		}
		for _, want := range []string{tc.source, tc.category, tc.subject, "合成利用者", "JST", "/admin"} {
			if !strings.Contains(m.Text, want) {
				t.Fatalf("source %s mail missing %q: %q", tc.source, want, m.Text)
			}
		}
		if strings.Contains(m.Text, tc.body) {
			t.Fatalf("source %s mail contains inquiry body: %q", tc.source, m.Text)
		}
		if tc.wantEmail {
			if !strings.Contains(m.Text, tc.email) {
				t.Fatalf("contact mail missing email: %q", m.Text)
			}
		} else if strings.Contains(m.Text, "@") {
			t.Fatalf("source %s mail has unexpected email: %q", tc.source, m.Text)
		}
	}
}

// メール未設定・送信失敗でも応募・問い合わせは成功し、ログに本文・連絡先が出ない。
func TestOperatorNoticeFailureKeepsSaveWithoutLeak(t *testing.T) {
	const contactEmail = "leak-check-624@example.com"
	const inquiryBody = "漏えい確認の問い合わせ本文624"
	const achievements = "漏えい確認の実績文624"

	run := func(t *testing.T, h http.Handler) *bytes.Buffer {
		t.Helper()
		logs := captureOperatorLogs(t)
		before := operatorMailCount()
		applicant := uuid.New()
		ta := token(t, applicant)
		apply := coachApplicationBody()
		apply["achievements"] = achievements
		if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", ta, apply); rec.Code != http.StatusOK {
			t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
		}
		user := uuid.New()
		tu := tokenWithProfile(t, user, "漏えい確認利用者", true)
		rec := doJSON(t, h, http.MethodPost, "/v1/me/inquiries", tu, map[string]string{
			"category": "account", "subject": "漏えい確認の件名", "body": inquiryBody, "source": "contact", "email": contactEmail,
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("create %d %s", rec.Code, rec.Body.String())
		}
		if got := operatorMailCount() - before; got != 0 {
			t.Fatalf("mails = %d, want 0", got)
		}
		if !strings.Contains(logs.String(), "operator notice mail failed") {
			t.Fatalf("no failure warn in logs: %q", logs.String())
		}
		for _, leak := range []string{inquiryBody, achievements, contactEmail} {
			if strings.Contains(logs.String(), leak) {
				t.Fatalf("log leaks %q: %q", leak, logs.String())
			}
		}
		return logs
	}

	// メール未設定でも保存は成功する。
	h := setupConfig(t, func(cfg *config.Config) { cfg.ResendAPIKey = "" })
	run(t, h)

	// 送信に失敗しても保存は成功する。
	h = setup(t)
	testMailDown.Store(true)
	defer testMailDown.Store(false)
	run(t, h)
	testMailDown.Store(false)
}
