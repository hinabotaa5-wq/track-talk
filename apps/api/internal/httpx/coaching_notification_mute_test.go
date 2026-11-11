package httpx_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

// 一時ミュート中は、新規コーチ通知の記録を残さず、許可リスト内だけにベルとプッシュを送る。
func TestCoachingMuteNewCoachAnnouncementAllowlist(t *testing.T) {
	fakeCoachingCloudinary(t)
	a := uuid.New()
	mutedSetup := func(cfg *config.Config) {
		cfg.CoachingNewAcceptance = true
		cfg.CoachingCoachList = true
		cfg.CoachingNotificationsMuted = true
		cfg.CoachingNotificationAllowlist = map[string]struct{}{a.String(): {}}
	}
	gate := newPushGate(t, 0)
	h := setupPushVAPID(t, gate, mutedSetup)
	tadmin := adminToken(t, h)

	ta := token(t, a)
	subscribeFakePush(t, h, ta, gate.server.URL+"/a")
	b := uuid.New()
	tb := token(t, b)
	subscribeFakePush(t, h, tb, gate.server.URL+"/b")

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
	var n int
	queryRowArgs(t, `SELECT count(*) FROM announcements WHERE id=$1`, []any{annID}, &n)
	if n != 0 {
		t.Fatalf("muted announcement row created n=%d", n)
	}
	queryRowArgs(t, `SELECT count(*) FROM new_coach_announcements WHERE coach_id=$1`, []any{coach}, &n)
	if n != 0 {
		t.Fatalf("muted new_coach_announcements record created n=%d", n)
	}
	countBell := func(id uuid.UUID) int {
		var c int
		queryRowArgs(t, `SELECT count(*) FROM notifications WHERE recipient_id=$1 AND kind='message' AND title='新しいコーチが加わりました'`, []any{id}, &c)
		return c
	}
	if got := countBell(a); got != 1 {
		t.Fatalf("allowlisted bell a=%d", got)
	}
	if got := countBell(b); got != 0 {
		t.Fatalf("outside bell b=%d", got)
	}
	if got := countBell(coach); got != 0 {
		t.Fatalf("coach bell=%d", got)
	}
	var outbox int
	queryRowArgs(t, `SELECT count(*) FROM push_outbox WHERE dedup_key LIKE 'announcement:%'`, nil, &outbox)
	if outbox != 0 {
		t.Fatalf("announcement outbox=%d", outbox)
	}
	// 受付開始の疎通テスト（コーチ本人の操作への直接応答、抑止対象外）と許可リスト内の a への1通。
	if got := gate.sent.Load() - base; got != 2 {
		t.Fatalf("sent=%d", got)
	}
}

// 一時ミュート中に受付が閉じていれば、許可リスト内にも何も送らない（今夜の順序どおり）。
func TestCoachingMuteNewCoachAnnouncementClosedAcceptance(t *testing.T) {
	fakeCoachingCloudinary(t)
	a := uuid.New()
	gate := newPushGate(t, 0)
	h := setupPushVAPID(t, gate, func(cfg *config.Config) {
		cfg.CoachingNewAcceptance = false
		cfg.CoachingCoachList = true
		cfg.CoachingNotificationsMuted = true
		cfg.CoachingNotificationAllowlist = map[string]struct{}{a.String(): {}}
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
	execSQLArgs(t, `UPDATE coach_profiles SET stripe_account_id = $2, payouts_enabled = true WHERE profile_id = $1`, coach, "acct-"+coach.String())
	subscribeFakePush(t, h, tcoach, gate.server.URL+"/coach")

	base := gate.sent.Load()
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me", tcoach, map[string]any{"on_duty": true}); rec.Code != http.StatusOK {
		t.Fatalf("on duty %d %s", rec.Code, rec.Body.String())
	}
	var n int
	queryRowArgs(t, `SELECT count(*) FROM new_coach_announcements WHERE coach_id=$1`, []any{coach}, &n)
	if n != 0 {
		t.Fatalf("record created n=%d", n)
	}
	queryRowArgs(t, `SELECT count(*) FROM notifications WHERE recipient_id=$1 AND kind='message' AND title='新しいコーチが加わりました'`, []any{a}, &n)
	if n != 0 {
		t.Fatalf("allowlisted bell sent while closed n=%d", n)
	}
	// 受付開始の疎通テストの1通だけ（コーチ本人の操作への直接応答）。
	if got := gate.sent.Load() - base; got != 1 {
		t.Fatalf("sent=%d", got)
	}
}

// 許可リストが空のままミュートしたら全抑止に倒れる（フェイルセーフ）。
func TestCoachingMuteEmptyAllowlistSuppressesAll(t *testing.T) {
	fakeCoachingCloudinary(t)
	gate := newPushGate(t, 0)
	h := setupPushVAPID(t, gate, func(cfg *config.Config) {
		cfg.CoachingNewAcceptance = true
		cfg.CoachingCoachList = true
		cfg.CoachingNotificationsMuted = true
	})
	tadmin := adminToken(t, h)

	a := uuid.New()
	subscribeFakePush(t, h, token(t, a), gate.server.URL+"/a")

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
	var n int
	queryRowArgs(t, `SELECT count(*) FROM new_coach_announcements WHERE coach_id=$1`, []any{coach}, &n)
	if n != 0 {
		t.Fatalf("record created n=%d", n)
	}
	queryRowArgs(t, `SELECT count(*) FROM notifications WHERE kind='message' AND title='新しいコーチが加わりました'`, nil, &n)
	if n != 0 {
		t.Fatalf("bell created n=%d", n)
	}
	// 受付開始の疎通テストの1通だけ（コーチ本人の操作への直接応答）。告知の送信はゼロ。
	if got := gate.sent.Load() - base; got != 1 {
		t.Fatalf("sent=%d", got)
	}
}

// 抑止の判定そのもの：OFF は常に通す、ON＋空は全抑止、ON＋リストは内外で分ける。
func TestCoachingNotificationSuppressedPredicate(t *testing.T) {
	off := config.Config{}
	if off.CoachingNotificationSuppressed("11111111-1111-1111-1111-111111111111") {
		t.Fatalf("mute OFF should not suppress")
	}
	empty := config.Config{CoachingNotificationsMuted: true}
	if !empty.CoachingNotificationSuppressed("11111111-1111-1111-1111-111111111111") {
		t.Fatalf("mute ON with empty allowlist should suppress all")
	}
	a := "22222222-2222-2222-2222-222222222222"
	listed := config.Config{
		CoachingNotificationsMuted:    true,
		CoachingNotificationAllowlist: map[string]struct{}{a: {}},
	}
	if listed.CoachingNotificationSuppressed(a) {
		t.Fatalf("allowlisted should not be suppressed")
	}
	if !listed.CoachingNotificationSuppressed("33333333-3333-3333-3333-333333333333") {
		t.Fatalf("outside should be suppressed")
	}
	if listed.CoachingNotificationSuppressed("22222222-2222-2222-2222-222222222222") {
		t.Fatalf("uppercase input should match case-insensitively")
	}
}
