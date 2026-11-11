package httpx_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

const identityViewNoticeTo = "tracktalk.office@gmail.com"

// identityViewMails は、そのコーチの閲覧通知として届いたメール。
func identityViewMails(coach uuid.UUID) []sentMail {
	testOutboxMu.Lock()
	defer testOutboxMu.Unlock()
	var out []sentMail
	for _, m := range testOutbox {
		if len(m.To) == 1 && m.To[0] == identityViewNoticeTo && strings.Contains(m.Text, coach.String()) {
			out = append(out, m)
		}
	}
	return out
}

// 身元情報の表示1回につき、決まった宛先・件名・本文の通知が1通だけ届き、本文には識別子と JST の時刻しかない。
func TestIdentityViewNoticeSendsOneMailWithoutValues(t *testing.T) {
	h := setup(t)
	adminID := uuid.New()
	admin := token(t, adminID)
	coach := uuid.New()
	viewedAt := time.Now().UTC().Truncate(time.Second)
	path := "/v1/admin/coaching/coaches/" + coach.String() + "/identity-view-notice"
	body := map[string]any{"viewed_at": viewedAt.Format(time.RFC3339)}

	if rec := doJSON(t, h, http.MethodPost, path, "", body); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, path, admin, body); rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin %d %s", rec.Code, rec.Body.String())
	}
	if got := identityViewMails(coach); len(got) != 0 {
		t.Fatalf("mail before admin: %+v", got)
	}

	setAdmin(t, adminID)
	if rec := doJSON(t, h, http.MethodPost, path, admin, body); rec.Code != http.StatusNoContent {
		t.Fatalf("notice %d %s", rec.Code, rec.Body.String())
	}
	got := identityViewMails(coach)
	if len(got) != 1 {
		t.Fatalf("mails = %d, want 1: %+v", len(got), got)
	}
	m := got[0]
	if m.Subject != "身元情報の閲覧通知" {
		t.Fatalf("subject %q", m.Subject)
	}
	jst := viewedAt.In(time.FixedZone("JST", 9*60*60)).Format("2006-01-02 15:04:05")
	want := "管理画面でコーチの身元情報が表示されました。対象コーチID：" + coach.String() + "／時刻：" + jst + " JST。このメールに身元情報の値は含まれていません。心当たりがない場合はアカウント乗っ取り対応ランブックに従ってください。"
	if m.Text != want {
		t.Fatalf("text\n got %q\nwant %q", m.Text, want)
	}
}

// 身元情報の値を一緒に渡そうとしても受け付けず、メールにもログにも出ない。
func TestIdentityViewNoticeRejectsIdentityValues(t *testing.T) {
	h := setup(t)
	adminID := uuid.New()
	admin := token(t, adminID)
	doJSON(t, h, http.MethodGet, "/v1/me", admin, nil)
	setAdmin(t, adminID)
	coach := uuid.New()
	path := "/v1/admin/coaching/coaches/" + coach.String() + "/identity-view-notice"
	const name, address, phone = "合成太郎", "合成県合成市1-2-3", "090-0000-0000"

	var logs bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	defer slog.SetDefault(prev)

	rec := doJSON(t, h, http.MethodPost, path, admin, map[string]any{
		"viewed_at": time.Now().UTC().Format(time.RFC3339),
		"identity":  map[string]any{"legal_name": name, "address": address, "phone": phone},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("with values %d %s", rec.Code, rec.Body.String())
	}
	for _, bad := range []map[string]any{
		{},
		{"viewed_at": "not-a-time"},
		{"viewed_at": time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339)},
		{"viewed_at": time.Now().Add(time.Hour).UTC().Format(time.RFC3339)},
	} {
		if rec := doJSON(t, h, http.MethodPost, path, admin, bad); rec.Code != http.StatusBadRequest && rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("bad %v: %d %s", bad, rec.Code, rec.Body.String())
		}
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/coaches/not-a-uuid/identity-view-notice", admin, map[string]any{
		"viewed_at": time.Now().UTC().Format(time.RFC3339),
	}); rec.Code != http.StatusNotFound {
		t.Fatalf("bad id %d %s", rec.Code, rec.Body.String())
	}
	if got := identityViewMails(coach); len(got) != 0 {
		t.Fatalf("mail sent for rejected request: %+v", got)
	}
	for _, v := range []string{name, address, phone} {
		if strings.Contains(logs.String(), v) || strings.Contains(rec.Body.String(), v) {
			t.Fatalf("identity value %q leaked", v)
		}
	}
}

// 送信に失敗しても値はログに出ず、画面へは失敗だけを返す（表示は画面側で続く）。
func TestIdentityViewNoticeMailFailureIsRecordedWithoutValues(t *testing.T) {
	h := setup(t)
	adminID := uuid.New()
	admin := token(t, adminID)
	doJSON(t, h, http.MethodGet, "/v1/me", admin, nil)
	setAdmin(t, adminID)
	coach := uuid.New()

	var logs bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	defer slog.SetDefault(prev)
	testMailDown.Store(true)
	defer testMailDown.Store(false)

	rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/coaches/"+coach.String()+"/identity-view-notice", admin,
		map[string]any{"viewed_at": time.Now().UTC().Format(time.RFC3339)})
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("mail down %d %s", rec.Code, rec.Body.String())
	}
	if got := identityViewMails(coach); len(got) != 0 {
		t.Fatalf("mail recorded while down: %+v", got)
	}
	if !strings.Contains(logs.String(), "identity view notice mail failed") || !strings.Contains(logs.String(), coach.String()) {
		t.Fatalf("failure not logged: %s", logs.String())
	}
}
