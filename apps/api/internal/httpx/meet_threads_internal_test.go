package httpx

import (
	"strings"
	"testing"
	"time"
)

func TestIsFeaturedInternational(t *testing.T) {
	cases := []struct {
		name     string
		organizer string
		source   string
		want     bool
	}{
		{name: "オリンピック・ロサンゼルス", want: true},
		{name: "世界陸上・北京", want: true},
		{name: "U20世界陸上・ユージーン", want: true},
		{name: "ダイヤモンドリーグ・ファイナル", want: true},
		{name: "第21回アジア競技大会", want: true},
		{name: "Asian Games", want: true},
		{name: "ゴールデンガラ", organizer: "http://rome.diamondleague.com/", want: true},
		{name: "", want: false},
		{name: "コンチネンタルツアー・ゴールド", want: false},
		{name: "世界室内陸上", want: false},
		{name: "U18世界陸上", want: false},
		{name: "冬季オリンピック", want: false},
	}
	for _, c := range cases {
		if got := isFeaturedInternational(c.name, c.organizer, c.source); got != c.want {
			t.Errorf("isFeaturedInternational(%q, %q, %q) = %v, want %v", c.name, c.organizer, c.source, got, c.want)
		}
	}
}

func TestIsDiamondLeagueSite(t *testing.T) {
	if !isDiamondLeagueSite("https://diamondleague.com/rome") {
		t.Error("apex host should match")
	}
	if !isDiamondLeagueSite("http://rome.diamondleague.com/") {
		t.Error("subdomain should match")
	}
	if isDiamondLeagueSite("https://example.com/diamondleague.com") {
		t.Error("path should not match")
	}
	if isDiamondLeagueSite("") {
		t.Error("empty should not match")
	}
}

func TestMeetIsOngoing(t *testing.T) {
	single := time.Date(2026, time.October, 12, 0, 0, 0, 0, time.UTC)
	multiEnd := time.Date(2026, time.October, 13, 0, 0, 0, 0, time.UTC)

	if !meetIsOngoing(single, single, "2026-10-12") {
		t.Error("single-day meet should be ongoing on its day")
	}
	if meetIsOngoing(single, single, "2026-10-13") {
		t.Error("single-day meet should end the next day")
	}
	if !meetIsOngoing(single, multiEnd, "2026-10-13") {
		t.Error("multi-day meet should be ongoing on its last day")
	}
	if meetIsOngoing(single, multiEnd, "2026-10-11") {
		t.Error("meet should not be ongoing before it starts")
	}
}

func TestMeetTitle(t *testing.T) {
	if got, want := meetTitle("出雲駅伝", "2026年10月12日"), "出雲駅伝（2026年10月12日）実況"; got != want {
		t.Errorf("title = %q, want %q", got, want)
	}
	// posts.title は120文字まで。長い大会名は「…」で詰めて超えないようにする。
	long := meetTitle(strings.Repeat("あ", 200), "2026年10月12日")
	if n := len([]rune(long)); n != 120 {
		t.Errorf("long title rune count = %d, want 120", n)
	}
	if !strings.HasSuffix(long, "…（2026年10月12日）実況") {
		t.Errorf("long title suffix = %q", long)
	}
}

func TestMeetBody(t *testing.T) {
	if got, want := meetBody("島根・出雲", "https://izumo.example/"), "会場: 島根・出雲\n公式サイト: https://izumo.example/"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
	if got, want := meetBody("", ""), "大会当日の実況スレッドです。"; got != want {
		t.Errorf("empty body = %q, want %q", got, want)
	}
	if got, want := meetBody("  ", ""), "大会当日の実況スレッドです。"; got != want {
		t.Errorf("blank venue body = %q, want %q", got, want)
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "  ", "a", "b"); got != "a" {
		t.Errorf("firstNonEmpty = %q, want a", got)
	}
	if got := firstNonEmpty("", " "); got != "" {
		t.Errorf("firstNonEmpty all blank = %q, want empty", got)
	}
}
