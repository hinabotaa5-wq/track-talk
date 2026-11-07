package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

// 新しいコーチの初回候補入りで、全ユーザーへ自動通知が1回だけ届く。
// コーチ本人を除き、announcement の設定に従う。受付の再開・再承認・編集では再送しない。
func TestNewCoachAnnouncementSentOnce(t *testing.T) {
	fakeCoachingCloudinary(t)
	gate := newPushGate(t, 0)
	h := setupPushVAPID(t, gate, func(cfg *config.Config) {
		cfg.CoachingNewAcceptance = true
		cfg.CoachingCoachList = true
	})
	tadmin := adminToken(t, h)

	a := uuid.New()
	ta := token(t, a)
	subscribeFakePush(t, h, ta, gate.server.URL+"/a")
	b := uuid.New()
	tb := token(t, b)
	subscribeFakePush(t, h, tb, gate.server.URL+"/b")
	if rec := doJSON(t, h, http.MethodPut, "/v1/me/notification-settings/announcement", tb, map[string]any{"in_app": true, "push": false}); rec.Code != http.StatusOK {
		t.Fatalf("mute push %d %s", rec.Code, rec.Body.String())
	}
	c := uuid.New()
	tc := token(t, c)
	if rec := doJSON(t, h, http.MethodPut, "/v1/me/notification-settings/announcement", tc, map[string]any{"in_app": false, "push": false}); rec.Code != http.StatusOK {
		t.Fatalf("mute all %d %s", rec.Code, rec.Body.String())
	}

	coach := uuid.New()
	tcoach := token(t, coach)
	body := coachApplicationBody()
	body["tier"] = "coach"
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", tcoach, body); rec.Code != http.StatusOK {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
	withCoachPhoto(t, coach)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+coach.String()+"/approve", tadmin, nil); rec.Code != http.StatusOK {
		t.Fatalf("approve %d %s", rec.Code, rec.Body.String())
	}
	execSQLArgs(t, `UPDATE coach_profiles SET stripe_account_id = $2, payouts_enabled = true WHERE profile_id = $1`, coach, "acct-"+coach.String())
	subscribeFakePush(t, h, tcoach, gate.server.URL+"/coach")

	base := gate.sent.Load()
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me", tcoach, map[string]any{"on_duty": true}); rec.Code != http.StatusOK {
		t.Fatalf("on duty %d %s", rec.Code, rec.Body.String())
	}

	annID := uuid.NewSHA1(uuid.NameSpaceURL, []byte("new-coach:"+coach.String())).String()
	var title, url string
	queryRowArgs(t, `SELECT title, url FROM announcements WHERE id=$1`, []any{annID}, &title, &url)
	if title != "新しいコーチが加わりました" {
		t.Fatalf("title %q", title)
	}
	if url != "/coaches/"+coach.String() {
		t.Fatalf("url %q", url)
	}
	var bellBody string
	queryRowArgs(t, `SELECT body FROM announcements WHERE id=$1`, []any{annID}, &bellBody)
	if !strings.Contains(bellBody, "テストコーチ") || !strings.Contains(bellBody, "動画添削のコーチに加わりました") {
		t.Fatalf("body %q", bellBody)
	}
	countBell := func(id uuid.UUID) int {
		var n int
		queryRowArgs(t, `SELECT count(*) FROM notifications WHERE announcement_id=$1 AND recipient_id=$2`, []any{annID, id}, &n)
		return n
	}
	var adminID uuid.UUID
	queryRowArgs(t, `SELECT id FROM profiles WHERE is_admin ORDER BY created_at LIMIT 1`, nil, &adminID)
	if got := countBell(a); got != 1 {
		t.Fatalf("bell a=%d", got)
	}
	if got := countBell(b); got != 1 {
		t.Fatalf("bell b=%d", got)
	}
	if got := countBell(c); got != 0 {
		t.Fatalf("bell c=%d", got)
	}
	if got := countBell(coach); got != 0 {
		t.Fatalf("bell coach=%d", got)
	}
	if got := countBell(adminID); got != 1 {
		t.Fatalf("bell admin=%d", got)
	}
	var outbox int
	queryRowArgs(t, `SELECT count(*) FROM push_outbox WHERE dedup_key LIKE $1`, []any{"announcement:" + annID + ":%"}, &outbox)
	if outbox != 1 {
		t.Fatalf("outbox=%d", outbox)
	}
	var status string
	queryRowArgs(t, `SELECT status FROM push_outbox WHERE dedup_key=$1`, []any{"announcement:" + annID + ":" + a.String()}, &status)
	if status != "sent" {
		t.Fatalf("push status %q", status)
	}
	// 受付開始の疎通確認・本人通知と合わせ、新規コーチの分は購読ありで設定オンの a の1通だけ。
	if got := gate.sent.Load() - base; got != 3 {
		t.Fatalf("sent=%d", got)
	}

	// 受付の停止→再開・編集では再送しない。
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me", tcoach, map[string]any{"on_duty": false}); rec.Code != http.StatusOK {
		t.Fatalf("off duty %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me", tcoach, map[string]any{"on_duty": true}); rec.Code != http.StatusOK {
		t.Fatalf("re on duty %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me", tcoach, map[string]any{"daily_cap": 3}); rec.Code != http.StatusOK {
		t.Fatalf("edit %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/coaches/"+coach.String()+"/suspend", tadmin, nil); rec.Code != http.StatusOK {
		t.Fatalf("suspend %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/coaches/"+coach.String()+"/resume", tadmin, nil); rec.Code != http.StatusOK {
		t.Fatalf("resume %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me", tcoach, map[string]any{"on_duty": true}); rec.Code != http.StatusOK {
		t.Fatalf("on duty again %d %s", rec.Code, rec.Body.String())
	}
	if got := countBell(a); got != 1 {
		t.Fatalf("bell a after=%d", got)
	}
	queryRowArgs(t, `SELECT count(*) FROM push_outbox WHERE dedup_key LIKE $1`, []any{"announcement:" + annID + ":%"}, &outbox)
	if outbox != 1 {
		t.Fatalf("outbox after=%d", outbox)
	}
}

// 受付が閉じている間の候補入りは送らず、開放後にまとめて送らない。
func TestNewCoachAnnouncementSuppressedWhenAcceptanceClosed(t *testing.T) {
	fakeCoachingCloudinary(t)
	gate := newPushGate(t, 0)
	h := setupPushVAPID(t, gate, func(cfg *config.Config) {
		cfg.CoachingNewAcceptance = false
		cfg.CoachingCoachList = false
	})
	tadmin := adminToken(t, h)

	a := uuid.New()
	ta := token(t, a)
	subscribeFakePush(t, h, ta, gate.server.URL+"/a")

	coach := uuid.New()
	tcoach := token(t, coach)
	body := coachApplicationBody()
	body["tier"] = "coach"
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", tcoach, body); rec.Code != http.StatusOK {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
	withCoachPhoto(t, coach)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+coach.String()+"/approve", tadmin, nil); rec.Code != http.StatusOK {
		t.Fatalf("approve %d %s", rec.Code, rec.Body.String())
	}
	execSQLArgs(t, `UPDATE coach_profiles SET stripe_account_id = $2, payouts_enabled = true WHERE profile_id = $1`, coach, "acct-"+coach.String())
	subscribeFakePush(t, h, tcoach, gate.server.URL+"/coach")
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me", tcoach, map[string]any{"on_duty": true}); rec.Code != http.StatusOK {
		t.Fatalf("on duty %d %s", rec.Code, rec.Body.String())
	}

	annID := uuid.NewSHA1(uuid.NameSpaceURL, []byte("new-coach:"+coach.String())).String()
	var bells int
	queryRowArgs(t, `SELECT count(*) FROM notifications WHERE announcement_id=$1`, []any{annID}, &bells)
	if bells != 0 {
		t.Fatalf("bells=%d", bells)
	}
	var rows int
	queryRowArgs(t, `SELECT count(*) FROM announcements WHERE id=$1`, []any{annID}, &rows)
	if rows != 0 {
		t.Fatalf("announcements=%d", rows)
	}
	// 発火済み記録は残り、受付の切り直しでは送らない。
	var fired int
	queryRowArgs(t, `SELECT count(*) FROM new_coach_announcements WHERE coach_id=$1`, []any{coach}, &fired)
	if fired != 1 {
		t.Fatalf("fired=%d", fired)
	}
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me", tcoach, map[string]any{"on_duty": false}); rec.Code != http.StatusOK {
		t.Fatalf("off duty %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me", tcoach, map[string]any{"on_duty": true}); rec.Code != http.StatusOK {
		t.Fatalf("on duty again %d %s", rec.Code, rec.Body.String())
	}
	queryRowArgs(t, `SELECT count(*) FROM notifications WHERE announcement_id=$1`, []any{annID}, &bells)
	if bells != 0 {
		t.Fatalf("bells after=%d", bells)
	}
}

// スタッフ権限のアカウントの候補入りは告知しない。受付の確定自体は通る。
func TestNewCoachAnnouncementStaffExcluded(t *testing.T) {
	fakeCoachingCloudinary(t)
	gate := newPushGate(t, 0)
	h := setupPushVAPID(t, gate, func(cfg *config.Config) {
		cfg.CoachingNewAcceptance = true
		cfg.CoachingCoachList = false
	})
	tadmin := adminToken(t, h)

	coach := uuid.New()
	tcoach := token(t, coach)
	body := coachApplicationBody()
	body["tier"] = "coach"
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", tcoach, body); rec.Code != http.StatusOK {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
	withCoachPhoto(t, coach)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+coach.String()+"/approve", tadmin, nil); rec.Code != http.StatusOK {
		t.Fatalf("approve %d %s", rec.Code, rec.Body.String())
	}
	setAdmin(t, coach)
	execSQLArgs(t, `UPDATE coach_profiles SET stripe_account_id = $2, payouts_enabled = true WHERE profile_id = $1`, coach, "acct-"+coach.String())
	subscribeFakePush(t, h, tcoach, gate.server.URL+"/coach")
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me", tcoach, map[string]any{"on_duty": true}); rec.Code != http.StatusOK {
		t.Fatalf("on duty %d %s", rec.Code, rec.Body.String())
	}

	annID := uuid.NewSHA1(uuid.NameSpaceURL, []byte("new-coach:"+coach.String())).String()
	var bells int
	queryRowArgs(t, `SELECT count(*) FROM notifications WHERE announcement_id=$1`, []any{annID}, &bells)
	if bells != 0 {
		t.Fatalf("bells=%d", bells)
	}
	var fired int
	queryRowArgs(t, `SELECT count(*) FROM new_coach_announcements WHERE coach_id=$1`, []any{coach}, &fired)
	if fired != 1 {
		t.Fatalf("fired=%d", fired)
	}
}

// 一覧が閉じている間の遷移先は指名申し込み。「準備中」には着地させない。
// 通知の送信に失敗しても受付開始は確定し、送り直しで二重送信にならない。
func TestNewCoachAnnouncementURLAndRetry(t *testing.T) {
	fakeCoachingCloudinary(t)
	// 受付開始の疎通確認（コーチ宛て）だけ通し、利用者宛ての配信だけ失敗させる。
	var mu sync.Mutex
	failUser := true
	var sentUser atomic.Int64
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		fail := failUser && r.URL.Path == "/a"
		mu.Unlock()
		if fail {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if r.URL.Path == "/a" {
			sentUser.Add(1)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(srv.Close)
	priv, pub, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	h := setupConfig(t, func(cfg *config.Config) {
		cfg.VAPIDPublicKey = pub
		cfg.VAPIDPrivateKey = priv
		cfg.VAPIDSubject = "mailto:test@example.com"
		cfg.PushHTTP = srv.Client()
		cfg.CoachingNewAcceptance = true
		cfg.CoachingCoachList = false
		cfg.CoachingTickSecret = "tick-secret"
	})
	tadmin := adminToken(t, h)

	a := uuid.New()
	ta := token(t, a)
	subscribeFakePush(t, h, ta, srv.URL+"/a")

	coach := uuid.New()
	tcoach := token(t, coach)
	body := coachApplicationBody()
	body["tier"] = "coach"
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", tcoach, body); rec.Code != http.StatusOK {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
	withCoachPhoto(t, coach)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+coach.String()+"/approve", tadmin, nil); rec.Code != http.StatusOK {
		t.Fatalf("approve %d %s", rec.Code, rec.Body.String())
	}
	execSQLArgs(t, `UPDATE coach_profiles SET stripe_account_id = $2, payouts_enabled = true WHERE profile_id = $1`, coach, "acct-"+coach.String())
	subscribeFakePush(t, h, tcoach, srv.URL+"/coach")
	// 送信がすべて失敗しても、受付開始自体は確定する。
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me", tcoach, map[string]any{"on_duty": true}); rec.Code != http.StatusOK {
		t.Fatalf("on duty %d %s", rec.Code, rec.Body.String())
	}

	annID := uuid.NewSHA1(uuid.NameSpaceURL, []byte("new-coach:"+coach.String())).String()
	var url string
	queryRowArgs(t, `SELECT url FROM announcements WHERE id=$1`, []any{annID}, &url)
	if url != "/coaching/new" {
		t.Fatalf("url %q", url)
	}
	var bells int
	queryRowArgs(t, `SELECT count(*) FROM notifications WHERE announcement_id=$1 AND recipient_id=$2`, []any{annID, a}, &bells)
	if bells != 1 {
		t.Fatalf("bells=%d", bells)
	}
	dedup := "announcement:" + annID + ":" + a.String()
	if status, _ := outboxStatus(t, dedup); status != "pending" {
		t.Fatalf("status %s", status)
	}
	// 送り直しの時刻が来るまで tick しても送らない。
	if rec := tickWithSecret(t, h, "tick-secret"); rec.Code != http.StatusOK {
		t.Fatalf("tick %d %s", rec.Code, rec.Body.String())
	}
	if status, _ := outboxStatus(t, dedup); status != "pending" {
		t.Fatalf("status after tick %s", status)
	}
	mu.Lock()
	failUser = false
	mu.Unlock()
	execSQLArgs(t, `UPDATE push_outbox SET next_attempt_at=now() WHERE dedup_key=$1`, dedup)
	base := sentUser.Load()
	if rec := tickWithSecret(t, h, "tick-secret"); rec.Code != http.StatusOK {
		t.Fatalf("tick %d %s", rec.Code, rec.Body.String())
	}
	if got := sentUser.Load() - base; got != 1 {
		t.Fatalf("resent=%d", got)
	}
	if status, _ := outboxStatus(t, dedup); status != "sent" {
		t.Fatalf("status %s", status)
	}
	// 送り直してもベルは増えない。
	execSQLArgs(t, `UPDATE push_outbox SET next_attempt_at=now() WHERE dedup_key=$1`, dedup)
	if rec := tickWithSecret(t, h, "tick-secret"); rec.Code != http.StatusOK {
		t.Fatalf("tick %d %s", rec.Code, rec.Body.String())
	}
	queryRowArgs(t, `SELECT count(*) FROM notifications WHERE announcement_id=$1 AND recipient_id=$2`, []any{annID, a}, &bells)
	if bells != 1 {
		t.Fatalf("bells after=%d", bells)
	}
}
