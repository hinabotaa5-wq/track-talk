package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

// pushGate は、失敗と成功を切り替えられるテスト用のプッシュサーバー。
// 失敗の回数と成功の回数を数え、送り直しと二重送信を確かめる。
type pushGate struct {
	mu       sync.Mutex
	failLeft int
	failed   atomic.Int64
	sent     atomic.Int64
	server   *httptest.Server
}

func newPushGate(t *testing.T, failLeft int) *pushGate {
	t.Helper()
	g := &pushGate{failLeft: failLeft}
	g.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.mu.Lock()
		fail := g.failLeft > 0
		if fail {
			g.failLeft--
		}
		g.mu.Unlock()
		if fail {
			g.failed.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		g.sent.Add(1)
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(g.server.Close)
	return g
}

func (g *pushGate) recover() {
	g.mu.Lock()
	g.failLeft = 0
	g.mu.Unlock()
}

func (g *pushGate) failNext(n int) {
	g.mu.Lock()
	g.failLeft = n
	g.mu.Unlock()
}

func setupPushVAPID(t *testing.T, gate *pushGate, mutate func(*config.Config)) http.Handler {
	t.Helper()
	priv, pub, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	return setupConfig(t, func(cfg *config.Config) {
		cfg.VAPIDPublicKey = pub
		cfg.VAPIDPrivateKey = priv
		cfg.VAPIDSubject = "mailto:test@example.com"
		cfg.PushHTTP = gate.server.Client()
		if mutate != nil {
			mutate(cfg)
		}
	})
}

// subscribeFakePush は、その人の購読をテスト用サーバーに向け直す。
func subscribeFakePush(t *testing.T, h http.Handler, tok, endpoint string) {
	t.Helper()
	p256dh, auth := testPushKeys(t)
	if rec := doJSON(t, h, http.MethodPost, "/v1/me/push-subscriptions", tok, map[string]any{
		"endpoint": endpoint,
		"keys":     map[string]string{"p256dh": p256dh, "auth": auth},
	}); rec.Code != http.StatusNoContent {
		t.Fatalf("subscribe %d %s", rec.Code, rec.Body.String())
	}
}

func outboxStatus(t *testing.T, dedup string) (string, int) {
	t.Helper()
	var status string
	var attempts int
	queryRowArgs(t, `SELECT status, attempts FROM push_outbox WHERE dedup_key=$1`, []any{dedup}, &status, &attempts)
	return status, attempts
}

func tickWithSecret(t *testing.T, h http.Handler, secret string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/internal/coaching/tick", nil)
	req.Header.Set("X-Coaching-Tick-Secret", secret)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// onDutyCoachTo は、受付開始の疎通確認が通る宛先で受付を始める。テスト用サーバーに向ける。
func onDutyCoachTo(t *testing.T, h http.Handler, tadmin, endpoint string) (uuid.UUID, string) {
	t.Helper()
	id := uuid.New()
	tc := token(t, id)
	body := coachApplicationBody()
	body["tier"] = "coach"
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", tc, body); rec.Code != http.StatusOK {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
	withCoachPhoto(t, id)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+id.String()+"/approve", tadmin, nil); rec.Code != http.StatusOK {
		t.Fatalf("approve %d", rec.Code)
	}
	p256dh, auth := testPushKeys(t)
	execSQLArgs(t, `INSERT INTO push_subscriptions (id, profile_id, endpoint, p256dh, auth) VALUES ($1, $2, $3, $4, $5)`,
		uuid.New(), id, endpoint, p256dh, auth)
	execSQLArgs(t, `UPDATE coach_profiles SET stripe_account_id = $2, payouts_enabled = true WHERE profile_id = $1`, id, "acct_"+id.String())
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me", tc, map[string]any{"on_duty": true}); rec.Code != http.StatusOK {
		t.Fatalf("on duty %d %s", rec.Code, rec.Body.String())
	}
	return id, tc
}

// 新規打診の Push が一時障害で送れなくても、tick の送り直しで1回だけ届く。期限切れの打診は送り直さない。
func TestReviewOfferPushResentAfterFailure(t *testing.T) {
	fakeCoachingCloudinary(t)
	gate := newPushGate(t, 0)
	h := setupPushVAPID(t, gate, func(cfg *config.Config) { cfg.CoachingTickSecret = "tick-secret" })
	tadmin := adminToken(t, h)
	_, _ = onDutyCoachTo(t, h, tadmin, gate.server.URL+"/coach")
	_, tu := demoRequester(t, h, tadmin)
	gate.failNext(1)
	base := gate.sent.Load()

	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, reviewBodyFor(t, h, tu))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var dedup string
	queryRowArgs(t, `SELECT dedup_key FROM push_outbox WHERE dedup_key LIKE 'review_offer:%' ORDER BY created_at DESC LIMIT 1`, nil, &dedup)
	if !strings.HasPrefix(dedup, "review_offer:") {
		t.Fatalf("dedup %q", dedup)
	}
	// その場の送信は失敗し、待ちが残る。
	if status, _ := outboxStatus(t, dedup); status != "pending" {
		t.Fatalf("status %s", status)
	}
	if got := gate.sent.Load(); got != base {
		t.Fatalf("sent %d, want %d", got, base)
	}
	// 送り直しの時刻が来るまで tick しても送らない。
	if rec := tickWithSecret(t, h, "tick-secret"); rec.Code != http.StatusOK {
		t.Fatalf("tick %d %s", rec.Code, rec.Body.String())
	}
	if got := gate.sent.Load(); got != base {
		t.Fatalf("resent too early %d", got)
	}
	execSQLArgs(t, `UPDATE push_outbox SET next_attempt_at=now() WHERE dedup_key=$1`, dedup)
	if rec := tickWithSecret(t, h, "tick-secret"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"push_resent":1`) {
		t.Fatalf("tick %d %s", rec.Code, rec.Body.String())
	}
	if got := gate.sent.Load(); got != base+1 {
		t.Fatalf("sent %d, want %d", got, base+1)
	}
	if status, _ := outboxStatus(t, dedup); status != "sent" {
		t.Fatalf("status %s", status)
	}
	// 送り終えた待ちは、tick を回しても送り直さない。
	if rec := tickWithSecret(t, h, "tick-secret"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"push_resent":0`) {
		t.Fatalf("second tick %d %s", rec.Code, rec.Body.String())
	}
	if got := gate.sent.Load(); got != base+1 {
		t.Fatalf("duplicate resend %d", got)
	}
}

// 有効期限を過ぎた打診の Push は、回復しても送り直さない。
func TestReviewOfferPushNotResentAfterExpiry(t *testing.T) {
	fakeCoachingCloudinary(t)
	gate := newPushGate(t, 0)
	h := setupPushVAPID(t, gate, func(cfg *config.Config) { cfg.CoachingTickSecret = "tick-secret" })
	tadmin := adminToken(t, h)
	_, _ = onDutyCoachTo(t, h, tadmin, gate.server.URL+"/coach")
	_, tu := demoRequester(t, h, tadmin)
	gate.failNext(1000)
	base := gate.sent.Load()

	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, reviewBodyFor(t, h, tu))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var dedup string
	queryRowArgs(t, `SELECT dedup_key FROM push_outbox ORDER BY created_at DESC LIMIT 1`, nil, &dedup)
	execSQLArgs(t, `UPDATE push_outbox SET expires_at=now()-interval '1 minute', next_attempt_at=now() WHERE dedup_key=$1`, dedup)
	gate.recover()
	if rec := tickWithSecret(t, h, "tick-secret"); rec.Code != http.StatusOK {
		t.Fatalf("tick %d %s", rec.Code, rec.Body.String())
	}
	if got := gate.sent.Load(); got != base {
		t.Fatalf("expired resent %d", got)
	}
	if status, _ := outboxStatus(t, dedup); status != "expired" {
		t.Fatalf("status %s", status)
	}
}

// 返却の Push が一時障害で送れなくても、tick の送り直しで1回だけ届く。
func TestReviewReturnPushResentAfterFailure(t *testing.T) {
	fakeCoachingCloudinary(t)
	gate := newPushGate(t, 0)
	h := setupPushVAPID(t, gate, func(cfg *config.Config) { cfg.CoachingTickSecret = "tick-secret" })
	tadmin := adminToken(t, h)
	_, tc := onDutyCoachTo(t, h, tadmin, gate.server.URL+"/coach")
	_, tu := demoRequester(t, h, tadmin)
	subscribeFakePush(t, h, tu, gate.server.URL+"/requester")
	id := acceptedReview(t, h, tadmin, tc, tu)
	gate.failNext(1)
	base := gate.sent.Load()
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/response", tc, responseBody()); rec.Code != http.StatusOK {
		t.Fatalf("respond %d %s", rec.Code, rec.Body.String())
	}
	dedup := "review_returned:" + id
	if status, _ := outboxStatus(t, dedup); status != "pending" {
		t.Fatalf("status %s", status)
	}
	if got := gate.sent.Load(); got != base {
		t.Fatalf("sent %d, want %d", got, base)
	}
	execSQLArgs(t, `UPDATE push_outbox SET next_attempt_at=now() WHERE dedup_key=$1`, dedup)
	if rec := tickWithSecret(t, h, "tick-secret"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"push_resent":1`) {
		t.Fatalf("tick %d %s", rec.Code, rec.Body.String())
	}
	if got := gate.sent.Load(); got != base+1 {
		t.Fatalf("sent %d, want %d", got, base+1)
	}
	if rec := tickWithSecret(t, h, "tick-secret"); rec.Code != http.StatusOK {
		t.Fatalf("second tick %d %s", rec.Code, rec.Body.String())
	}
	if got := gate.sent.Load(); got != base+1 {
		t.Fatalf("duplicate resend %d", got)
	}
}

// 購読の有無・送信の成否は名前で追える。購読がなければ送らず、購読が戻れば次の送り直しで届く。
func TestPushOutboxRecordsNoSubscription(t *testing.T) {
	fakeCoachingCloudinary(t)
	gate := newPushGate(t, 0)
	h := setupPushVAPID(t, gate, func(cfg *config.Config) { cfg.CoachingTickSecret = "tick-secret" })
	tadmin := adminToken(t, h)
	_, _ = onDutyCoachTo(t, h, tadmin, gate.server.URL+"/coach")
	_, tu := demoRequester(t, h, tadmin)
	// コーチの購読を消してから打診する（購読消失の再現）。
	base := gate.sent.Load()
	execSQLArgs(t, `DELETE FROM push_subscriptions`)
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, reviewBodyFor(t, h, tu))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var dedup string
	queryRowArgs(t, `SELECT dedup_key FROM push_outbox ORDER BY created_at DESC LIMIT 1`, nil, &dedup)
	execSQLArgs(t, `UPDATE push_outbox SET next_attempt_at=now() WHERE dedup_key=$1`, dedup)
	// 購読がないまま送り直しても送らず、理由が残る。
	execSQLArgs(t, `DELETE FROM push_subscriptions`)
	if rec := tickWithSecret(t, h, "tick-secret"); rec.Code != http.StatusOK {
		t.Fatalf("tick %d %s", rec.Code, rec.Body.String())
	}
	var last string
	queryRowArgs(t, `SELECT last_result FROM push_outbox WHERE dedup_key=$1`, []any{dedup}, &last)
	if last != "no_subscription" {
		t.Fatalf("last_result %s", last)
	}
	if got := gate.sent.Load(); got != base {
		t.Fatalf("sent %d, want %d", got, base)
	}
}

// 月次お知らせが途中で止まっても、呼び直しで未送信分だけが届く。ベルは二重に入らない。
func TestMonthlyReminderResumesAfterInterruption(t *testing.T) {
	gate := newPushGate(t, 1000)
	fixed := time.Date(2031, 5, 1, 1, 0, 0, 0, time.UTC)
	h := setupPushVAPID(t, gate, func(cfg *config.Config) {
		cfg.ReminderSecret = "test-reminder-secret"
		cfg.CoachingEnabled = false
		cfg.Clock = func() time.Time { return fixed }
	})
	a := mustTokenWithSubscription(t, h, "到達する人", gate.server.URL+"/a")
	mustTokenWithSubscription(t, h, "あとで直る人", gate.server.URL+"/b")
	muted := mustTokenWithSubscription(t, h, "切っている人", gate.server.URL+"/muted")
	if rec := doJSON(t, h, http.MethodPut, "/v1/me/notification-settings/monthly", muted,
		map[string]any{"in_app": true, "push": false}); rec.Code != http.StatusOK {
		t.Fatalf("settings %d %s", rec.Code, rec.Body.String())
	}
	month := time.Date(2031, 5, 1, 0, 0, 0, 0, time.UTC)
	execSQLArgs(t, `DELETE FROM monthly_reminders WHERE month=$1`, month)

	// 1回目は全滅する。ベルは入り、送り直しの待ちが残り、終わっていない。
	rec := postMonthlyReminder(t, h)
	if rec.Code != http.StatusOK {
		t.Fatalf("first %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"already_sent":false`) {
		t.Fatalf("first %s", rec.Body.String())
	}
	var finished *time.Time
	var recipients int
	queryRowArgs(t, `SELECT finished_at, recipients FROM monthly_reminders WHERE month=$1`, []any{month}, &finished, &recipients)
	if finished != nil {
		t.Fatal("finished despite failures")
	}
	var pending int
	queryRowArgs(t, `SELECT count(*) FROM push_outbox WHERE status='pending' AND dedup_key LIKE 'monthly:2031-05:%'`, nil, &pending)
	if pending != 2 {
		t.Fatalf("pending %d, want 2", pending)
	}
	items, _ := listNotifications(t, h, a)
	if len(items) != 1 {
		t.Fatalf("a bells %d", len(items))
	}

	// 回復後に呼び直すと、未送信の2人にだけ届き、ベルは増えない。
	gate.recover()
	execSQLArgs(t, `UPDATE monthly_reminders SET started_at=now()-interval '1 hour' WHERE month=$1`, month)
	execSQLArgs(t, `UPDATE push_outbox SET next_attempt_at=now()`)
	rec = postMonthlyReminder(t, h)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"recipients":2`) {
		t.Fatalf("second %d %s", rec.Code, rec.Body.String())
	}
	if got := gate.sent.Load(); got != 2 {
		t.Fatalf("sent %d, want 2", got)
	}
	items, _ = listNotifications(t, h, a)
	if len(items) != 1 {
		t.Fatalf("a bells after resume %d", len(items))
	}
	queryRowArgs(t, `SELECT finished_at, recipients FROM monthly_reminders WHERE month=$1`, []any{month}, &finished, &recipients)
	if finished == nil || recipients != 2 {
		t.Fatalf("finished=%v recipients=%d", finished, recipients)
	}
}

func mustTokenWithSubscription(t *testing.T, h http.Handler, nickname, endpoint string) string {
	t.Helper()
	tok := tokenWithProfile(t, uuid.New(), nickname, true)
	subscribeFakePush(t, h, tok, endpoint)
	return tok
}

// 運営お知らせが途中で止まっても、同じ id の送り直しで未送信分だけが届く。
func TestAnnouncementResumesAfterInterruption(t *testing.T) {
	gate := newPushGate(t, 1000)
	h := setupPushVAPID(t, gate, nil)
	admin := adminToken(t, h)
	mustTokenWithSubscription(t, h, "届く人", gate.server.URL+"/x")
	mustTokenWithSubscription(t, h, "あとで直る人", gate.server.URL+"/y")
	id := uuid.New().String()
	body := map[string]any{"title": "送り直しの確認", "body": "本文です", "url": "/x", "confirm": "送信"}
	rec := doJSON(t, h, http.MethodPost, "/v1/admin/announcements", admin, withID(body, id))
	if rec.Code != http.StatusOK {
		t.Fatalf("first %d %s", rec.Code, rec.Body.String())
	}
	var finishedAt *time.Time
	queryRowArgs(t, `SELECT finished_at FROM announcements WHERE id=$1`, []any{id}, &finishedAt)
	if finishedAt != nil {
		t.Fatal("finished despite failures")
	}
	var firstInApp int
	queryRowArgs(t, `SELECT count(*) FROM notifications WHERE announcement_id=$1`, []any{id}, &firstInApp)
	if firstInApp == 0 {
		t.Fatal("no bells")
	}
	// 回復後に同じ内容で送り直すと、未送信分だけ届き、ベルは増えない。
	gate.recover()
	execSQLArgs(t, `UPDATE push_outbox SET next_attempt_at=now()`)
	rec = doJSON(t, h, http.MethodPost, "/v1/admin/announcements", admin, withID(body, id))
	if rec.Code != http.StatusOK {
		t.Fatalf("second %d %s", rec.Code, rec.Body.String())
	}
	if got := gate.sent.Load(); got != 2 {
		t.Fatalf("sent %d, want 2", got)
	}
	var secondInApp int
	queryRowArgs(t, `SELECT count(*) FROM notifications WHERE announcement_id=$1`, []any{id}, &secondInApp)
	if secondInApp != firstInApp {
		t.Fatalf("bells %d -> %d", firstInApp, secondInApp)
	}
	queryRowArgs(t, `SELECT finished_at FROM announcements WHERE id=$1`, []any{id}, &finishedAt)
	if finishedAt == nil {
		t.Fatal("not finished after resume")
	}
	// 送り終えたお知らせの送り直しは拒否する。内容違いも拒否する。
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/announcements", admin, withID(body, id)); rec.Code != http.StatusConflict {
		t.Fatalf("resend finished %d %s", rec.Code, rec.Body.String())
	}
	other := map[string]any{"title": "別の内容です", "body": "本文です", "url": "/x", "confirm": "送信"}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/announcements", admin, withID(other, id)); rec.Code != http.StatusConflict {
		t.Fatalf("different content %d %s", rec.Code, rec.Body.String())
	}
}

// 許可が残ったまま購読だけ消えた端末は、ログイン時に自動で再購読してよいか確かめられる。
func TestPushSubscriptionStatusTracksEverSubscribed(t *testing.T) {
	h := setup(t)
	user := uuid.New()
	tok := token(t, user)
	var before struct {
		EverSubscribed bool `json:"ever_subscribed"`
	}
	rec := doJSON(t, h, http.MethodGet, "/v1/me/push-subscriptions/status", tok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &before)
	if before.EverSubscribed {
		t.Fatal("ever_subscribed before any subscription")
	}
	p256dh, auth := testPushKeys(t)
	if rec := doJSON(t, h, http.MethodPost, "/v1/me/push-subscriptions", tok, map[string]any{
		"endpoint": "https://web.push.apple.com/sub",
		"keys":     map[string]string{"p256dh": p256dh, "auth": auth},
	}); rec.Code != http.StatusNoContent {
		t.Fatalf("subscribe %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/me/push-subscriptions/status", tok, nil)
	var after struct {
		EverSubscribed bool `json:"ever_subscribed"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &after)
	if !after.EverSubscribed {
		t.Fatal("ever_subscribed not recorded")
	}
}

func withID(body map[string]any, id string) map[string]any {
	out := map[string]any{}
	for k, v := range body {
		out[k] = v
	}
	out["id"] = id
	return out
}

func postMonthlyReminder(t *testing.T, h http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/internal/monthly-reminder", nil)
	req.Header.Set("X-Reminder-Secret", "test-reminder-secret")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// ケガ経過のプッシュは、未読のベルがあっても新しい報告ごとに送る。ベルは重ねない。
func TestInjuryUpdatePushAfterRead(t *testing.T) {
	gate := newPushGate(t, 0)
	h := setupPushVAPID(t, gate, nil)
	author := tokenWithProfile(t, uuid.New(), "報告する人", true)
	follower := tokenWithProfile(t, uuid.New(), "見守る人", true)
	rec := postInjury(t, h, author, map[string]any{"kind": "injury", "title": "プッシュの確認", "body": "本文", "body_part": "achilles-tendinitis"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	joinGroup(t, h, follower, "achilles-tendinitis")
	var post struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &post)
	subscribeFakePush(t, h, follower, gate.server.URL+"/follower")
	if r := doJSON(t, h, http.MethodPost, "/v1/posts/"+post.ID+"/likes", follower, nil); r.Code >= 300 {
		t.Fatalf("like %d %s", r.Code, r.Body.String())
	}
	update := map[string]any{"stage": "rehab", "body": "歩けた"}
	if r := doJSON(t, h, http.MethodPost, "/v1/posts/"+post.ID+"/injury/updates", author, update); r.Code != http.StatusCreated {
		t.Fatalf("update1 %d %s", r.Code, r.Body.String())
	}
	deadline := time.Now().Add(10 * time.Second)
	for gate.sent.Load() < 1 && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if got := gate.sent.Load(); got != 1 {
		t.Fatalf("pushes after update1 = %d", got)
	}
	// 未読のベルのまま2回目の報告をすると、プッシュは届くがベルは増えない。
	if r := doJSON(t, h, http.MethodPost, "/v1/posts/"+post.ID+"/injury/updates", author, map[string]any{"stage": "jogging", "body": "走れた"}); r.Code != http.StatusCreated {
		t.Fatalf("update2 %d %s", r.Code, r.Body.String())
	}
	deadline = time.Now().Add(10 * time.Second)
	for gate.sent.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if got := gate.sent.Load(); got != 2 {
		t.Fatalf("pushes after update2 = %d, want 2", got)
	}
	var bells int
	queryRowArgs(t, `SELECT count(*) FROM notifications n JOIN profiles p ON p.id=n.recipient_id WHERE n.kind='injury_update' AND n.post_id=$1 AND p.nickname='見守る人'`, []any{post.ID}, &bells)
	if bells != 1 {
		t.Fatalf("bells %d, want 1", bells)
	}
}
