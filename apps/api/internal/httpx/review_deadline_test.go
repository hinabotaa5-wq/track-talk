package httpx

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/jst"
)

func jstAt(day, hour, minute int) time.Time {
	return time.Date(2026, 11, day, hour, minute, 0, 0, jst.Zone)
}

// 返却期限は申込みから24時間。夜間も止まらず、日付や時刻に関係なく同じ。
func TestReviewDueAtIs24Hours(t *testing.T) {
	for _, start := range []time.Time{jstAt(2, 2, 0), jstAt(2, 12, 0), jstAt(2, 22, 45), jstAt(2, 23, 30), jstAt(30, 23, 59)} {
		if got := reviewDueAt(start); !got.Equal(start.Add(24 * time.Hour)) {
			t.Fatalf("%v → %v", start, got)
		}
	}
}

// 引受期限は90分。日本時間の23時〜翌7時はカウントを止める。
func TestOfferExpiresAtPausesOvernight(t *testing.T) {
	cases := []struct {
		name  string
		start time.Time
		want  time.Time
	}{
		{"昼はそのまま90分", jstAt(2, 12, 0), jstAt(2, 13, 30)},
		{"02:00 は 08:30", jstAt(2, 2, 0), jstAt(2, 8, 30)},
		{"22:45 は23時までに15分、残り75分を7時から", jstAt(2, 22, 45), jstAt(3, 8, 15)},
		{"23:30 は翌朝 08:30", jstAt(2, 23, 30), jstAt(3, 8, 30)},
		{"23:00 ちょうどは止まっている", jstAt(2, 23, 0), jstAt(3, 8, 30)},
		{"22:59 は23時までに1分", jstAt(2, 22, 59), jstAt(3, 8, 29)},
		{"22:30 は23時までに30分、残り60分を7時から", jstAt(2, 22, 30), jstAt(3, 8, 0)},
		{"21:30 は23:00 ちょうどで期限", jstAt(2, 21, 30), jstAt(2, 23, 0)},
		{"21:31 は残り1分を7時から", jstAt(2, 21, 31), jstAt(3, 7, 1)},
		{"07:00 ちょうどから数える", jstAt(2, 7, 0), jstAt(2, 8, 30)},
		{"06:59 はまだ止まっている", jstAt(2, 6, 59), jstAt(2, 8, 30)},
		{"00:00 は 08:30", jstAt(3, 0, 0), jstAt(3, 8, 30)},
		{"月末の夜は翌月の7時から", jstAt(30, 23, 10), time.Date(2026, 12, 1, 8, 30, 0, 0, jst.Zone)},
	}
	for _, c := range cases {
		if got := offerExpiresAt(c.start, offerWindow); !got.Equal(c.want) {
			t.Errorf("%s: %v → %v, want %v", c.name, c.start, got.In(jst.Zone), c.want)
		}
	}
}

// 夜間の判定は日本時間で行い、渡された時刻やサーバーのタイムゾーンに左右されない。
func TestOfferExpiresAtUsesJSTWallClock(t *testing.T) {
	// 日本時間の 02:00 を UTC とニューヨーク時間で渡しても 08:30（日本時間）になる。
	start := jstAt(2, 2, 0)
	ny := time.FixedZone("EST", -5*60*60)
	for _, in := range []time.Time{start.UTC(), start.In(ny)} {
		if got := offerExpiresAt(in, offerWindow); !got.Equal(jstAt(2, 8, 30)) {
			t.Fatalf("%v → %v", in, got)
		}
	}
	prev := time.Local
	time.Local = ny
	defer func() { time.Local = prev }()
	if got := offerExpiresAt(time.Date(2026, 11, 2, 8, 45, 0, 0, time.Local), offerWindow); !got.Equal(jstAt(3, 8, 15)) {
		// ニューヨークの 08:45 は日本時間の 22:45。
		t.Fatalf("local tz: %v", got.In(jst.Zone))
	}
}

func TestOfferDeadlineText(t *testing.T) {
	if got := offerDeadlineText(jstAt(3, 7, 30).UTC()); got != "11月3日 7:30" {
		t.Fatal(got)
	}
}

func TestOfferReminderWindowPausesOvernight(t *testing.T) {
	// 22:10 の打診は23時まで50分を数え、翌朝の7:40に期限を迎える。
	expires := offerExpiresAt(jstAt(2, 22, 10), offerWindow)
	before := offerExpiresAt(jstAt(3, 7, 9), reviewOfferReminderLead)
	due := offerExpiresAt(jstAt(3, 7, 10), reviewOfferReminderLead)
	if !before.Before(expires) || !due.Equal(expires) {
		t.Fatalf("before=%v, due=%v, expires=%v", before, due, expires)
	}
	// 翌朝7:20の期限には、前夜の22:50で残り30分になる。
	expires = offerExpiresAt(jstAt(2, 21, 50), offerWindow)
	if got := offerExpiresAt(jstAt(2, 22, 50), reviewOfferReminderLead); !got.Equal(expires) {
		t.Fatalf("overnight threshold: %v, want %v", got, expires)
	}
}

func TestReviewOfferTagsArePerOffer(t *testing.T) {
	first, second := uuid.New(), uuid.New()
	if reviewOfferTag(first) == reviewOfferTag(second) {
		t.Fatal("different offers shared a notification tag")
	}
}
