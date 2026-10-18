package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
)

// テストでは Resend の代わりにこのサーバーへ送り、届いたメールを testOutbox に貯める。
type sentMail struct {
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Text    string   `json:"text"`
}

var (
	testOutboxMu   sync.Mutex
	testOutbox     []sentMail
	testMailOnce   sync.Once
	testMailServer *httptest.Server
)

func mailServerURL() string {
	testMailOnce.Do(func() {
		testMailServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/emails" || r.Header.Get("Authorization") != "Bearer re_test" {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			var m sentMail
			if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
				http.Error(w, "bad json", http.StatusBadRequest)
				return
			}
			testOutboxMu.Lock()
			testOutbox = append(testOutbox, m)
			testOutboxMu.Unlock()
			_, _ = w.Write([]byte(`{"id":"em_test"}`))
		}))
	})
	return testMailServer.URL
}

var guardianLinkPattern = regexp.MustCompile(`/guardian/([A-Za-z0-9_-]{43})`)

// mailsTo は、そのアドレスに届いたメールの数。
func mailsTo(to string) int {
	testOutboxMu.Lock()
	defer testOutboxMu.Unlock()
	n := 0
	for _, m := range testOutbox {
		if len(m.To) == 1 && m.To[0] == to {
			n++
		}
	}
	return n
}

// guardianTokenFromMail は、そのアドレスに最後に届いた同意のメールからリンクのトークンを取り出す。
func guardianTokenFromMail(t *testing.T, to string) string {
	t.Helper()
	testOutboxMu.Lock()
	defer testOutboxMu.Unlock()
	for i := len(testOutbox) - 1; i >= 0; i-- {
		m := testOutbox[i]
		if len(m.To) == 1 && m.To[0] == to {
			match := guardianLinkPattern.FindStringSubmatch(m.Text)
			if match == nil {
				t.Fatalf("no link in mail %q", m.Text)
			}
			return match[1]
		}
	}
	t.Fatalf("no mail to %s", to)
	return ""
}

// issueGuardianLink は、本人が保護者のメールあてにリンクを送り、保護者に届いたトークンを返す。
// 本人へのレスポンスにトークンが入っていないことも確かめる。
func issueGuardianLink(t *testing.T, h http.Handler, tm, email string) string {
	t.Helper()
	rec := doJSON(t, h, http.MethodPost, "/v1/me/guardian-link", tm, map[string]any{"guardian_email": email})
	if rec.Code != http.StatusCreated {
		t.Fatalf("link %d %s", rec.Code, rec.Body.String())
	}
	tok := guardianTokenFromMail(t, email)
	if strings.Contains(rec.Body.String(), tok) || strings.Contains(rec.Body.String(), `"token"`) {
		t.Fatalf("token returned to minor: %s", rec.Body.String())
	}
	return tok
}

// guardianEmail は、テストごとに別の保護者のアドレスを作る。
func guardianEmail() string {
	return "parent-" + uuid.NewString()[:8] + "@example.com"
}
