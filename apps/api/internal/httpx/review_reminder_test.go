package httpx

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/jst"
)

// countableUntil は offerExpiresAt の逆演算。打診から期限までのカウント可能な時間と一致する。
func TestCountableUntilMatchesOfferWindow(t *testing.T) {
	for _, start := range []time.Time{jstAt(2, 2, 0), jstAt(2, 12, 0), jstAt(2, 22, 45), jstAt(2, 23, 30), jstAt(2, 6, 59), jstAt(30, 23, 10)} {
		if got := countableUntil(start, offerExpiresAt(start, offerWindow)); got != offerWindow {
			t.Errorf("%v: %v, want %v", start, got, offerWindow)
		}
	}
}

func TestCountableUntil(t *testing.T) {
	cases := []struct {
		name string
		from time.Time
		to   time.Time
		want time.Duration
	}{
		{"昼はそのまま", jstAt(2, 12, 0), jstAt(2, 13, 30), 90 * time.Minute},
		{"23時まで15分", jstAt(2, 22, 45), jstAt(3, 8, 15), 90 * time.Minute},
		{"夜間は数えない", jstAt(2, 22, 45), jstAt(3, 0, 0), 15 * time.Minute},
		{"夜間の終わりは0", jstAt(2, 1, 0), jstAt(2, 6, 0), 0},
		{"0時から7時は0", jstAt(3, 0, 0), jstAt(3, 7, 0), 0},
		{"7時ちょうどから数える", jstAt(2, 7, 0), jstAt(2, 7, 20), 20 * time.Minute},
		{"期限が前なら数えない", jstAt(2, 12, 0), jstAt(2, 11, 0), 0},
	}
	for _, c := range cases {
		if got := countableUntil(c.from, c.to); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

// reminderDue は、カウント可能な残りが30分以下で期限前だけ真。夜間停止を跨いでも残り時間で判定する。
func TestReminderDue(t *testing.T) {
	at := func(day, hour, minute int) time.Time {
		return time.Date(2026, 11, day, hour, minute, 0, 0, jst.Zone)
	}
	expires := offerExpiresAt(at(2, 22, 45), offerWindow) // 11月3日 8:15
	cases := []struct {
		name string
		now  time.Time
		want bool
	}{
		{"残り90分は送らない", at(2, 22, 45), false},
		{"残り31分は送らない", at(3, 7, 44), false},
		{"残り30分で送る", at(3, 7, 45), true},
		{"残り1分で送る", at(3, 8, 14), true},
		{"期限切れは送らない", at(3, 8, 15), false},
		{"期限後は送らない", at(3, 9, 0), false},
		{"打診前は送らない", at(2, 22, 44), false},
		{"夜間でも残りで判定", at(2, 23, 30), false}, // 残り75分
	}
	for _, c := range cases {
		if got := reminderDue(c.now, expires); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

// offerPushTag は打診（依頼）ごとに一意。同じ打診は同じ tag。
func TestOfferPushTagUniquePerRequest(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	if offerPushTag(a) == offerPushTag(b) {
		t.Fatal("different requests share a tag")
	}
	if offerPushTag(a) != offerPushTag(a) {
		t.Fatal("same request has different tags")
	}
	if offerPushTag(a) == "review-offer" {
		t.Fatal("tag is still the shared literal")
	}
}
