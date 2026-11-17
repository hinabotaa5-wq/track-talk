package httpx

import (
	"time"

	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/jst"
)

// 継続プランのコーチ不在日の合意延長（#898）の、日数の数え方。
// 不在日は、不在開始日以上・復帰日未満の各暦日（復帰日は不在日に含めない）。日本時間の日付で数える。

// absentDaysThrough は、合意した不在日のうち、asOf の日（日本時間の日付を UTC の 0 時で持つ）までの日数。asOf の日も数える。
func absentDaysThrough(ct db.CoachingContract, asOf time.Time) int64 {
	if ct.AbsenceFrom == nil || ct.AbsenceReturnOn == nil {
		return 0
	}
	from, ret := calendarDate(*ct.AbsenceFrom), calendarDate(*ct.AbsenceReturnOn)
	last := calendarDate(asOf).AddDate(0, 0, 1)
	if last.After(ret) {
		last = ret
	}
	n := int64(last.Sub(from).Hours() / 24)
	return min(max(n, 0), int64(ret.Sub(from).Hours()/24))
}

// absentElapsed は、時刻 at までに過ぎた合意不在日の長さ（不在期間は、開始日の 0 時〜復帰日の 0 時、日本時間）。
func absentElapsed(ct db.CoachingContract, at time.Time) time.Duration {
	if ct.AbsenceFrom == nil || ct.AbsenceReturnOn == nil {
		return 0
	}
	from := jstMidnight(*ct.AbsenceFrom)
	ret := jstMidnight(*ct.AbsenceReturnOn)
	end := at
	if end.After(ret) {
		end = ret
	}
	if !end.After(from) {
		return 0
	}
	return end.Sub(from)
}

// jstMidnight は、DB の date 列の日付の、日本時間の 0 時。
func jstMidnight(d time.Time) time.Time {
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, jst.Zone)
}
