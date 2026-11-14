package httpx_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

func TestNewCoachAnnouncementDisabledOnDuty(t *testing.T) {
	fakeCoachingCloudinary(t)
	gate := newPushGate(t, 0)
	h := setupPushVAPID(t, gate, func(cfg *config.Config) {
		cfg.CoachingNewAcceptance = true
		cfg.CoachingCoachList = true
	})
	admin := adminToken(t, h)
	athlete := uuid.New()
	athleteToken := token(t, athlete)
	subscribeFakePush(t, h, athleteToken, gate.server.URL+"/athlete")

	base := gate.sent.Load()
	coach, coachToken := onDutyCoachTo(t, h, admin, gate.server.URL+"/coach")
	var onDuty, staff bool
	queryRowArgs(t, `SELECT c.on_duty, p.is_admin FROM coach_profiles c JOIN profiles p ON p.id=c.profile_id WHERE c.profile_id=$1`, []any{coach}, &onDuty, &staff)
	if !onDuty || staff {
		t.Fatalf("coach on duty=%v, staff=%v", onDuty, staff)
	}
	nominees := doJSON(t, h, http.MethodGet, "/v1/coaching/nominees?event=100m", athleteToken, nil)
	if nominees.Code != http.StatusOK || !strings.Contains(nominees.Body.String(), coach.String()) {
		t.Fatalf("coach missing from nominees: %d %s", nominees.Code, nominees.Body.String())
	}
	assertNoNewCoachAnnouncement(t, coach)
	// 従来のコーチ本人宛ての接続テストと受付開始通知だけが届く。
	if got := gate.sent.Load() - base; got != 2 {
		t.Fatalf("push deliveries=%d, want 2", got)
	}

	for _, onDuty := range []bool{false, true} {
		if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me", coachToken, map[string]any{"on_duty": onDuty}); rec.Code != http.StatusOK {
			t.Fatalf("set on duty=%v: %d %s", onDuty, rec.Code, rec.Body.String())
		}
	}
	assertNoNewCoachAnnouncement(t, coach)
}

func assertNoNewCoachAnnouncement(t *testing.T, coach uuid.UUID) {
	t.Helper()
	id := uuid.NewSHA1(uuid.NameSpaceURL, []byte("new-coach:"+coach.String()))
	var events, announcements, bells, pushes int
	queryRowArgs(t, `SELECT
		(SELECT count(*) FROM new_coach_announcements WHERE coach_id=$1),
		(SELECT count(*) FROM announcements WHERE id=$2),
		(SELECT count(*) FROM notifications WHERE announcement_id=$2),
		(SELECT count(*) FROM push_outbox WHERE dedup_key LIKE $3)`,
		[]any{coach, id, "announcement:" + id.String() + ":%"}, &events, &announcements, &bells, &pushes)
	if events != 0 || announcements != 0 || bells != 0 || pushes != 0 {
		t.Fatalf("auto announcement created: events=%d announcements=%d bells=%d pushes=%d", events, announcements, bells, pushes)
	}
}

func TestNewCoachAnnouncementPendingPushIsNotRetried(t *testing.T) {
	gate := newPushGate(t, 0)
	h := setupPushVAPID(t, gate, func(cfg *config.Config) { cfg.CoachingTickSecret = "tick-secret" })
	admin := adminToken(t, h)
	athlete := uuid.New()
	subscribeFakePush(t, h, token(t, athlete), gate.server.URL+"/athlete")

	const title = "新しいコーチが加わりました"
	const autoBody = "テストコーチが動画添削のコーチに加わりました"
	autoID := uuid.NewSHA1(uuid.NameSpaceURL, []byte("new-coach:"+uuid.NewString()))
	execSQLArgs(t, `INSERT INTO announcements (id, title, body, url, in_app_count) VALUES ($1, $2, $3, '/coaching/new', 1)`, autoID, title, autoBody)
	execSQLArgs(t, `INSERT INTO notifications (id, recipient_id, kind, title, body, url, announcement_id)
		VALUES ($1, $2, 'message', $3, $4, '/coaching/new', $5)`, uuid.New(), athlete, title, autoBody, autoID)
	dedup := "announcement:" + autoID.String() + ":" + athlete.String()
	execSQLArgs(t, `INSERT INTO push_outbox (id, profile_id, title, body, url, tag, category, ttl_seconds, dedup_key, expires_at)
		VALUES ($1, $2, $3, $4, '/coaching/new', $5, 'announcement', 3600, $6, now()+interval '1 hour')`,
		uuid.New(), athlete, title, autoBody, "announcement-"+autoID.String(), dedup)

	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/announcements", admin, map[string]any{
		"id": autoID, "title": title, "body": autoBody, "url": "/coaching/new", "confirm": "送信",
	}); rec.Code != http.StatusConflict {
		t.Fatalf("resume retired announcement: %d %s", rec.Code, rec.Body.String())
	}
	if rec := tickWithSecret(t, h, "tick-secret"); rec.Code != http.StatusOK {
		t.Fatalf("tick %d %s", rec.Code, rec.Body.String())
	}
	if got := gate.sent.Load(); got != 0 {
		t.Fatalf("retired push sent %d times", got)
	}
	if status, _ := outboxStatus(t, dedup); status != "discarded" {
		t.Fatalf("retired push status=%s", status)
	}
	var bells int
	queryRowArgs(t, `SELECT count(*) FROM notifications WHERE announcement_id=$1`, []any{autoID}, &bells)
	if bells != 1 {
		t.Fatalf("existing bells=%d, want 1", bells)
	}

	// 同じ見出しの手動配信は、一時障害後も tick から再送できる。
	manualID := uuid.NewSHA1(uuid.NameSpaceURL, []byte("manual:"+uuid.NewString()))
	gate.failNext(1)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/announcements", admin, map[string]any{
		"id": manualID, "title": title, "body": "運営からのお知らせです", "url": "/coaching/new", "confirm": "送信",
	}); rec.Code != http.StatusOK {
		t.Fatalf("manual announcement: %d %s", rec.Code, rec.Body.String())
	}
	manualDedup := "announcement:" + manualID.String() + ":" + athlete.String()
	if status, _ := outboxStatus(t, manualDedup); status != "pending" {
		t.Fatalf("manual push status=%s, want pending", status)
	}
	execSQLArgs(t, `UPDATE push_outbox SET next_attempt_at=now() WHERE dedup_key=$1`, manualDedup)
	if rec := tickWithSecret(t, h, "tick-secret"); rec.Code != http.StatusOK {
		t.Fatalf("tick %d %s", rec.Code, rec.Body.String())
	}
	if status, _ := outboxStatus(t, manualDedup); status != "sent" {
		t.Fatalf("manual push status=%s, want sent", status)
	}
	if got := gate.sent.Load(); got != 1 {
		t.Fatalf("push deliveries=%d, want 1", got)
	}
}
