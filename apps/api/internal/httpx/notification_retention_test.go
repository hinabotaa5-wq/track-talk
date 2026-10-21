package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

// insertAgedNotifications は、届いてから days 日たった通知を、読んだものと読んでいないものの1つずつ入れる。
func insertAgedNotifications(t *testing.T, recipient uuid.UUID, days int) {
	t.Helper()
	for _, read := range []bool{true, false} {
		execSQLArgs(t, `INSERT INTO notifications (id, recipient_id, kind, title, body, url, created_at, read_at)
			VALUES (gen_random_uuid(), $1, 'message', 'お知らせ', '本文', '/', now() - make_interval(days => $2::int),
			        CASE WHEN $3::bool THEN now() ELSE NULL END)`, recipient, days, read)
	}
}

func countNotificationsOf(t *testing.T, recipient uuid.UUID) (read, unread int) {
	t.Helper()
	queryRowArgs(t, `SELECT count(*) FILTER (WHERE read_at IS NOT NULL), count(*) FILTER (WHERE read_at IS NULL)
		FROM notifications WHERE recipient_id = $1`, []any{recipient}, &read, &unread)
	return read, unread
}

// 通知は、読んだものは届いてから90日、読んでいないものは180日で消す。ベルを開いたときにその人の分を消す。
func TestOldNotificationsAreDeletedWhenListed(t *testing.T) {
	h := setup(t)
	id := uuid.New()
	tok := tokenWithProfile(t, id, "通知の持ち主", true)
	if rec := doJSON(t, h, http.MethodGet, "/v1/me", tok, nil); rec.Code != http.StatusOK {
		t.Fatalf("profile %d", rec.Code)
	}
	insertAgedNotifications(t, id, 10)  // どちらも残す
	insertAgedNotifications(t, id, 100) // 読んだものだけ消す
	insertAgedNotifications(t, id, 200) // どちらも消す
	if rec := doJSON(t, h, http.MethodGet, "/v1/me/notifications", tok, nil); rec.Code != http.StatusOK {
		t.Fatalf("list %d %s", rec.Code, rec.Body.String())
	}
	if read, unread := countNotificationsOf(t, id); read != 1 || unread != 2 {
		t.Fatalf("left read %d unread %d, want 1 and 2", read, unread)
	}
}

// ベルを開かない人の古い通知も、月1回のお知らせのときに消す。
func TestOldNotificationsAreDeletedMonthly(t *testing.T) {
	priv, pub, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	fixed := time.Date(2033, 7, 1, 1, 0, 0, 0, time.UTC)
	h := setupConfig(t, func(cfg *config.Config) {
		cfg.VAPIDPublicKey = pub
		cfg.VAPIDPrivateKey = priv
		cfg.VAPIDSubject = "mailto:test@example.com"
		cfg.ReminderSecret = "monthly-secret"
		cfg.CoachingEnabled = false
		cfg.Clock = func() time.Time { return fixed }
	})
	execSQLArgs(t, `DELETE FROM monthly_reminders WHERE month = $1::date`, "2033-07-01")
	id := uuid.New()
	if rec := doJSON(t, h, http.MethodGet, "/v1/me", tokenWithProfile(t, id, "開かない人", true), nil); rec.Code != http.StatusOK {
		t.Fatalf("profile %d", rec.Code)
	}
	insertAgedNotifications(t, id, 10)
	insertAgedNotifications(t, id, 200)

	req := httptest.NewRequest(http.MethodPost, "/internal/monthly-reminder", nil)
	req.Header.Set("X-Reminder-Secret", "monthly-secret")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("monthly %d %s", rec.Code, rec.Body.String())
	}
	// 10日前の2つと、今月のお知らせが残る。
	if read, unread := countNotificationsOf(t, id); read != 1 || unread != 2 {
		t.Fatalf("left read %d unread %d, want 1 and 2", read, unread)
	}
}
