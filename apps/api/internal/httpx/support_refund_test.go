package httpx

import (
	"testing"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/jst"
)

func jstDay(y int, m time.Month, d, h int) time.Time { return time.Date(y, m, d, h, 0, 0, 0, jst.Zone) }

// 動画が無制限の大会までプランの未提供分は、期間の日割り（契約価格 ÷ 期間日数 × 未経過日数・円未満切り捨て）。
func TestSupportRemainingByDays(t *testing.T) {
	zero := int16(0)
	start, comp := jstDay(2026, 11, 10, 9), jstDay(2026, 11, 19, 0) // 10日間（両端を含む）
	ct := db.CoachingContract{PriceYen: 9000, VideoReviews: &zero, StartsAt: &start, CompetitionDate: &comp, CallCount: 0}
	for _, c := range []struct {
		name string
		now  time.Time
		want int32
	}{
		{"開始日当日は1日分を使ったことにする", jstDay(2026, 11, 10, 23), 8100},
		{"3日目", jstDay(2026, 11, 12, 10), 6300},
		{"目標大会の日は全日使った", jstDay(2026, 11, 19, 12), 0},
		{"期間が終わったあとは0円", jstDay(2026, 12, 1, 12), 0},
		{"開始前は全額", jstDay(2026, 11, 9, 12), 9000},
	} {
		if got := supportRemainingByDays(ct, c.now); got != c.want {
			t.Errorf("%s: %d, want %d", c.name, got, c.want)
		}
	}
	// 円未満は切り捨て: 10,000円 ÷ 3日 × 2日 = 6,666円
	comp3 := jstDay(2026, 11, 12, 0)
	ct3 := db.CoachingContract{PriceYen: 10000, VideoReviews: &zero, StartsAt: &start, CompetitionDate: &comp3}
	if got := supportRemainingByDays(ct3, jstDay(2026, 11, 10, 12)); got != 6666 {
		t.Errorf("floor: %d", got)
	}
	// 通話0回でも日割りで返る（回数の按分は使わない）。通話があっても同じ。
	ct.CallCount = 2
	if got := supportRemaining(ct, db.SupportUsageRow{CallsCompleted: 2}); got < 0 {
		t.Errorf("remaining %d", got)
	}
	// 始まっていない契約は全額。
	ct.StartsAt = nil
	if got := supportRemainingByDays(ct, jstDay(2026, 11, 12, 10)); got != 9000 {
		t.Errorf("not started: %d", got)
	}
	// 動画の回数がある従来の契約は、これまでどおり回数で按分する。
	one := int16(1)
	old := db.CoachingContract{PriceYen: 9000, VideoReviews: &one, CallCount: 1}
	if got := supportRemaining(old, db.SupportUsageRow{VideoAnswered: 1}); got != 4500 {
		t.Errorf("old: %d", got)
	}
}
